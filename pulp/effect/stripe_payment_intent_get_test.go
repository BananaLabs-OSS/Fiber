package effect

import (
	"testing"

	"github.com/vmihailenco/msgpack/v5"
)

func TestStripePaymentIntentGetContract(t *testing.T) {
	payload := StripePaymentIntentGetPayload{PaymentIntentID: "pi_123"}
	intent, err := NewIntent("payment-intent-get:1", "stripe.payment_intent.get", "payment-intent-get:pi_123", payload)
	if err != nil {
		t.Fatalf("NewIntent: %v", err)
	}
	if intent.Kind != KindStripePaymentIntentGet {
		t.Fatalf("kind = %q, want %q", intent.Kind, KindStripePaymentIntentGet)
	}

	receipt, err := NewCompletedReceipt(intent, StripePaymentIntentGetResult{
		PaymentIntentID: "pi_123", Status: "requires_capture", AmountCents: 1200,
		Currency: "usd", CaptureMethod: "manual",
	})
	if err != nil {
		t.Fatalf("NewCompletedReceipt: %v", err)
	}
	if err := receipt.ValidateFor(intent); err != nil {
		t.Fatalf("ValidateFor: %v", err)
	}
	decoded, err := DecodeResult[StripePaymentIntentGetResult](receipt)
	if err != nil || decoded != (StripePaymentIntentGetResult{PaymentIntentID: "pi_123", Status: "requires_capture", AmountCents: 1200, Currency: "usd", CaptureMethod: "manual"}) {
		t.Fatalf("DecodeResult = %#v, %v", decoded, err)
	}
}

func TestStripePaymentIntentGetRejectsInvalidOrSecretContractFields(t *testing.T) {
	if _, err := NewIntent("payment-intent-get:missing", KindStripePaymentIntentGet, "payment-intent-get:missing", StripePaymentIntentGetPayload{}); err == nil {
		t.Fatal("missing payment_intent_id validated")
	}

	unknownPayload, err := msgpack.Marshal(map[string]any{"payment_intent_id": "pi_123", "client_secret": "must-not-cross"})
	if err != nil {
		t.Fatal(err)
	}
	if err := (Intent{Version: VersionV1, ID: "payment-intent-get:unknown", Kind: KindStripePaymentIntentGet, IdempotencyKey: "payment-intent-get:unknown", Payload: unknownPayload}).Validate(); err == nil {
		t.Fatal("payload with a client secret field validated")
	}

	intent, err := NewIntent("payment-intent-get:1", KindStripePaymentIntentGet, "payment-intent-get:1", StripePaymentIntentGetPayload{PaymentIntentID: "pi_123"})
	if err != nil {
		t.Fatal(err)
	}
	for _, result := range []StripePaymentIntentGetResult{
		{},
		{PaymentIntentID: "pi_123", Status: "requires_capture", AmountCents: -1, Currency: "usd", CaptureMethod: "manual"},
		{PaymentIntentID: "pi_123", Status: "requires_capture", AmountCents: 1200, Currency: "USD", CaptureMethod: "manual"},
		{PaymentIntentID: "pi_123", Status: "requires_capture", AmountCents: 1200, Currency: "usd"},
	} {
		if _, err := NewCompletedReceipt(intent, result); err == nil {
			t.Fatalf("invalid result %#v validated", result)
		}
	}

	unknownResult, err := msgpack.Marshal(map[string]any{
		"payment_intent_id": "pi_123", "status": "requires_capture", "amount_cents": int64(1200), "currency": "usd", "capture_method": "manual", "client_secret": "must-not-cross",
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := (Receipt{Version: VersionV1, IntentID: intent.ID, Kind: intent.Kind, IdempotencyKey: intent.IdempotencyKey, Status: Completed, Result: unknownResult}).Validate(); err == nil {
		t.Fatal("result with a client secret field validated")
	}

	mismatchResult, err := msgpack.Marshal(StripePaymentIntentGetResult{PaymentIntentID: "pi_other", Status: "requires_capture", AmountCents: 1200, Currency: "usd", CaptureMethod: "manual"})
	if err != nil {
		t.Fatal(err)
	}
	mismatch := receiptFor(intent, Completed)
	mismatch.Result = mismatchResult
	if err := mismatch.ValidateFor(intent); err == nil {
		t.Fatal("mismatched payment intent result validated")
	}
}
