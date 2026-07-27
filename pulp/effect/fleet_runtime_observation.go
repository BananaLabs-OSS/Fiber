package effect

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"regexp"
	"strings"
	"time"
	"unicode"

	"github.com/vmihailenco/msgpack/v5"
)

const FleetRuntimeObservationContractV1 = "fleet.live-observation.v1"

type FleetRuntimeObservationFieldV1 string

const (
	FleetRuntimeObservationFieldSettingsV1       FleetRuntimeObservationFieldV1 = "settings"
	FleetRuntimeObservationFieldGameRulesV1      FleetRuntimeObservationFieldV1 = "gamerules"
	FleetRuntimeObservationFieldPlayersV1        FleetRuntimeObservationFieldV1 = "players"
	FleetRuntimeObservationFieldPlayerHistoryV1  FleetRuntimeObservationFieldV1 = "player_history"
	FleetRuntimeObservationFieldAccessV1         FleetRuntimeObservationFieldV1 = "access"
	FleetRuntimeObservationFieldAccessSnapshotV1 FleetRuntimeObservationFieldV1 = "access_snapshot"
	FleetRuntimeObservationFieldArtifactsV1      FleetRuntimeObservationFieldV1 = "artifacts"
	FleetRuntimeObservationFieldStatusV1         FleetRuntimeObservationFieldV1 = "status"
)

// FleetRuntimeObservationIntentV1 identifies one exact runtime generation and
// one bounded observation field. There is deliberately no endpoint, command,
// RCON text, file path, or arbitrary query field in this contract.
type FleetRuntimeObservationIntentV1 struct {
	Contract       string                         `msgpack:"contract"`
	ServerID       string                         `msgpack:"server_id"`
	NodeID         string                         `msgpack:"node_id"`
	ContainerID    string                         `msgpack:"container_id"`
	Field          FleetRuntimeObservationFieldV1 `msgpack:"field"`
	Generation     string                         `msgpack:"generation"`
	SourceRevision string                         `msgpack:"source_revision"`
}

type FleetRuntimeObservationAccessV1 struct {
	ServerID  string   `msgpack:"server_id"`
	Whitelist []string `msgpack:"whitelist,omitempty"`
	Operators []string `msgpack:"operators,omitempty"`
	Bans      []string `msgpack:"bans,omitempty"`
	UpdatedAt string   `msgpack:"updated_at"`
}

// FleetRuntimePlayerHistoryEntryV1 is one bounded usercache entry. It remains
// distinct from Players, which is the current online-player name list.
type FleetRuntimePlayerHistoryEntryV1 struct {
	UUID      string `msgpack:"uuid"`
	Name      string `msgpack:"name"`
	ExpiresOn string `msgpack:"expires_on"`
}

// FleetRuntimeAccessSnapshotIdentityV1 is the exact read-only identity shape
// shared by whitelist entries. It carries no access mutation authority.
type FleetRuntimeAccessSnapshotIdentityV1 struct {
	UUID string `msgpack:"uuid"`
	Name string `msgpack:"name"`
}

// FleetRuntimeAccessSnapshotOperatorV1 preserves the bounded operator metadata
// stored by the runtime without turning the observation into an op command.
type FleetRuntimeAccessSnapshotOperatorV1 struct {
	UUID                string `msgpack:"uuid"`
	Name                string `msgpack:"name"`
	Level               int    `msgpack:"level"`
	BypassesPlayerLimit bool   `msgpack:"bypasses_player_limit"`
}

// FleetRuntimeAccessSnapshotBanV1 preserves the bounded banned-player evidence
// stored by the runtime. Created and Expires intentionally remain strings:
// Minecraft persists its own timestamp representation and the "forever"
// sentinel, so the host must not reinterpret either value.
type FleetRuntimeAccessSnapshotBanV1 struct {
	UUID    string `msgpack:"uuid"`
	Name    string `msgpack:"name"`
	Created string `msgpack:"created"`
	Source  string `msgpack:"source"`
	Expires string `msgpack:"expires"`
	Reason  string `msgpack:"reason"`
}

