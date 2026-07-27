package effect

import (
	"errors"
	"strings"
	"testing"

	"github.com/BananaLabs-OSS/Fiber/pulp"
	"github.com/vmihailenco/msgpack/v5"
)

func TestExecuteFleetEffectCanonicalReceipt(t *testing.T) {
	intent := fleetEffectTestIntent(t)
	receipt, err := NewCompletedReceipt(intent, map[string]string{"server_id": "srv-1"})
	if err != nil {
		t.Fatalf("NewCompletedReceipt: %v", err)
	}
	wire, err := MarshalReceipt(receipt)
	if err != nil {
		t.Fatalf("MarshalReceipt: %v", err)
	}

	restore := replaceFleetEffectHost(t, func(request []byte) ([]byte, uint32) {
		decoded, err := UnmarshalIntent(request)
		if err != nil || decoded.ID != intent.ID || decoded.Kind != intent.Kind || decoded.IdempotencyKey != intent.IdempotencyKey {
			t.Errorf("host intent = %#v, %v", decoded, err)
			return nil, 4
		}
		return wire, 0
	})
	defer restore()

	got, err := ExecuteFleetEffect(intent)
	if err != nil {
		t.Fatalf("ExecuteFleetEffect: %v", err)
	}
	if err := got.ValidateFor(intent); err != nil {
		t.Fatalf("receipt binding: %v", err)
	}
}

func TestExecuteFleetEffectRejectsCapabilityAndInvalidReceipt(t *testing.T) {
	intent := fleetEffectTestIntent(t)
	if _, err := ExecuteFleetEffect(intent); !errors.Is(err, pulp.ErrCapabilityUnavailable) {
		t.Fatalf("native ExecuteFleetEffect error = %v, want ErrCapabilityUnavailable", err)
	}

	wrong, err := NewCompletedReceipt(intent, map[string]string{"server_id": "srv-1"})
	if err != nil {
		t.Fatal(err)
	}
	wrong.IntentID = "other"
	wire, err := msgpack.Marshal(wrong)
	if err != nil {
		t.Fatal(err)
	}
	restore := replaceFleetEffectHost(t, func(request []byte) ([]byte, uint32) {
		return wire, 0
	})
	defer restore()
	if _, err := ExecuteFleetEffect(intent); err == nil {
		t.Fatal("mismatched receipt validated")
	}
}

func TestExecuteFleetEffectRejectsNonFleetIntentBeforeHost(t *testing.T) {
	intent, err := NewIntent("stripe-1", KindStripeCustomerCreate, "stripe-1", StripeCustomerCreatePayload{Email: "owner@example.test"})
	if err != nil {
		t.Fatal(err)
	}
	called := false
	restore := replaceFleetEffectHost(t, func(request []byte) ([]byte, uint32) {
		called = true
		return nil, 0
	})
	defer restore()
	if _, err := ExecuteFleetEffect(intent); err == nil {
		t.Fatal("non-Fleet intent executed")
	}
	if called {
		t.Fatal("host called for non-Fleet intent")
	}
}

func TestExecuteFleetEffectAdmitsExistingSaveFlushExtension(t *testing.T) {
	for _, action := range []string{"save_flush", "restart"} {
		intent, err := NewIntent("fleet-extension-"+action, KindFleetExtensionApply, "fleet-extension-"+action, map[string]string{"rcon_action": action})
		if err != nil {
			t.Fatal(err)
		}
		called := false
		restore := replaceFleetEffectHost(t, func(request []byte) ([]byte, uint32) {
			called = true
			return nil, 99
		})
		_, err = ExecuteFleetEffect(intent)
		restore()
		if action == "save_flush" {
			if !called || !errors.Is(err, pulp.ErrCapabilityUnavailable) {
				t.Fatalf("save_flush call = called:%t err:%v", called, err)
			}
		} else if called || err == nil {
			t.Fatalf("%q extension call = called:%t err:%v", action, called, err)
		}
	}
}

