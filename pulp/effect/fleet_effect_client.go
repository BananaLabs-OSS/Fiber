package effect

import (
	"bytes"
	"fmt"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/BananaLabs-OSS/Fiber/pulp"
	"github.com/vmihailenco/msgpack/v5"
)

const (
	maxFleetAnnouncementCharacters = 500
	instantExtensionAnnouncement   = "Server extended! You have more time. Enjoy!"
)

type fleetAnnouncementPayload struct {
	Extension   string `msgpack:"extension"`
	RCONAction  string `msgpack:"rcon_action"`
	ServerID    string `msgpack:"server_id"`
	NodeID      string `msgpack:"node_id"`
	ContainerID string `msgpack:"container_id"`
	Message     string `msgpack:"message"`
	Reason      string `msgpack:"reason"`
}

type fleetAnnouncementResult struct {
	ServerID    string `msgpack:"server_id"`
	NodeID      string `msgpack:"node_id"`
	ContainerID string `msgpack:"container_id"`
	Status      string `msgpack:"status"`
}

type fleetLifecyclePayload struct {
	Extension   string `msgpack:"extension"`
	ServerID    string `msgpack:"server_id"`
	NodeID      string `msgpack:"node_id"`
	ContainerID string `msgpack:"container_id"`
	Reason      string `msgpack:"reason,omitempty"`
}

type fleetLifecycleResult struct {
	ServerID    string `msgpack:"server_id"`
	NodeID      string `msgpack:"node_id"`
	ContainerID string `msgpack:"container_id"`
	Operation   string `msgpack:"operation"`
	Status      string `msgpack:"status"`
	CompletedAt string `msgpack:"completed_at"`
}

// ExecuteFleetEffect sends one canonical Fleet effect intent to the narrowly
// scoped effect.fleet.runtime host import. The host owns the privileged Fleet
// operation only; the calling state owner remains responsible for persisting
// the intent and receipt before it advances its workflow.
//
// A cell using this wrapper must declare the effect.fleet.runtime capability.
// Only the canonical Fleet kinds are admitted, so this import cannot become a
// general-purpose privileged-effect escape hatch.
func ExecuteFleetEffect(intent Intent) (Receipt, error) {
	if err := intent.Validate(); err != nil {
		return Receipt{}, fmt.Errorf("effect.fleet.runtime: invalid intent: %w", err)
	}
	if !isFleetEffectKind(intent) {
		return Receipt{}, fmt.Errorf("effect.fleet.runtime: unsupported intent kind %q", intent.Kind)
	}

	request, err := MarshalIntent(intent)
	if err != nil {
		return Receipt{}, fmt.Errorf("effect.fleet.runtime: marshal intent: %w", err)
	}

	wire, code := fleetEffectExecuteWire(request)
	if err := fleetEffectCodeError(code); err != nil {
		return Receipt{}, err
	}
	if len(wire) == 0 {
		return Receipt{}, fmt.Errorf("effect.fleet.runtime: empty receipt response")
	}

	// Decode directly rather than through UnmarshalReceipt: a host response is
	// required to be canonical and must not silently accept a legacy alias.
	var receipt Receipt
	if err := msgpack.Unmarshal(wire, &receipt); err != nil {
		return Receipt{}, fmt.Errorf("effect.fleet.runtime: decode receipt: %w", err)
	}
	if err := receipt.ValidateFor(intent); err != nil {
		return Receipt{}, fmt.Errorf("effect.fleet.runtime: invalid receipt: %w", err)
	}
	if err := validateFleetAnnouncementReceiptForIntent(intent, receipt); err != nil {
		return Receipt{}, fmt.Errorf("effect.fleet.runtime: invalid receipt: %w", err)
	}
	if err := validateFleetLifecycleReceiptForIntent(intent, receipt); err != nil {
		return Receipt{}, fmt.Errorf("effect.fleet.runtime: invalid receipt: %w", err)
	}
	return receipt, nil
}

func isFleetEffectKind(intent Intent) bool {
	switch intent.Kind {
	case KindFleetServerDeprovision:
		return true
	case KindFleetExtensionApply:
		return isAdmittedFleetExtensionPayload(intent)
	default:
		return false
	}
}

func isAdmittedFleetExtensionPayload(intent Intent) bool {
	var payload struct {
		Extension  string `msgpack:"extension"`
		RCONAction string `msgpack:"rcon_action"`
	}
	if err := msgpack.Unmarshal(intent.Payload, &payload); err != nil {
		return false
	}
	switch payload.Extension {
	case "restart", "regenerate":
		_, err := decodeFleetLifecyclePayload(intent.Payload)
		return err == nil
	}
	switch payload.RCONAction {
	case "save_flush":
		// Preserve the existing save-flush admission contract. The privileged
		// host applies its own exact payload validation before execution.
		return true
	case "announce":
		_, err := decodeFleetAnnouncementPayload(intent.Payload)
		return err == nil
	default:
		return false
	}
}

func decodeFleetLifecyclePayload(raw msgpack.RawMessage) (fleetLifecyclePayload, error) {
	var payload fleetLifecyclePayload
	decoder := msgpack.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields(true)
	if err := decoder.Decode(&payload); err != nil {
		return payload, fmt.Errorf("decode Fleet lifecycle payload: %w", err)
	}
	switch payload.Extension {
	case "restart", "regenerate":
	default:
		return payload, fmt.Errorf("Fleet lifecycle extension %q is not allowed", payload.Extension)
	}
	if err := validateField("Fleet lifecycle server_id", payload.ServerID); err != nil {
		return payload, err
	}
	if err := validateField("Fleet lifecycle node_id", payload.NodeID); err != nil {
		return payload, err
	}
	if err := validateField("Fleet lifecycle container_id", payload.ContainerID); err != nil {
		return payload, err
	}
	if payload.Reason != "" {
		if err := validateField("Fleet lifecycle reason", payload.Reason); err != nil {
			return payload, err
		}
	}
	return payload, nil
}

