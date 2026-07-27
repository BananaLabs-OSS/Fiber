package effect

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/vmihailenco/msgpack/v5"
)

// HTTPProbeResultContractV1 identifies the typed result stored in a completed
// host.http.probe.v1 effect receipt.
const HTTPProbeResultContractV1 = "http-probe-result.v1"

// HTTPProbeCommandV1 preserves the legacy Control-owned unauthenticated probe
// shape. URL is an allowlist selector only: the guest adapter derives an
// opaque destination name from ComponentSlug and URL, while the scoped host
// capability owns the actual URL, method, timeout, redirects, and body limit.
// Authenticated observations use ServiceObservationCommandV1 instead.
type HTTPProbeCommandV1 struct {
	Version        string `msgpack:"version"`
	EffectID       string `msgpack:"effect_id"`
	IdempotencyKey string `msgpack:"idempotency_key"`
	Kind           string `msgpack:"kind"`
	RequestID      string `msgpack:"request_id"`
	ComponentSlug  string `msgpack:"component_slug"`
	URL            string `msgpack:"url"`
	ObservedAt     string `msgpack:"observed_at"`
}

// HTTPProbeResultV1 binds the normalized observation to both the durable
// owner command and the exact host destination/fence derived from it.
type HTTPProbeResultV1 struct {
	Contract       string `msgpack:"contract"`
	EffectID       string `msgpack:"effect_id"`
	IdempotencyKey string `msgpack:"idempotency_key"`
	RequestID      string `msgpack:"request_id"`
	ComponentSlug  string `msgpack:"component_slug"`
	Destination    string `msgpack:"destination"`
	Fence          string `msgpack:"fence"`
	Status         string `msgpack:"status"`
	Message        string `msgpack:"message,omitempty"`
	Transport      string `msgpack:"transport"`
	HTTPStatus     uint16 `msgpack:"http_status,omitempty"`
	BodyBytes      uint32 `msgpack:"body_bytes,omitempty"`
	BodySHA256     string `msgpack:"body_sha256,omitempty"`
	ObservedAt     string `msgpack:"observed_at"`
}

func (command HTTPProbeCommandV1) Validate() error {
	if err := validateHTTPProbeTokenV1("HTTP probe owner version", command.Version, 128); err != nil {
		return err
	}
	if err := validateField("HTTP probe effect_id", command.EffectID); err != nil {
		return err
	}
	if err := validateField("HTTP probe idempotency_key", command.IdempotencyKey); err != nil {
		return err
	}
	if command.Kind != KindHTTPProbeExecute {
		return fmt.Errorf("HTTP probe kind must be %q", KindHTTPProbeExecute)
	}
	if err := validateField("HTTP probe request_id", command.RequestID); err != nil {
		return err
	}
	if err := validateHTTPProbeSlugV1(command.ComponentSlug); err != nil {
		return err
	}
	if err := validateHTTPProbeURLV1(command.URL); err != nil {
		return err
	}
	if _, err := time.Parse(time.RFC3339Nano, command.ObservedAt); err != nil {
		return errors.New("HTTP probe observed_at must be RFC3339")
	}
	return nil
}

func (result HTTPProbeResultV1) Validate() error {
	if result.Contract != HTTPProbeResultContractV1 {
		return errors.New("HTTP probe result contract is invalid")
	}
	if err := validateField("HTTP probe result effect_id", result.EffectID); err != nil {
		return err
	}
	if err := validateField("HTTP probe result idempotency_key", result.IdempotencyKey); err != nil {
		return err
	}
	if err := validateField("HTTP probe result request_id", result.RequestID); err != nil {
		return err
	}
	if err := validateHTTPProbeSlugV1(result.ComponentSlug); err != nil {
		return err
	}
	if err := validateHTTPProbeTokenV1("HTTP probe result destination", result.Destination, 256); err != nil {
		return err
	}
	if err := validateHTTPProbeTokenV1("HTTP probe result fence", result.Fence, 256); err != nil {
		return err
	}
	switch result.Status {
	case "operational", "degraded", "major", "maintenance":
	default:
		return errors.New("HTTP probe result status is not allowlisted")
	}
	if result.Message != "" {
		if err := validateHTTPProbeTokenV1("HTTP probe result message", result.Message, 512); err != nil {
			return err
		}
	}
	switch result.Transport {
	case "observed":
		if result.HTTPStatus < 100 || result.HTTPStatus > 599 ||
			result.BodySHA256 == "" || !isHTTPProbeSHA256V1(result.BodySHA256) {
			return errors.New("observed HTTP probe result requires status and body digest")
		}
	case "timeout", "network", "redirect", "body_too_large", "invalid_response":
		if result.HTTPStatus != 0 || result.BodyBytes != 0 || result.BodySHA256 != "" {
			return errors.New("failed HTTP probe transport cannot carry HTTP response evidence")
		}
	default:
		return errors.New("HTTP probe result transport is not allowlisted")
	}
	if _, err := time.Parse(time.RFC3339Nano, result.ObservedAt); err != nil {
		return errors.New("HTTP probe result observed_at must be RFC3339")
	}
	return nil
}

