package profile

import (
	"errors"
	"testing"

	"github.com/BananaLabs-OSS/Fiber/pulp"
	"github.com/vmihailenco/msgpack/v5"
)

func TestResolveNormalizesOnlyAllowedFields(t *testing.T) {
	wire, err := msgpack.Marshal(Profile{UUID: "123e4567-e89b-12d3-a456-426614174000", Name: "Player_One", Source: PlatformJava})
	if err != nil {
		t.Fatal(err)
	}
	previous := minecraftProfileResolveWire
	minecraftProfileResolveWire = func(request []byte) ([]byte, uint32) {
		var query Query
		if err := msgpack.Unmarshal(request, &query); err != nil {
			t.Error(err)
		}
		if query != (Query{PlayerName: "Player_One", Platform: PlatformJava}) {
			t.Errorf("query = %#v", query)
		}
		return wire, 0
	}
	defer func() { minecraftProfileResolveWire = previous }()
	got, err := Resolve(" Player_One ", "JAVA")
	if err != nil {
		t.Fatal(err)
	}
	if got.Source != PlatformJava || got.Name != "Player_One" {
		t.Fatalf("profile = %#v", got)
	}
}

func TestResolveFailsClosedAndDoesNotCallInvalidInput(t *testing.T) {
	if _, err := Resolve("Player_One", PlatformJava); !errors.Is(err, pulp.ErrCapabilityUnavailable) {
		t.Fatalf("native error = %v", err)
	}
	previous := minecraftProfileResolveWire
	called := false
	minecraftProfileResolveWire = func([]byte) ([]byte, uint32) { called = true; return nil, 0 }
	defer func() { minecraftProfileResolveWire = previous }()
	for _, query := range []Query{{PlayerName: "../../etc", Platform: PlatformJava}, {PlayerName: "ab", Platform: PlatformJava}, {PlayerName: "Player", Platform: "http"}, {PlayerName: "Player\n", Platform: PlatformJava}} {
		if _, err := Resolve(query.PlayerName, query.Platform); !errors.Is(err, ErrInvalidRequest) {
			t.Errorf("Resolve(%#v) = %v", query, err)
		}
	}
	if called {
		t.Fatal("invalid input reached host")
	}
}

func TestResolveRejectsWrongSourceAndMapsNotFound(t *testing.T) {
	previous := minecraftProfileResolveWire
	defer func() { minecraftProfileResolveWire = previous }()
	minecraftProfileResolveWire = func([]byte) ([]byte, uint32) { return nil, 7 }
	if _, err := Resolve("Player_One", PlatformJava); !errors.Is(err, pulp.ErrNotFound) {
		t.Fatalf("not found = %v", err)
	}
	bad, err := msgpack.Marshal(Profile{UUID: "id", Name: "Player_One", Source: PlatformBedrock})
	if err != nil {
		t.Fatal(err)
	}
	minecraftProfileResolveWire = func([]byte) ([]byte, uint32) { return bad, 0 }
	if _, err := Resolve("Player_One", PlatformJava); err == nil {
		t.Fatal("wrong source accepted")
	}
}
