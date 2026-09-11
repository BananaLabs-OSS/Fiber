package pulp

import (
	"bytes"
	"errors"
	"strings"
	"testing"
)

func resetSnapshotHooks(t *testing.T) {
	t.Helper()
	snapshotState.Lock()
	oldSnapshot, oldRestore := snapshotState.snapshot, snapshotState.restore
	snapshotState.snapshot, snapshotState.restore = nil, nil
	snapshotState.Unlock()
	t.Cleanup(func() {
		snapshotState.Lock()
		snapshotState.snapshot, snapshotState.restore = oldSnapshot, oldRestore
		snapshotState.Unlock()
	})
}

func TestSnapshotRestoreHooksCopyAcrossBoundary(t *testing.T) {
	resetSnapshotHooks(t)
	original := []byte("state-v2")
	var restored []byte
	OnSnapshot(func() ([]byte, error) { return original, nil })
	OnRestore(func(snapshot []byte) error {
		restored = snapshot
		snapshot[0] = 'X'
		return nil
	})

	snapshot, err := runSnapshot()
	if err != nil {
		t.Fatal(err)
	}
	original[0] = 'X'
	if string(snapshot) != "state-v2" {
		t.Fatalf("snapshot aliases hook memory: %q", snapshot)
	}
	if err := runRestore(snapshot); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(restored, []byte("Xtate-v2")) || string(snapshot) != "state-v2" {
		t.Fatalf("restore boundary aliases input: restored=%q input=%q", restored, snapshot)
	}
}

func TestSnapshotRestoreRequireRegistration(t *testing.T) {
	resetSnapshotHooks(t)
	if _, err := runSnapshot(); err == nil || !strings.Contains(err.Error(), "not registered") {
		t.Fatalf("snapshot error = %v", err)
	}
	if err := runRestore(nil); err == nil || !strings.Contains(err.Error(), "not registered") {
		t.Fatalf("restore error = %v", err)
	}
}

func TestSnapshotRestoreContainPanicsAndErrors(t *testing.T) {
	resetSnapshotHooks(t)
	OnSnapshot(func() ([]byte, error) { panic("capture exploded") })
	if _, err := runSnapshot(); err == nil || !strings.Contains(err.Error(), "capture exploded") {
		t.Fatalf("snapshot panic error = %v", err)
	}
	OnSnapshot(func() ([]byte, error) { return nil, errors.New("capture failed") })
	if _, err := runSnapshot(); err == nil || err.Error() != "capture failed" {
		t.Fatalf("snapshot error = %v", err)
	}
	OnRestore(func([]byte) error { panic("install exploded") })
	if err := runRestore(nil); err == nil || !strings.Contains(err.Error(), "install exploded") {
		t.Fatalf("restore panic error = %v", err)
	}
}

func TestBoundedSnapshotError(t *testing.T) {
	got := boundedSnapshotError(errors.New(strings.Repeat("x", maxSnapshotErrorBytes+10)))
	if len(got) != maxSnapshotErrorBytes || !bytes.HasSuffix(got, []byte("...")) {
		t.Fatalf("bounded error length=%d suffix=%q", len(got), got[len(got)-3:])
	}
}