func (result HTTPProbeResultV1) ValidateFor(command HTTPProbeCommandV1) error {
	if err := command.Validate(); err != nil {
		return err
	}
	if err := result.Validate(); err != nil {
		return err
	}
	destination, err := HTTPProbeDestinationV1(command)
	if err != nil {
		return err
	}
	fence, err := HTTPProbeFenceV1(command)
	if err != nil {
		return err
	}
	if result.EffectID != command.EffectID ||
		result.IdempotencyKey != command.IdempotencyKey ||
		result.RequestID != command.RequestID ||
		result.ComponentSlug != command.ComponentSlug ||
		result.Destination != destination ||
		result.Fence != fence ||
		result.ObservedAt != command.ObservedAt {
		return errors.New("HTTP probe result does not match command")
	}
	return nil
}

// HTTPProbeDestinationV1 derives the only destination name a host may resolve
// for command. The URL itself never crosses the effect.http.probe.v1 import.
func HTTPProbeDestinationV1(command HTTPProbeCommandV1) (string, error) {
	if err := command.Validate(); err != nil {
		return "", err
	}
	digest := sha256.Sum256([]byte(command.URL))
	return "status." + command.ComponentSlug + "." + hex.EncodeToString(digest[:8]), nil
}

// HTTPProbeFenceV1 deterministically binds the host receipt to every durable
// owner field. Any retry of the exact command reuses it; any field tamper
// produces an idempotency/fence conflict at the scoped host capability.
func HTTPProbeFenceV1(command HTTPProbeCommandV1) (string, error) {
	if err := command.Validate(); err != nil {
		return "", err
	}
	hash := sha256.New()
	for _, value := range []string{
		command.Version,
		command.EffectID,
		command.IdempotencyKey,
		command.Kind,
		command.RequestID,
		command.ComponentSlug,
		command.URL,
		command.ObservedAt,
	} {
		_, _ = hash.Write([]byte(strconv.Itoa(len(value))))
		_, _ = hash.Write([]byte{':'})
		_, _ = hash.Write([]byte(value))
		_, _ = hash.Write([]byte{'\n'})
	}
	return "status-probe.v1." + hex.EncodeToString(hash.Sum(nil)), nil
}

func decodeHTTPProbeCommandV1(raw msgpack.RawMessage) (HTTPProbeCommandV1, error) {
	var command HTTPProbeCommandV1
	if err := decodeHTTPProbeStrictV1(raw, &command); err != nil {
		return command, fmt.Errorf("decode HTTP probe command: %w", err)
	}
	return command, nil
}

func decodeHTTPProbeResultV1(raw msgpack.RawMessage) (HTTPProbeResultV1, error) {
	var result HTTPProbeResultV1
	if err := decodeHTTPProbeStrictV1(raw, &result); err != nil {
		return result, fmt.Errorf("decode HTTP probe result: %w", err)
	}
	return result, nil
}

func decodeHTTPProbeStrictV1(raw []byte, target any) error {
	if len(raw) == 0 {
		return errors.New("empty MessagePack value")
	}
	reader := bytes.NewReader(raw)
	decoder := msgpack.NewDecoder(reader)
	decoder.DisallowUnknownFields(true)
	if err := decoder.Decode(target); err != nil {
		return err
	}
	if reader.Len() != 0 {
		return errors.New("trailing MessagePack data")
	}
	return nil
}

func validateHTTPProbeSlugV1(value string) error {
	if err := validateHTTPProbeTokenV1("HTTP probe component_slug", value, 128); err != nil {
		return err
	}
	for _, r := range value {
		if !(unicode.IsLetter(r) || unicode.IsDigit(r) || r == '.' || r == '_' || r == '-') {
			return errors.New("HTTP probe component_slug contains an unsupported character")
		}
	}
	return nil
}

func validateHTTPProbeURLV1(raw string) error {
	if raw == "" || len(raw) > 2048 || strings.TrimSpace(raw) != raw ||
		strings.IndexFunc(raw, unicode.IsControl) >= 0 {
		return errors.New("HTTP probe URL must be a bounded canonical HTTPS URL")
	}
	target, err := url.Parse(raw)
	if err != nil || target.Scheme != "https" || target.Host == "" ||
		target.User != nil || target.Fragment != "" || target.Opaque != "" ||
		target.String() != raw {
		return errors.New("HTTP probe URL must be a canonical absolute HTTPS URL without credentials or fragment")
	}
	return nil
}

func validateHTTPProbeTokenV1(label, value string, limit int) error {
	if value == "" || len(value) > limit || strings.TrimSpace(value) != value ||
		strings.IndexFunc(value, unicode.IsControl) >= 0 {
		return fmt.Errorf("%s must be a non-empty bounded value", label)
	}
	return nil
}

func isHTTPProbeSHA256V1(value string) bool {
	if len(value) != sha256.Size*2 {
		return false
	}
	for _, r := range value {
		if !(r >= '0' && r <= '9') && !(r >= 'a' && r <= 'f') {
			return false
		}
	}
	return true
}
