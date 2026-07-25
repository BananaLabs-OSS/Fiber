package effect

import (
	"bytes"
	"encoding/hex"
	"errors"
	"testing"

	"github.com/vmihailenco/msgpack/v5"
)

func TestIntentGoldenWireRoundTrip(t *testing.T) {
	type paymentIntentRequest struct {
		AmountCents int64  `msgpack:"amount_cents"`
		Currency    string `msgpack:"currency"`
	}
	intent, err := NewIntent("effect-1", "stripe.payment_intent.create", "checkout:order-1", paymentIntentRequest{AmountCents: 1200, Currency: "usd"})
	if err != nil {
		t.Fatalf("NewIntent: %v", err)
	}
	wire, err := MarshalIntent(intent)
	if err != nil {
		t.Fatalf("MarshalIntent: %v", err)
	}
	const wantHex = "85a776657273696f6eae70756c702e6566666563742e7631a26964a86566666563742d31a46b696e64d92b70756c702e6566666563742e7374726970652e7061796d656e742d696e74656e742e6372656174652e7631af6964656d706f74656e63795f6b6579b0636865636b6f75743a6f726465722d31a77061796c6f616482ac616d6f756e745f63656e7473d300000000000004b0a863757272656e6379a3757364"
	if got := hex.EncodeToString(wire); got != wantHex {
		t.Fatalf("intent wire = %s, want %s", got, wantHex)
	}
	decoded, err := UnmarshalIntent(wire)
	if err != nil {
		t.Fatalf("UnmarshalIntent: %v", err)
	}
	if decoded.Version != intent.Version || decoded.ID != intent.ID || decoded.Kind != intent.Kind || decoded.IdempotencyKey != intent.IdempotencyKey || !bytes.Equal(decoded.Payload, intent.Payload) {
		t.Fatalf("intent = %#v, want %#v", decoded, intent)
	}
	payload, err := DecodePayload[paymentIntentRequest](decoded)
	if err != nil || payload.AmountCents != 1200 || payload.Currency != "usd" {
		t.Fatalf("payload = %#v, %v", payload, err)
	}
}

func TestReceiptGoldenWireRoundTrip(t *testing.T) {
	intent, err := NewIntent("effect-1", KindStripeRefundCreate, "refund:order-1", map[string]string{"charge": "ch_1"})
	if err != nil {
		t.Fatalf("NewIntent: %v", err)
	}
	receipt, err := NewCompletedReceipt(intent, map[string]string{"refund": "re_1"})
	if err != nil {
		t.Fatalf("NewCompletedReceipt: %v", err)
	}
	wire, err := MarshalReceipt(receipt)
	if err != nil {
		t.Fatalf("MarshalReceipt: %v", err)
	}
	const wantHex = "86a776657273696f6eae70756c702e6566666563742e7631a9696e74656e745f6964a86566666563742d31a46b696e64d92370756c702e6566666563742e7374726970652e726566756e642e6372656174652e7631af6964656d706f74656e63795f6b6579ae726566756e643a6f726465722d31a6737461747573a9636f6d706c65746564a6726573756c7481a6726566756e64a472655f31"
	if got := hex.EncodeToString(wire); got != wantHex {
		t.Fatalf("receipt wire = %s, want %s", got, wantHex)
	}
	decoded, err := UnmarshalReceipt(wire)
	if err != nil {
		t.Fatalf("UnmarshalReceipt: %v", err)
	}
	if err := decoded.ValidateFor(intent); err != nil {
		t.Fatalf("ValidateFor: %v", err)
	}
	result, err := DecodeResult[map[string]string](decoded)
	if err != nil || result["refund"] != "re_1" {
		t.Fatalf("result = %#v, %v", result, err)
	}
}

func TestNormalizeKindLegacyAliases(t *testing.T) {
	tests := map[string]string{
		"payment_intent.create":                    KindStripePaymentIntentCreate,
		"stripe.payment_intent.create":             KindStripePaymentIntentCreate,
		"stripe.checkout_session.create":           KindStripeCheckoutSessionCreate,
		"stripe.setup_intent.create":               KindStripeSetupIntentCreate,
		"stripe.refund":                            KindStripeRefundCreate,
		"fleet.provision.request":                  KindFleetServerProvision,
		"fleet.deprovision.request":                KindFleetServerDeprovision,
		"fleet.extension.apply.v1":                 KindFleetExtensionApply,
		"sessions.notification.extension.ready.v1": KindNotificationEmailSend,
		"workers.email.send":                       KindNotificationEmailSend,
	}
	for alias, want := range tests {
		if got, err := NormalizeKind(alias); err != nil || got != want {
			t.Errorf("NormalizeKind(%q) = %q, %v; want %q", alias, got, err, want)
		}
	}
	if _, err := NormalizeKind("stripe.unknown"); !errors.Is(err, ErrUnsupportedKind) {
		t.Fatalf("unknown kind error = %v", err)
	}
}

func TestUnmarshalIntentNormalizesLegacyWire(t *testing.T) {
	payload, err := msgpack.Marshal(map[string]string{"customer": "cus_1"})
	if err != nil {
		t.Fatal(err)
	}
	wire, err := msgpack.Marshal(Intent{
		Version: VersionV1, ID: "effect-1", Kind: "stripe.payment_intent.create",
		IdempotencyKey: "checkout:order-1", Payload: payload,
	})
	if err != nil {
		t.Fatal(err)
	}
	intent, err := UnmarshalIntent(wire)
	if err != nil {
		t.Fatalf("UnmarshalIntent: %v", err)
	}
	if intent.Kind != KindStripePaymentIntentCreate {
		t.Fatalf("kind = %q", intent.Kind)
	}
}

func TestReceiptStatusValidation(t *testing.T) {
	intent, err := NewIntent("effect-1", KindNotificationEmailSend, "mail:1", map[string]string{"template": "ready"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := NewPendingReceipt(intent); err != nil {
		t.Fatalf("pending receipt: %v", err)
	}
	if _, err := NewCompletedReceipt(intent, map[string]bool{"sent": true}); err != nil {
		t.Fatalf("completed receipt: %v", err)
	}
	if _, err := NewFailedReceipt(intent, Failure{Code: "unavailable", Message: "provider unavailable"}); err != nil {
		t.Fatalf("failed receipt: %v", err)
	}

	bad := receiptFor(intent, Completed)
	if err := bad.ValidateFor(intent); err == nil {
		t.Fatal("completed receipt without result validated")
	}
	bad = receiptFor(intent, Failed)
	bad.Result = msgpack.RawMessage{0xc0}
	bad.Failure = &Failure{Code: "nope", Message: "failed"}
	if err := bad.ValidateFor(intent); err == nil {
		t.Fatal("failed receipt with result validated")
	}
	badIntent := intent
	badIntent.Kind = ""
	if err := badIntent.Validate(); err == nil {
		t.Fatal("intent with empty kind validated")
	}
}
