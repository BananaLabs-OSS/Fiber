// Package profile resolves one Minecraft player's public profile through the
// narrow host-owned identity.minecraft-profile.resolve capability.
//
// This is deliberately a stateless read, not an effect: it has no durable
// intent or receipt envelope. Callers can select only a normalized player name
// and platform; origins, paths, methods, headers, and timeouts are host-owned.
package profile

import (
	"errors"
	"fmt"
	"strings"
	"unicode"

	"github.com/BananaLabs-OSS/Fiber/pulp"
	"github.com/vmihailenco/msgpack/v5"
)

const (
	PlatformJava    = "java"
	PlatformBedrock = "bedrock"
)

var ErrInvalidRequest = errors.New("minecraft_profile_resolve: invalid request")

// Query is the complete guest-controlled input accepted by the broker.
// Platform is either java or bedrock. PlayerName is normalized before it
// crosses the host boundary.
type Query struct {
	PlayerName string `msgpack:"player_name"`
	Platform   string `msgpack:"platform"`
}

// Profile is the canonical public result. UUID is supplied by the selected
// configured profile authority; Source identifies java or bedrock.
type Profile struct {
	UUID   string `msgpack:"uuid"`
	Name   string `msgpack:"name"`
	Source string `msgpack:"source"`
}

// Resolve performs one exact profile lookup through the host capability. It
// fails closed in native builds, where no Pulp host import is present.
func Resolve(playerName, platform string) (Profile, error) {
	query, err := NormalizeQuery(Query{PlayerName: playerName, Platform: platform})
	if err != nil {
		return Profile{}, err
	}
	wire, err := msgpack.Marshal(query)
	if err != nil {
		return Profile{}, fmt.Errorf("minecraft_profile_resolve: encode request: %w", err)
	}
	response, code := minecraftProfileResolveWire(wire)
	if err := profileResolveCodeError(code); err != nil {
		return Profile{}, err
	}
	if len(response) == 0 {
		return Profile{}, errors.New("minecraft_profile_resolve: empty response")
	}
	var result Profile
	if err := msgpack.Unmarshal(response, &result); err != nil {
		return Profile{}, fmt.Errorf("minecraft_profile_resolve: decode response: %w", err)
	}
	if err := result.ValidateFor(query.Platform); err != nil {
		return Profile{}, fmt.Errorf("minecraft_profile_resolve: invalid host response: %w", err)
	}
	return result, nil
}

// NormalizeQuery validates and canonicalizes the only two caller-controlled
// fields. Keeping this identical to the host validation means an invalid query
// never makes a network request, even from a native test seam.
func NormalizeQuery(query Query) (Query, error) {
	if containsControl(query.PlayerName) || containsControl(query.Platform) {
		return Query{}, fmt.Errorf("%w: control characters are not allowed", ErrInvalidRequest)
	}
	query.PlayerName = strings.TrimSpace(query.PlayerName)
	query.Platform = strings.ToLower(strings.TrimSpace(query.Platform))
	if query.Platform != PlatformJava && query.Platform != PlatformBedrock {
		return Query{}, fmt.Errorf("%w: platform must be java or bedrock", ErrInvalidRequest)
	}
	if err := validatePlayerName(query.PlayerName, query.Platform); err != nil {
		return Query{}, err
	}
	return query, nil
}

func (p Profile) ValidateFor(platform string) error {
	if strings.TrimSpace(p.UUID) != p.UUID || p.UUID == "" || len(p.UUID) > 128 || containsControl(p.UUID) {
		return errors.New("uuid is required and must be a bounded printable value")
	}
	if strings.TrimSpace(p.Name) != p.Name {
		return errors.New("name must not have surrounding whitespace")
	}
	if err := validatePlayerName(p.Name, platform); err != nil {
		return fmt.Errorf("name: %w", err)
	}
	if p.Source != platform {
		return fmt.Errorf("source %q does not match requested platform %q", p.Source, platform)
	}
	return nil
}

func validatePlayerName(name, platform string) error {
	if name == "" || len(name) > 32 || containsControl(name) {
		return fmt.Errorf("%w: player_name is required and must be at most 32 bytes", ErrInvalidRequest)
	}
	if platform == PlatformJava && (len(name) < 3 || len(name) > 16) {
		return fmt.Errorf("%w: java player_name must be 3-16 bytes", ErrInvalidRequest)
	}
	for _, r := range name {
		if r > unicode.MaxASCII || !(unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' || (platform == PlatformBedrock && (r == '-' || r == ' '))) {
			return fmt.Errorf("%w: player_name contains unsupported characters", ErrInvalidRequest)
		}
	}
	return nil
}

func containsControl(value string) bool {
	for _, r := range value {
		if unicode.IsControl(r) {
			return true
		}
	}
	return false
}

func profileResolveCodeError(code uint32) error {
	switch code {
	case 0:
		return nil
	case 1:
		return errors.New("minecraft_profile_resolve: empty request")
	case 2:
		return errors.New("minecraft_profile_resolve: request memory read failed")
	case 3:
		return ErrInvalidRequest
	case 4:
		return errors.New("minecraft_profile_resolve: profile authority unavailable")
	case 5:
		return errors.New("minecraft_profile_resolve: invalid profile authority response")
	case 6:
		return errors.New("minecraft_profile_resolve: response allocation or write failed")
	case 7:
		return pulp.ErrNotFound
	case 99:
		return pulp.ErrCapabilityUnavailable
	default:
		return fmt.Errorf("minecraft_profile_resolve: host code %d", code)
	}
}