func TestExecuteFleetEffectAdmitsStrictLifecycleExtensions(t *testing.T) {
	tests := []struct {
		extension string
		reason    string
		status    string
	}{
		{extension: "restart", status: "restarted"},
		{extension: "regenerate", reason: "customer-request", status: "regenerated"},
	}
	for _, test := range tests {
		t.Run(test.extension, func(t *testing.T) {
			intent := fleetLifecycleTestIntent(t, fleetLifecyclePayload{
				Extension: test.extension, ServerID: "srv-1", NodeID: "node-1",
				ContainerID: "container-1", Reason: test.reason,
			})
			receipt, err := NewCompletedReceipt(intent, fleetLifecycleResult{
				ServerID: "srv-1", NodeID: "node-1", ContainerID: "container-1",
				Operation: test.extension, Status: test.status, CompletedAt: "2026-07-26T12:34:56.123456789Z",
			})
			if err != nil {
				t.Fatal(err)
			}
			wire, err := MarshalReceipt(receipt)
			if err != nil {
				t.Fatal(err)
			}
			called := false
			restore := replaceFleetEffectHost(t, func(request []byte) ([]byte, uint32) {
				called = true
				return wire, 0
			})
			_, err = ExecuteFleetEffect(intent)
			restore()
			if err != nil {
				t.Fatalf("ExecuteFleetEffect: %v", err)
			}
			if !called {
				t.Fatal("strict lifecycle extension did not reach host")
			}
		})
	}
}

func TestExecuteFleetEffectRejectsLifecyclePayloadRidersBeforeHost(t *testing.T) {
	valid := map[string]any{
		"extension": "restart", "server_id": "srv-1",
		"node_id": "node-1", "container_id": "container-1",
	}
	tests := []struct {
		name    string
		payload map[string]any
	}{
		{name: "rcon action", payload: fleetAnnouncementPayloadWith(valid, "rcon_action", "save_flush")},
		{name: "message", payload: fleetAnnouncementPayloadWith(valid, "message", "stop")},
		{name: "command", payload: fleetAnnouncementPayloadWith(valid, "command", "stop")},
		{name: "cmd", payload: fleetAnnouncementPayloadWith(valid, "cmd", "stop")},
		{name: "rule", payload: fleetAnnouncementPayloadWith(valid, "rule", "difficulty")},
		{name: "value", payload: fleetAnnouncementPayloadWith(valid, "value", "hard")},
		{name: "settings", payload: fleetAnnouncementPayloadWith(valid, "settings", map[string]string{"difficulty": "hard"})},
		{name: "game rules", payload: fleetAnnouncementPayloadWith(valid, "game_rules", map[string]string{"keepInventory": "true"})},
		{name: "resources", payload: fleetAnnouncementPayloadWith(valid, "resources", map[string]int{"memory": 1024})},
		{name: "lease consumer", payload: fleetAnnouncementPayloadWith(valid, "lease_consumer_id", "worker")},
		{name: "lease duration", payload: fleetAnnouncementPayloadWith(valid, "lease_duration_millis", int64(60000))},
		{name: "label", payload: fleetAnnouncementPayloadWith(valid, "label", "scheduled")},
		{name: "unknown", payload: fleetAnnouncementPayloadWith(valid, "unknown", "value")},
		{name: "wrong extension", payload: fleetAnnouncementPayloadWith(valid, "extension", "recreate")},
		{name: "missing extension", payload: fleetPayloadWithout(valid, "extension")},
		{name: "missing server", payload: fleetAnnouncementPayloadWith(valid, "server_id", "")},
		{name: "missing node", payload: fleetAnnouncementPayloadWith(valid, "node_id", "")},
		{name: "missing container", payload: fleetAnnouncementPayloadWith(valid, "container_id", "")},
		{name: "untrimmed reason", payload: fleetAnnouncementPayloadWith(valid, "reason", " customer-request ")},
		{name: "control reason", payload: fleetAnnouncementPayloadWith(valid, "reason", "customer\nrequest")},
		{name: "oversize reason", payload: fleetAnnouncementPayloadWith(valid, "reason", strings.Repeat("a", 257))},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			intent := fleetLifecycleTestIntent(t, test.payload)
			called := false
			restore := replaceFleetEffectHost(t, func(request []byte) ([]byte, uint32) {
				called = true
				return nil, 99
			})
			_, err := ExecuteFleetEffect(intent)
			restore()
			if err == nil {
				t.Fatal("invalid lifecycle payload executed")
			}
			if called {
				t.Fatal("host called for invalid lifecycle payload")
			}
		})
	}
}

