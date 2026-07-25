package effect

import (
	"strings"
	"testing"
)

func TestStripePaymentIntentCaptureCancelContracts(t *testing.T) {
	tests := []struct {
		name    string
		alias   string
		kind    string
		payload any
	}{
		{"capture", "stripe.payment_intent.capture", KindStripePaymentIntentCapture, StripePaymentIntentCapturePayload{PaymentIntentID: "pi_1"}},
		{"cancel", "payment_intent.cancel", KindStripePaymentIntentCancel, StripePaymentIntentCancelPayload{PaymentIntentID: "pi_1"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			intent, err := NewIntent(test.name+":1", test.alias, test.name+":pi_1", test.payload)
			if err != nil {
				t.Fatalf("NewIntent: %v", err)
			}
			if intent.Kind != test.kind {
				t.Fatalf("kind = %q, want %q", intent.Kind, test.kind)
			}
			receipt, err := NewCompletedReceipt(intent, StripePaymentIntentMutationResult{
				PaymentIntentID: "pi_1", Status: "succeeded", Amount: 1200, Currency: "usd",
			})
			if err != nil {
				t.Fatalf("NewCompletedReceipt: %v", err)
			}
			if err := receipt.ValidateFor(intent); err != nil {
				t.Fatalf("ValidateFor: %v", err)
			}
		})
	}
}

func TestStripePaymentIntentCaptureCancelValidation(t *testing.T) {
	if _, err := NewIntent("capture:1", KindStripePaymentIntentCapture, "capture:1", StripePaymentIntentCapturePayload{}); err == nil {
		t.Fatal("capture without payment_intent_id validated")
	}
	intent, err := NewIntent("cancel:1", KindStripePaymentIntentCancel, "cancel:1", StripePaymentIntentCancelPayload{PaymentIntentID: "pi_1"})
	if err != nil {
		t.Fatal(err)
	}
	for _, result := range []StripePaymentIntentMutationResult{
		{},
		{PaymentIntentID: "pi_1", Status: "canceled", Amount: -1, Currency: "usd"},
		{PaymentIntentID: "pi_1", Status: "canceled", Amount: 1200, Currency: "USD"},
	} {
		if _, err := NewCompletedReceipt(intent, result); err == nil {
			t.Fatalf("invalid result %#v validated", result)
		}
	}
	if got, err := NormalizeKind("stripe.payment_intent.cancel.v1"); err != nil || got != KindStripePaymentIntentCancel {
		t.Fatalf("cancel alias = %q, %v", got, err)
	}
	if got, err := NormalizeKind("stripe.payment_intent.capture.v1"); err != nil || got != KindStripePaymentIntentCapture {
		t.Fatalf("capture alias = %q, %v", got, err)
	}
	if _, err := NormalizeKind("commerce.v1.contribution.capture"); err == nil || !strings.Contains(err.Error(), "unsupported") {
		t.Fatalf("domain command kind error = %v", err)
	}
}
