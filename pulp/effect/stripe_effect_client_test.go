package effect

import (
	"bytes"
	"errors"
	"testing"

	"github.com/BananaLabs-OSS/Fiber/pulp"
	"github.com/vmihailenco/msgpack/v5"
)

func TestExecuteStripeEffectCanonicalReceipt(t *testing.T) {
	intent, err := NewIntent(
		"stripe-customer-1", KindStripeCustomerCreate, "stripe:customer:1",
		StripeCustomerCreatePayload{Email: "owner@example.test"},
	)
	if err != nil {
		t.Fatal(err)
	}
	receipt, err := NewCompletedReceipt(intent, StripeCustomerCreateResult{
		CustomerID: "cus_123", Email: "owner@example.test",
	})
	if err != nil {
		t.Fatal(err)
	}
	wire, err := MarshalReceipt(receipt)
	if err != nil {
		t.Fatal(err)
	}
	restore := replaceStripeEffectHost(t, func(request []byte) ([]byte, uint32) {
		var got Intent
		if err := msgpack.Unmarshal(request, &got); err != nil || got.ID != intent.ID || got.Kind != intent.Kind || got.IdempotencyKey != intent.IdempotencyKey || !bytes.Equal(got.Payload, intent.Payload) {
			t.Errorf("host intent = %#v, %v", got, err)
			return nil, 3
		}
		return wire, 0
	})
	defer restore()

	got, err := ExecuteStripeEffect(intent)
	if err != nil {
		t.Fatalf("ExecuteStripeEffect: %v", err)
	}
	if err := got.ValidateFor(intent); err != nil {
		t.Fatalf("receipt binding: %v", err)
	}
}

func TestExecuteStripeEffectRejectsCapabilityAndApplicationSequence(t *testing.T) {
	intent, err := NewIntent(
		"stripe-customer-1", KindStripeCustomerCreate, "stripe:customer:1",
		StripeCustomerCreatePayload{Email: "owner@example.test"},
	)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ExecuteStripeEffect(intent); !errors.Is(err, pulp.ErrCapabilityUnavailable) {
		t.Fatalf("native ExecuteStripeEffect error = %v, want ErrCapabilityUnavailable", err)
	}

	free, err := NewIntent(
		"free-1", KindStripeFreeInvoiceFinalize, "stripe:free:1",
		StripeFreeInvoiceFinalizePayload{
			Customer:    StripeCustomerCreatePayload{Email: "owner@example.test"},
			InvoiceItem: StripeFreeInvoiceItem{AmountCents: 100, Currency: "usd", Description: "item"},
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	called := false
	restore := replaceStripeEffectHost(t, func([]byte) ([]byte, uint32) {
		called = true
		return nil, 99
	})
	defer restore()
	if _, err := ExecuteStripeEffect(free); err == nil {
		t.Fatal("compound free-invoice intent executed")
	}
	if called {
		t.Fatal("host called for application sequencing intent")
	}
}

func TestExecuteStripeEffectRejectsReceiptEnvelopeRider(t *testing.T) {
	intent, err := NewIntent(
		"stripe-customer-1", KindStripeCustomerCreate, "stripe:customer:1",
		StripeCustomerCreatePayload{Email: "owner@example.test"},
	)
	if err != nil {
		t.Fatal(err)
	}
	receipt, err := NewCompletedReceipt(intent, StripeCustomerCreateResult{CustomerID: "cus_123"})
	if err != nil {
		t.Fatal(err)
	}
	value := map[string]any{}
	wire, err := msgpack.Marshal(receipt)
	if err != nil {
		t.Fatal(err)
	}
	if err := msgpack.Unmarshal(wire, &value); err != nil {
		t.Fatal(err)
	}
	value["unexpected"] = "rider"
	wire, err = msgpack.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	restore := replaceStripeEffectHost(t, func([]byte) ([]byte, uint32) { return wire, 0 })
	defer restore()
	if _, err := ExecuteStripeEffect(intent); err == nil {
		t.Fatal("receipt envelope rider accepted")
	}
}

func replaceStripeEffectHost(t *testing.T, host func([]byte) ([]byte, uint32)) func() {
	t.Helper()
	previous := stripeEffectExecuteWire
	stripeEffectExecuteWire = host
	return func() { stripeEffectExecuteWire = previous }
}