func TestExecuteFleetEffectRejectsMismatchedLifecycleResult(t *testing.T) {
	intent := fleetLifecycleTestIntent(t, fleetLifecyclePayload{
		Extension: "restart", ServerID: "srv-1", NodeID: "node-1", ContainerID: "container-1",
	})
	valid := map[string]any{
		"server_id": "srv-1", "node_id": "node-1", "container_id": "container-1",
		"operation": "restart", "status": "restarted", "completed_at": "2026-07-26T12:34:56Z",
	}
	tests := []struct {
		name   string
		result map[string]any
	}{
		{name: "server", result: fleetAnnouncementPayloadWith(valid, "server_id", "other")},
		{name: "node", result: fleetAnnouncementPayloadWith(valid, "node_id", "other")},
		{name: "container", result: fleetAnnouncementPayloadWith(valid, "container_id", "other")},
		{name: "operation", result: fleetAnnouncementPayloadWith(valid, "operation", "regenerate")},
		{name: "missing operation", result: fleetPayloadWithout(valid, "operation")},
		{name: "status", result: fleetAnnouncementPayloadWith(valid, "status", "regenerated")},
		{name: "missing status", result: fleetPayloadWithout(valid, "status")},
		{name: "timestamp", result: fleetAnnouncementPayloadWith(valid, "completed_at", "yesterday")},
		{name: "missing timestamp", result: fleetAnnouncementPayloadWith(valid, "completed_at", "")},
		{name: "extra", result: fleetAnnouncementPayloadWith(valid, "unknown", "value")},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			receipt, err := NewCompletedReceipt(intent, test.result)
			if err != nil {
				t.Fatal(err)
			}
			wire, err := MarshalReceipt(receipt)
			if err != nil {
				t.Fatal(err)
			}
			restore := replaceFleetEffectHost(t, func(request []byte) ([]byte, uint32) {
				return wire, 0
			})
			_, err = ExecuteFleetEffect(intent)
			restore()
			if err == nil {
				t.Fatal("mismatched lifecycle result validated")
			}
		})
	}
}

func TestExecuteFleetEffectAdmitsBoundedAnnouncement(t *testing.T) {
	intent := fleetAnnouncementTestIntent(t, fleetAnnouncementPayload{
		Extension: "rcon", RCONAction: "announce",
		ServerID: "srv-1", NodeID: "node-1", ContainerID: "container-1",
		Message: "Restarting in five minutes.", Reason: "scheduled-restart-warning",
	})
	receipt, err := NewCompletedReceipt(intent, fleetAnnouncementResult{
		ServerID: "srv-1", NodeID: "node-1", ContainerID: "container-1", Status: "rcon_announce",
	})
	if err != nil {
		t.Fatal(err)
	}
	wire, err := MarshalReceipt(receipt)
	if err != nil {
		t.Fatal(err)
	}
	called := false
	restore := replaceFleetEffectHost(t, func(request []byte) ([]byte, uint32) {
		called = true
		return wire, 0
	})
	defer restore()

	if _, err := ExecuteFleetEffect(intent); err != nil {
		t.Fatalf("ExecuteFleetEffect: %v", err)
	}
	if !called {
		t.Fatal("bounded announcement did not reach host")
	}
}

func TestExecuteFleetEffectPreservesFixedInstantExtensionAnnouncement(t *testing.T) {
	intent := fleetAnnouncementTestIntent(t, map[string]any{
		"extension": "rcon", "rcon_action": "announce",
		"server_id": "srv-1", "node_id": "node-1", "container_id": "container-1",
		"message": instantExtensionAnnouncement,
	})
	called := false
	restore := replaceFleetEffectHost(t, func(request []byte) ([]byte, uint32) {
		called = true
		return nil, 99
	})
	defer restore()

	if _, err := ExecuteFleetEffect(intent); !errors.Is(err, pulp.ErrCapabilityUnavailable) {
		t.Fatalf("ExecuteFleetEffect = %v, want ErrCapabilityUnavailable", err)
	}
	if !called {
		t.Fatal("fixed instant-extension announcement did not reach host")
	}
}

