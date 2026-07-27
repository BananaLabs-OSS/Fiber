package docker

import (
	"errors"
	"testing"

	"github.com/BananaLabs-OSS/Fiber/pulp"
	"github.com/vmihailenco/msgpack/v5"
)

func TestGetOwnedPreservesLogicalRequestAndCanonicalHostIdentity(t *testing.T) {
	want := Server{ID: "sha256:canonical-container-id", Name: "scope-app-cell-primary", Status: "running"}
	response, err := msgpack.Marshal(want)
	if err != nil {
		t.Fatal(err)
	}
	restore := replaceGetOwnedWire(t, func(request []byte) ([]byte, uint32) {
		var fields map[string]any
		if err := msgpack.Unmarshal(request, &fields); err != nil {
			t.Errorf("decode request: %v", err)
			return nil, 3
		}
		if len(fields) != 1 || fields["logical_name"] != "primary" {
			t.Errorf("request = %#v", fields)
			return nil, 4
		}
		return response, 0
	})
	defer restore()

	got, err := GetOwned("primary")
	if err != nil {
		t.Fatalf("GetOwned: %v", err)
	}
	if got.ID != want.ID || got.Name != want.Name || got.Status != want.Status {
		t.Fatalf("GetOwned = %#v, want %#v", *got, want)
	}
}

func TestGetOwnedFailsClosedAndRejectsInvalidInputs(t *testing.T) {
	if _, err := GetOwned("primary"); !errors.Is(err, pulp.ErrCapabilityUnavailable) {
		t.Fatalf("native GetOwned = %v, want ErrCapabilityUnavailable", err)
	}
	for _, name := range []string{"", " ", " primary", "primary\n"} {
		called := false
		restore := replaceGetOwnedWire(t, func(request []byte) ([]byte, uint32) {
			called = true
			return nil, 0
		})
		_, err := GetOwned(name)
		restore()
		if err == nil || !errors.Is(err, ErrInvalidRequest) {
			t.Errorf("GetOwned(%q) = %v, want ErrInvalidRequest", name, err)
		}
		if called {
			t.Errorf("host called for invalid logical name %q", name)
		}
	}
}

func TestGetOwnedRejectsEmptyCanonicalResult(t *testing.T) {
	for _, server := range []Server{{Name: "scope-primary"}, {ID: "container-id"}} {
		response, err := msgpack.Marshal(server)
		if err != nil {
			t.Fatal(err)
		}
		restore := replaceGetOwnedWire(t, func(request []byte) ([]byte, uint32) {
			return response, 0
		})
		_, err = GetOwned("primary")
		restore()
		if err == nil {
			t.Fatalf("empty canonical identity %#v validated", server)
		}
	}
}

func TestGetOwnedMapsNotFoundWithoutFallback(t *testing.T) {
	called := 0
	restore := replaceGetOwnedWire(t, func(request []byte) ([]byte, uint32) {
		called++
		return nil, 6
	})
	defer restore()
	if _, err := GetOwned("primary"); !errors.Is(err, pulp.ErrNotFound) {
		t.Fatalf("GetOwned = %v, want ErrNotFound", err)
	}
	if called != 1 {
		t.Fatalf("host calls = %d, want one exact lookup", called)
	}
}

func replaceGetOwnedWire(t *testing.T, wire func([]byte) ([]byte, uint32)) func() {
	t.Helper()
	previous := dockerGetOwnedWire
	dockerGetOwnedWire = wire
	return func() { dockerGetOwnedWire = previous }
}