// FleetRuntimeObservationAccessSnapshotV1 is a detailed, read-only runtime
// snapshot. It remains distinct from Access, whose name-only lists are the
// stable public projection.
type FleetRuntimeObservationAccessSnapshotV1 struct {
	ServerID  string                                 `msgpack:"server_id"`
	Whitelist []FleetRuntimeAccessSnapshotIdentityV1 `msgpack:"whitelist,omitempty"`
	Operators []FleetRuntimeAccessSnapshotOperatorV1 `msgpack:"operators,omitempty"`
	Bans      []FleetRuntimeAccessSnapshotBanV1      `msgpack:"bans,omitempty"`
	UpdatedAt string                                 `msgpack:"updated_at"`
}

// FleetRuntimeObservedArtifactV1 is read-only evidence of one installed
// artifact. It intentionally carries no approval, reference, or mutation
// authority. HashSHA256 is optional; when present it is lowercase hexadecimal
// without a "sha256:" prefix.
type FleetRuntimeObservedArtifactV1 struct {
	Name       string `msgpack:"name"`
	Kind       string `msgpack:"kind"`
	HashSHA256 string `msgpack:"hash_sha256,omitempty"`
}

// FleetRuntimeObservationDataV1 contains exactly one member selected by the
// enclosing field enum.
type FleetRuntimeObservationDataV1 struct {
	Settings       map[string]string                        `msgpack:"settings,omitempty"`
	GameRules      map[string]string                        `msgpack:"gamerules,omitempty"`
	Players        []string                                 `msgpack:"players,omitempty"`
	PlayerHistory  []FleetRuntimePlayerHistoryEntryV1       `msgpack:"player_history,omitempty"`
	Access         *FleetRuntimeObservationAccessV1         `msgpack:"access,omitempty"`
	AccessSnapshot *FleetRuntimeObservationAccessSnapshotV1 `msgpack:"access_snapshot,omitempty"`
	Artifacts      []FleetRuntimeObservedArtifactV1         `msgpack:"artifacts,omitempty"`
	Status         string                                   `msgpack:"status,omitempty"`
}

type FleetRuntimeObservationReceiptV1 struct {
	Contract       string                         `msgpack:"contract"`
	ServerID       string                         `msgpack:"server_id"`
	NodeID         string                         `msgpack:"node_id"`
	ContainerID    string                         `msgpack:"container_id"`
	Field          FleetRuntimeObservationFieldV1 `msgpack:"field"`
	Generation     string                         `msgpack:"generation"`
	SourceRevision string                         `msgpack:"source_revision"`
	ObservedAt     string                         `msgpack:"observed_at"`
	Data           FleetRuntimeObservationDataV1  `msgpack:"data"`
}

func (value FleetRuntimeObservationIntentV1) Validate() error {
	if value.Contract != FleetRuntimeObservationContractV1 ||
		!fleetRuntimeObservationName.MatchString(value.ServerID) ||
		!fleetRuntimeObservationName.MatchString(value.NodeID) ||
		!fleetRuntimeObservationName.MatchString(value.ContainerID) ||
		!validFleetRuntimeObservationFieldV1(value.Field) ||
		!validFleetRuntimeObservationRevisionV1(value.Generation) ||
		value.SourceRevision != value.Generation {
		return fmt.Errorf("fleet runtime observation intent is invalid")
	}
	return nil
}

func (value FleetRuntimeObservationReceiptV1) Validate() error {
	if value.Contract != FleetRuntimeObservationContractV1 ||
		!fleetRuntimeObservationName.MatchString(value.ServerID) ||
		!fleetRuntimeObservationName.MatchString(value.NodeID) ||
		!fleetRuntimeObservationName.MatchString(value.ContainerID) ||
		!validFleetRuntimeObservationFieldV1(value.Field) ||
		!validFleetRuntimeObservationRevisionV1(value.Generation) ||
		value.SourceRevision != value.Generation {
		return fmt.Errorf("fleet runtime observation result identity is invalid")
	}
	if _, err := time.Parse(time.RFC3339Nano, value.ObservedAt); err != nil {
		return fmt.Errorf("fleet runtime observation observed_at must be RFC3339")
	}
	return validateFleetRuntimeObservationDataV1(value.Field, value.ServerID, value.Data)
}