func TestExecuteFleetEffectRejectsUnboundedAnnouncementsBeforeHost(t *testing.T) {
	valid := map[string]any{
		"extension": "rcon", "rcon_action": "announce",
		"server_id": "srv-1", "node_id": "node-1", "container_id": "container-1",
		"message": "Expiry warning.", "reason": "expiry-warning",
	}
	tests := []struct {
		name    string
		payload map[string]any
	}{
		{name: "unknown command", payload: fleetAnnouncementPayloadWith(valid, "command", "stop")},
		{name: "unknown cmd", payload: fleetAnnouncementPayloadWith(valid, "cmd", "stop")},
		{name: "unknown rule", payload: fleetAnnouncementPayloadWith(valid, "rule", "difficulty")},
		{name: "unknown value", payload: fleetAnnouncementPayloadWith(valid, "value", "hard")},
		{name: "unknown field", payload: fleetAnnouncementPayloadWith(valid, "extra", "value")},
		{name: "wrong extension", payload: fleetAnnouncementPayloadWith(valid, "extension", "shell")},
		{name: "missing server", payload: fleetAnnouncementPayloadWith(valid, "server_id", "")},
		{name: "missing node", payload: fleetAnnouncementPayloadWith(valid, "node_id", "")},
		{name: "missing container", payload: fleetAnnouncementPayloadWith(valid, "container_id", "")},
		{name: "empty message", payload: fleetAnnouncementPayloadWith(valid, "message", "")},
		{name: "surrounding whitespace", payload: fleetAnnouncementPayloadWith(valid, "message", " warning ")},
		{name: "newline", payload: fleetAnnouncementPayloadWith(valid, "message", "warning\nstop")},
		{name: "tab", payload: fleetAnnouncementPayloadWith(valid, "message", "warning\tstop")},
		{name: "invalid UTF-8", payload: fleetAnnouncementPayloadWith(valid, "message", string([]byte{0xff}))},
		{name: "too long", payload: fleetAnnouncementPayloadWith(valid, "message", strings.Repeat("a", 501))},
		{name: "missing reason", payload: fleetAnnouncementPayloadWith(valid, "reason", "")},
		{name: "wrong reason", payload: fleetAnnouncementPayloadWith(valid, "reason", "admin-message")},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			intent := fleetAnnouncementTestIntent(t, test.payload)
			called := false
			restore := replaceFleetEffectHost(t, func(request []byte) ([]byte, uint32) {
				called = true
				return nil, 99
			})
			_, err := ExecuteFleetEffect(intent)
			restore()
			if err == nil {
				t.Fatal("unbounded announcement executed")
			}
			if called {
				t.Fatal("host called for unbounded announcement")
			}
		})
	}
}

func TestExecuteFleetEffectAcceptsFiveHundredPrintableAnnouncementCharacters(t *testing.T) {
	intent := fleetAnnouncementTestIntent(t, fleetAnnouncementPayload{
		Extension: "rcon", RCONAction: "announce",
		ServerID: "srv-1", NodeID: "node-1", ContainerID: "container-1",
		Message: strings.Repeat("界", 500), Reason: "expiry-warning",
	})
	called := false
	restore := replaceFleetEffectHost(t, func(request []byte) ([]byte, uint32) {
		called = true
		return nil, 99
	})
	defer restore()

	if _, err := ExecuteFleetEffect(intent); !errors.Is(err, pulp.ErrCapabilityUnavailable) {
		t.Fatalf("ExecuteFleetEffect = %v, want ErrCapabilityUnavailable", err)
	}
	if !called {
		t.Fatal("500-character announcement did not reach host")
	}
}