func decodeFleetAnnouncementPayload(raw msgpack.RawMessage) (fleetAnnouncementPayload, error) {
	var payload fleetAnnouncementPayload
	decoder := msgpack.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields(true)
	if err := decoder.Decode(&payload); err != nil {
		return payload, fmt.Errorf("decode Fleet announcement payload: %w", err)
	}
	if payload.Extension != "rcon" || payload.RCONAction != "announce" {
		return payload, fmt.Errorf("Fleet announcement extension identity is invalid")
	}
	if err := validateField("Fleet announcement server_id", payload.ServerID); err != nil {
		return payload, err
	}
	if err := validateField("Fleet announcement node_id", payload.NodeID); err != nil {
		return payload, err
	}
	if err := validateField("Fleet announcement container_id", payload.ContainerID); err != nil {
		return payload, err
	}
	switch payload.Reason {
	case "expiry-warning", "scheduled-restart-warning":
	case "":
		if payload.Message != instantExtensionAnnouncement {
			return payload, fmt.Errorf("Fleet announcement reason is required")
		}
	default:
		return payload, fmt.Errorf("Fleet announcement reason %q is not allowed", payload.Reason)
	}
	if !utf8.ValidString(payload.Message) || payload.Message == "" || strings.TrimSpace(payload.Message) != payload.Message {
		return payload, fmt.Errorf("Fleet announcement message must contain 1-%d trimmed printable characters", maxFleetAnnouncementCharacters)
	}
	if count := utf8.RuneCountInString(payload.Message); count < 1 || count > maxFleetAnnouncementCharacters {
		return payload, fmt.Errorf("Fleet announcement message must contain 1-%d trimmed printable characters", maxFleetAnnouncementCharacters)
	}
	for _, r := range payload.Message {
		if !unicode.IsPrint(r) {
			return payload, fmt.Errorf("Fleet announcement message must contain only printable single-line characters")
		}
	}
	return payload, nil
}

func validateFleetAnnouncementReceiptForIntent(intent Intent, receipt Receipt) error {
	if intent.Kind != KindFleetExtensionApply || receipt.Status != Completed {
		return nil
	}
	payload, err := decodeFleetAnnouncementPayload(intent.Payload)
	if err != nil {
		// Other admitted Fleet extension contracts retain their existing
		// receipt behavior.
		return nil
	}
	var result fleetAnnouncementResult
	decoder := msgpack.NewDecoder(bytes.NewReader(receipt.Result))
	decoder.DisallowUnknownFields(true)
	if err := decoder.Decode(&result); err != nil {
		return fmt.Errorf("decode Fleet announcement result: %w", err)
	}
	if result.ServerID != payload.ServerID ||
		result.NodeID != payload.NodeID ||
		result.ContainerID != payload.ContainerID ||
		result.Status != "rcon_announce" {
		return fmt.Errorf("Fleet announcement result does not match intent")
	}
	return nil
}

func validateFleetLifecycleReceiptForIntent(intent Intent, receipt Receipt) error {
	if intent.Kind != KindFleetExtensionApply || receipt.Status != Completed {
		return nil
	}
	payload, err := decodeFleetLifecyclePayload(intent.Payload)
	if err != nil {
		// Other admitted Fleet extension contracts retain their established
		// receipt validation.
		return nil
	}
	var result fleetLifecycleResult
	decoder := msgpack.NewDecoder(bytes.NewReader(receipt.Result))
	decoder.DisallowUnknownFields(true)
	if err := decoder.Decode(&result); err != nil {
		return fmt.Errorf("decode Fleet lifecycle result: %w", err)
	}
	expectedStatus := map[string]string{
		"restart": "restarted", "regenerate": "regenerated",
	}[payload.Extension]
	if result.ServerID != payload.ServerID ||
		result.NodeID != payload.NodeID ||
		result.ContainerID != payload.ContainerID ||
		result.Operation != payload.Extension ||
		result.Status != expectedStatus {
		return fmt.Errorf("Fleet lifecycle result does not match intent")
	}
	if _, err := time.Parse(time.RFC3339Nano, result.CompletedAt); err != nil {
		return fmt.Errorf("Fleet lifecycle result completed_at must be RFC3339Nano")
	}
	return nil
}

func fleetEffectCodeError(code uint32) error {
	switch code {
	case 0:
		return nil
	case 1:
		return fmt.Errorf("effect.fleet.runtime: empty request")
	case 2:
		return fmt.Errorf("effect.fleet.runtime: request memory read failed")
	case 3:
		return fmt.Errorf("effect.fleet.runtime: request decode failed")
	case 4:
		return fmt.Errorf("effect.fleet.runtime: invalid effect intent")
	case 5:
		return fmt.Errorf("effect.fleet.runtime: Fleet effect execution failed")
	case 6:
		return fmt.Errorf("effect.fleet.runtime: response allocation or write failed")
	case 99:
		return fmt.Errorf("effect.fleet.runtime: %w", pulp.ErrCapabilityUnavailable)
	default:
		return fmt.Errorf("effect.fleet.runtime: host code %d", code)
	}
}