func (value FleetRuntimeObservationReceiptV1) ValidateFor(intent FleetRuntimeObservationIntentV1) error {
	if err := intent.Validate(); err != nil {
		return err
	}
	if err := value.Validate(); err != nil {
		return err
	}
	if value.Contract != intent.Contract || value.ServerID != intent.ServerID ||
		value.NodeID != intent.NodeID || value.ContainerID != intent.ContainerID ||
		value.Field != intent.Field || value.Generation != intent.Generation ||
		value.SourceRevision != intent.SourceRevision {
		return fmt.Errorf("fleet runtime observation result does not match intent")
	}
	return nil
}

func decodeFleetRuntimeObservationIntentV1(raw msgpack.RawMessage) (FleetRuntimeObservationIntentV1, error) {
	var value FleetRuntimeObservationIntentV1
	decoder := msgpack.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields(true)
	if err := decoder.Decode(&value); err != nil {
		return value, fmt.Errorf("decode fleet runtime observation intent: %w", err)
	}
	return value, nil
}

func decodeFleetRuntimeObservationReceiptV1(raw msgpack.RawMessage) (FleetRuntimeObservationReceiptV1, error) {
	var value FleetRuntimeObservationReceiptV1
	decoder := msgpack.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields(true)
	if err := decoder.Decode(&value); err != nil {
		return value, fmt.Errorf("decode fleet runtime observation result: %w", err)
	}
	return value, nil
}

func validateFleetRuntimeObservationDataV1(field FleetRuntimeObservationFieldV1, serverID string, data FleetRuntimeObservationDataV1) error {
	present := 0
	if data.Settings != nil {
		present++
	}
	if data.GameRules != nil {
		present++
	}
	if data.Players != nil {
		present++
	}
	if data.PlayerHistory != nil {
		present++
	}
	if data.Access != nil {
		present++
	}
	if data.AccessSnapshot != nil {
		present++
	}
	if data.Artifacts != nil {
		present++
	}
	if data.Status != "" {
		present++
	}
	if present != 1 {
		return fmt.Errorf("fleet runtime observation result must contain exactly one typed field")
	}

	switch field {
	case FleetRuntimeObservationFieldSettingsV1:
		if data.Settings == nil {
			return fmt.Errorf("fleet runtime observation settings result is required")
		}
		return validateFleetRuntimeObservationMapV1(data.Settings)
	case FleetRuntimeObservationFieldGameRulesV1:
		if data.GameRules == nil {
			return fmt.Errorf("fleet runtime observation gamerules result is required")
		}
		return validateFleetRuntimeObservationMapV1(data.GameRules)
	case FleetRuntimeObservationFieldPlayersV1:
		if data.Players == nil || len(data.Players) > 1000 {
			return fmt.Errorf("fleet runtime observation players result exceeds limit")
		}
		for _, player := range data.Players {
			if !fleetRuntimeObservationName.MatchString(player) {
				return fmt.Errorf("fleet runtime observation player is invalid")
			}
		}
		return nil
	case FleetRuntimeObservationFieldPlayerHistoryV1:
		if data.PlayerHistory == nil {
			return fmt.Errorf("fleet runtime observation player history result is required")
		}
		return validateFleetRuntimePlayerHistoryV1(data.PlayerHistory)
	case FleetRuntimeObservationFieldAccessV1:
		if data.Access == nil || data.Access.ServerID != serverID {
			return fmt.Errorf("fleet runtime observation access result is invalid")
		}
		return validateFleetRuntimeObservationAccessV1(*data.Access)
	case FleetRuntimeObservationFieldAccessSnapshotV1:
		if data.AccessSnapshot == nil || data.AccessSnapshot.ServerID != serverID {
			return fmt.Errorf("fleet runtime observation access snapshot result is invalid")
		}
		return validateFleetRuntimeObservationAccessSnapshotV1(*data.AccessSnapshot)
	case FleetRuntimeObservationFieldArtifactsV1:
		if data.Artifacts == nil {
			return fmt.Errorf("fleet runtime observation artifacts result is invalid")
		}
		return validateFleetRuntimeObservationArtifactsV1(data.Artifacts)
	case FleetRuntimeObservationFieldStatusV1:
		if !fleetRuntimeObservationName.MatchString(data.Status) {
			return fmt.Errorf("fleet runtime observation status result is invalid")
		}
		return nil
	default:
		return fmt.Errorf("fleet runtime observation field %q is not allowlisted", field)
	}
}