func TestExecuteFleetEffectRejectsMismatchedAnnouncementResult(t *testing.T) {
	intent := fleetAnnouncementTestIntent(t, fleetAnnouncementPayload{
		Extension: "rcon", RCONAction: "announce",
		ServerID: "srv-1", NodeID: "node-1", ContainerID: "container-1",
		Message: "Expiry warning.", Reason: "expiry-warning",
	})
	tests := []struct {
		name   string
		result map[string]string
	}{
		{name: "server", result: map[string]string{"server_id": "other", "node_id": "node-1", "container_id": "container-1", "status": "rcon_announce"}},
		{name: "node", result: map[string]string{"server_id": "srv-1", "node_id": "other", "container_id": "container-1", "status": "rcon_announce"}},
		{name: "container", result: map[string]string{"server_id": "srv-1", "node_id": "node-1", "container_id": "other", "status": "rcon_announce"}},
		{name: "status", result: map[string]string{"server_id": "srv-1", "node_id": "node-1", "container_id": "container-1", "status": "announced"}},
		{name: "extra", result: map[string]string{"server_id": "srv-1", "node_id": "node-1", "container_id": "container-1", "status": "rcon_announce", "extra": "value"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			receipt, err := NewCompletedReceipt(intent, test.result)
			if err != nil {
				t.Fatal(err)
			}
			wire, err := MarshalReceipt(receipt)
			if err != nil {
				t.Fatal(err)
			}
			restore := replaceFleetEffectHost(t, func(request []byte) ([]byte, uint32) {
				return wire, 0
			})
			_, err = ExecuteFleetEffect(intent)
			restore()
			if err == nil {
				t.Fatal("mismatched announcement result validated")
			}
		})
	}
}

func TestFleetEffectCodeError(t *testing.T) {
	if err := fleetEffectCodeError(0); err != nil {
		t.Fatalf("code 0: %v", err)
	}
	if err := fleetEffectCodeError(1); err == nil {
		t.Fatal("code 1 did not fail")
	}
	if err := fleetEffectCodeError(6); err == nil {
		t.Fatal("code 6 did not fail")
	}
	if err := fleetEffectCodeError(88); err == nil {
		t.Fatal("unknown code did not fail")
	}
	if err := fleetEffectCodeError(99); !errors.Is(err, pulp.ErrCapabilityUnavailable) {
		t.Fatalf("code 99 = %v, want ErrCapabilityUnavailable", err)
	}
}

func fleetEffectTestIntent(t *testing.T) Intent {
	t.Helper()
	intent, err := NewIntent("fleet-effect-1", KindFleetServerDeprovision, "fleet:server-1", map[string]string{"server_id": "srv-1"})
	if err != nil {
		t.Fatalf("NewIntent: %v", err)
	}
	return intent
}

func fleetAnnouncementTestIntent[T any](t *testing.T, payload T) Intent {
	t.Helper()
	intent, err := NewIntent("fleet-announcement-1", KindFleetExtensionApply, "fleet-announcement-1", payload)
	if err != nil {
		t.Fatalf("NewIntent: %v", err)
	}
	return intent
}

func fleetLifecycleTestIntent[T any](t *testing.T, payload T) Intent {
	t.Helper()
	intent, err := NewIntent("fleet-lifecycle-1", KindFleetExtensionApply, "fleet-lifecycle-1", payload)
	if err != nil {
		t.Fatalf("NewIntent: %v", err)
	}
	return intent
}

func fleetAnnouncementPayloadWith(base map[string]any, key string, value any) map[string]any {
	clone := make(map[string]any, len(base)+1)
	for field, fieldValue := range base {
		clone[field] = fieldValue
	}
	clone[key] = value
	return clone
}

func fleetPayloadWithout(base map[string]any, key string) map[string]any {
	clone := fleetAnnouncementPayloadWith(base, key, nil)
	delete(clone, key)
	return clone
}

func replaceFleetEffectHost(t *testing.T, host func([]byte) ([]byte, uint32)) func() {
	t.Helper()
	previous := fleetEffectExecuteWire
	fleetEffectExecuteWire = host
	return func() { fleetEffectExecuteWire = previous }
}
