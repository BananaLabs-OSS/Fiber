package effect

import (
	"strings"
	"testing"
	"time"
)

func statusSignalIntent(t *testing.T, id, key string) Intent {
	t.Helper()
	intent, err := NewIntent(id, KindStatusSignalPublish, key, StatusSignalPublishPayload{
		Target: StatusSignalTargetPayments, Signal: StatusSignalOK, Detail: "Stripe reachable", ExpiresAtUnix: 1_800_000_000,
	})
	if err != nil {
		t.Fatalf("NewIntent: %v", err)
	}
	return intent
}

func TestStatusSignalPublishContractRejectsUnboundedRoutingAndMismatchedReceipt(t *testing.T) {
	for _, payload := range []StatusSignalPublishPayload{
		{Target: "arbitrary", Signal: StatusSignalOK, Detail: "healthy", ExpiresAtUnix: 1},
		{Target: StatusSignalTargetEmail, Signal: "custom", Detail: "healthy", ExpiresAtUnix: 1},
		{Target: StatusSignalTargetEmail, Signal: StatusSignalOK, Detail: "  unhealthy", ExpiresAtUnix: 1},
		{Target: StatusSignalTargetEmail, Signal: StatusSignalOK, Detail: strings.Repeat("x", maxStatusSignalDetailBytes+1), ExpiresAtUnix: 1},
		{Target: StatusSignalTargetEmail, Signal: StatusSignalOK, Detail: "healthy", ExpiresAtUnix: 0},
	} {
		if _, err := NewIntent("signal-1", KindStatusSignalPublish, "signal-1", payload); err == nil {
			t.Fatalf("NewIntent accepted invalid payload %#v", payload)
		}
	}
	if _, err := NewIntent("signal-url", KindStatusSignalPublish, "signal-url", map[string]any{
		"target": "payments", "signal": "ok", "detail": "healthy", "expires_at_unix": int64(1),
		"url": "https://guest.example/escape",
	}); err == nil {
		t.Fatal("NewIntent accepted a guest-controlled status URL field")
	}
	intent := statusSignalIntent(t, "signal-2", "signal-2")
	if _, err := NewCompletedReceipt(intent, StatusSignalPublishResult{
		Target: StatusSignalTargetEmail, Signal: StatusSignalOK, ExpiresAtUnix: 1_800_000_000,
	}); err == nil {
		t.Fatal("completed receipt accepted a result for a different target")
	}
}

func TestStatusSignalPublishAllowsFixedResolverTargetButRejectsDependencyRouting(t *testing.T) {
	if _, err := NewIntent(
		"signal-resolver", KindStatusSignalPublish, "signal-resolver",
		StatusSignalPublishPayload{
			Target: StatusSignalTargetResolver, Signal: StatusSignalDegraded,
			Detail: "Mojang degraded; Modrinth operational", ExpiresAtUnix: 1_800_000_000,
		},
	); err != nil {
		t.Fatalf("fixed resolver target: %v", err)
	}
	for _, target := range []StatusSignalTarget{
		"dep:mojang", "dep:modrinth", "resolver:mojang", "resolver.mojang",
	} {
		if _, err := NewIntent(
			"signal-"+string(target), KindStatusSignalPublish, "signal-"+string(target),
			StatusSignalPublishPayload{
				Target: target, Signal: StatusSignalDown,
				Detail: "dependency unavailable", ExpiresAtUnix: 1_800_000_000,
			},
		); err == nil {
			t.Fatalf("unbounded dependency target %q was accepted", target)
		}
	}
}

func TestPublishStatusSignalValidatesCanonicalReceiptAndCapabilityStub(t *testing.T) {
	old := statusSignalPublishWire
	t.Cleanup(func() { statusSignalPublishWire = old })
	intent := statusSignalIntent(t, "signal-3", "signal-3")

	statusSignalPublishWire = func(_ []byte) ([]byte, uint32) { return nil, 99 }
	if _, err := PublishStatusSignal(intent); err == nil || !strings.Contains(err.Error(), "capability unavailable") {
		t.Fatalf("stub error = %v", err)
	}

	statusSignalPublishWire = func(_ []byte) ([]byte, uint32) {
		receipt, err := NewCompletedReceipt(intent, StatusSignalPublishResult{
			Target: StatusSignalTargetPayments, Signal: StatusSignalOK, ExpiresAtUnix: 1_800_000_000,
		})
		if err != nil {
			t.Fatal(err)
		}
		wire, err := MarshalReceipt(receipt)
		if err != nil {
			t.Fatal(err)
		}
		return wire, 0
	}
	receipt, err := PublishStatusSignal(intent)
	if err != nil || receipt.Status != Completed {
		t.Fatalf("PublishStatusSignal = (%+v, %v)", receipt, err)
	}
	if _, err := DecodeResult[StatusSignalPublishResult](receipt); err != nil {
		t.Fatalf("decode result: %v", err)
	}
}

func TestStatusSignalExpiryRange(t *testing.T) {
	tooFar := time.Date(10_000, 1, 1, 0, 0, 0, 0, time.UTC).Unix()
	if _, err := NewIntent("signal-4", KindStatusSignalPublish, "signal-4", StatusSignalPublishPayload{
		Target: StatusSignalTargetEmail, Signal: StatusSignalDown, Detail: "delivery unavailable", ExpiresAtUnix: tooFar,
	}); err == nil {
		t.Fatal("NewIntent accepted an out-of-range expiry")
	}
}