func validateFleetRuntimeObservationMapV1(value map[string]string) error {
	if len(value) > 128 {
		return fmt.Errorf("fleet runtime observation map exceeds 128 entries")
	}
	for key, item := range value {
		if !fleetRuntimeObservationName.MatchString(key) || len(item) > 4096 || strings.IndexFunc(item, unicode.IsControl) >= 0 {
			return fmt.Errorf("fleet runtime observation map entry is invalid")
		}
	}
	return nil
}

func validateFleetRuntimeObservationAccessV1(value FleetRuntimeObservationAccessV1) error {
	for _, group := range [][]string{value.Whitelist, value.Operators, value.Bans} {
		if len(group) > 1000 {
			return fmt.Errorf("fleet runtime observation access exceeds limit")
		}
		for _, subject := range group {
			if !fleetRuntimeObservationName.MatchString(subject) {
				return fmt.Errorf("fleet runtime observation access subject is invalid")
			}
		}
	}
	return nil
}

func validateFleetRuntimePlayerHistoryV1(value []FleetRuntimePlayerHistoryEntryV1) error {
	if len(value) > 1000 {
		return fmt.Errorf("fleet runtime observation player history exceeds limit")
	}
	seen := make(map[string]struct{}, len(value))
	for _, item := range value {
		if !validFleetRuntimeObservationUUIDV1(item.UUID) ||
			!validFleetRuntimeObservationPlayerNameV1(item.Name) ||
			!validFleetRuntimeObservationBoundedTextV1(item.ExpiresOn, 64) {
			return fmt.Errorf("fleet runtime observation player history entry is invalid")
		}
		if _, exists := seen[item.UUID]; exists {
			return fmt.Errorf("fleet runtime observation player history contains duplicate UUID")
		}
		seen[item.UUID] = struct{}{}
	}
	return nil
}

func validateFleetRuntimeObservationAccessSnapshotV1(value FleetRuntimeObservationAccessSnapshotV1) error {
	if _, err := time.Parse(time.RFC3339Nano, value.UpdatedAt); err != nil {
		return fmt.Errorf("fleet runtime observation access snapshot updated_at must be RFC3339")
	}
	if len(value.Whitelist) > 1000 || len(value.Operators) > 1000 || len(value.Bans) > 1000 {
		return fmt.Errorf("fleet runtime observation access snapshot exceeds limit")
	}
	whitelist := make(map[string]struct{}, len(value.Whitelist))
	for _, item := range value.Whitelist {
		if !validFleetRuntimeObservationUUIDV1(item.UUID) || !validFleetRuntimeObservationPlayerNameV1(item.Name) {
			return fmt.Errorf("fleet runtime observation access snapshot whitelist entry is invalid")
		}
		if _, exists := whitelist[item.UUID]; exists {
			return fmt.Errorf("fleet runtime observation access snapshot whitelist contains duplicate UUID")
		}
		whitelist[item.UUID] = struct{}{}
	}
	operators := make(map[string]struct{}, len(value.Operators))
	for _, item := range value.Operators {
		if !validFleetRuntimeObservationUUIDV1(item.UUID) ||
			!validFleetRuntimeObservationPlayerNameV1(item.Name) ||
			item.Level < 1 || item.Level > 4 {
			return fmt.Errorf("fleet runtime observation access snapshot operator entry is invalid")
		}
		if _, exists := operators[item.UUID]; exists {
			return fmt.Errorf("fleet runtime observation access snapshot operators contain duplicate UUID")
		}
		operators[item.UUID] = struct{}{}
	}
	bans := make(map[string]struct{}, len(value.Bans))
	for _, item := range value.Bans {
		if !validFleetRuntimeObservationUUIDV1(item.UUID) ||
			!validFleetRuntimeObservationPlayerNameV1(item.Name) ||
			!validFleetRuntimeObservationBoundedTextV1(item.Created, 64) ||
			!validFleetRuntimeObservationBoundedTextV1(item.Source, 64) ||
			!validFleetRuntimeObservationBoundedTextV1(item.Expires, 64) ||
			!validFleetRuntimeObservationBoundedTextV1(item.Reason, 512) {
			return fmt.Errorf("fleet runtime observation access snapshot ban entry is invalid")
		}
		if _, exists := bans[item.UUID]; exists {
			return fmt.Errorf("fleet runtime observation access snapshot bans contain duplicate UUID")
		}
		bans[item.UUID] = struct{}{}
	}
	return nil
}

func validateFleetRuntimeObservationArtifactsV1(value []FleetRuntimeObservedArtifactV1) error {
	if len(value) > 256 {
		return fmt.Errorf("fleet runtime observation artifacts exceed limit")
	}
	for _, item := range value {
		if err := item.Validate(); err != nil {
			return err
		}
	}
	return nil
}

func (value FleetRuntimeObservedArtifactV1) Validate() error {
	if !validFleetRuntimeObservedArtifactNameV1(value.Name) {
		return fmt.Errorf("fleet runtime observed artifact name is invalid")
	}
	if value.Kind != "datapack" && value.Kind != "mod" {
		return fmt.Errorf("fleet runtime observed artifact kind is not allowlisted")
	}
	if value.HashSHA256 != "" {
		if len(value.HashSHA256) != 64 {
			return fmt.Errorf("fleet runtime observed artifact hash_sha256 is invalid")
		}
		for _, char := range value.HashSHA256 {
			if !strings.ContainsRune("0123456789abcdef", char) {
				return fmt.Errorf("fleet runtime observed artifact hash_sha256 is invalid")
			}
		}
	}
	return nil
}

func validFleetRuntimeObservedArtifactNameV1(value string) bool {
	if value == "" || len(value) > 255 || strings.TrimSpace(value) != value || value == "." || value == ".." {
		return false
	}
	return !strings.ContainsAny(value, `<>:"/\|?*`) && strings.IndexFunc(value, unicode.IsControl) < 0
}

func validFleetRuntimeObservationFieldV1(value FleetRuntimeObservationFieldV1) bool {
	switch value {
	case FleetRuntimeObservationFieldSettingsV1,
		FleetRuntimeObservationFieldGameRulesV1,
		FleetRuntimeObservationFieldPlayersV1,
		FleetRuntimeObservationFieldPlayerHistoryV1,
		FleetRuntimeObservationFieldAccessV1,
		FleetRuntimeObservationFieldAccessSnapshotV1,
		FleetRuntimeObservationFieldArtifactsV1,
		FleetRuntimeObservationFieldStatusV1:
		return true
	default:
		return false
	}
}

func validFleetRuntimeObservationRevisionV1(value string) bool {
	const prefix = "fleet-live-v1:"
	if !strings.HasPrefix(value, prefix) || len(value) != len(prefix)+64 {
		return false
	}
	_, err := hex.DecodeString(strings.TrimPrefix(value, prefix))
	return err == nil
}

func validFleetRuntimeObservationUUIDV1(value string) bool {
	return fleetRuntimeObservationUUID.MatchString(value)
}

func validFleetRuntimeObservationPlayerNameV1(value string) bool {
	return fleetRuntimeObservationPlayerName.MatchString(value)
}

func validFleetRuntimeObservationBoundedTextV1(value string, limit int) bool {
	return value != "" && len(value) <= limit && strings.TrimSpace(value) == value &&
		strings.IndexFunc(value, unicode.IsControl) < 0
}

var fleetRuntimeObservationName = regexp.MustCompile(`^[A-Za-z0-9_.-]{1,64}$`)
var fleetRuntimeObservationUUID = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)
var fleetRuntimeObservationPlayerName = regexp.MustCompile(`^[A-Za-z0-9_. -]{1,32}$`)
