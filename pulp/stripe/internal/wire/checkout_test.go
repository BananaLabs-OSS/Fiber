package wire

import (
	"testing"

	"github.com/vmihailenco/msgpack/v5"
)

func TestCheckoutRequestIdempotencyKeyMessagePackABI(t *testing.T) {
	req := CheckoutRequest{
		AmountCents:    2500,
		Currency:       "usd",
		SuccessURL:     "https://example.test/success",
		CancelURL:      "https://example.test/cancel",
		ProductName:    "Sessions server",
		IdempotencyKey: "checkout:order-42",
	}
	encoded, err := msgpack.Marshal(req)
	if err != nil {
		t.Fatal(err)
	}
	var wire map[string]any
	if err := msgpack.Unmarshal(encoded, &wire); err != nil {
		t.Fatal(err)
	}
	if got, want := wire["idempotency_key"], req.IdempotencyKey; got != want {
		t.Fatalf("idempotency_key = %#v, want %#v", got, want)
	}
}

func TestCheckoutRequestLegacyWireOmitsEmptyIdempotencyKey(t *testing.T) {
	encoded, err := msgpack.Marshal(CheckoutRequest{
		AmountCents: 100,
		Currency:    "usd",
		SuccessURL:  "https://example.test/success",
		CancelURL:   "https://example.test/cancel",
		ProductName: "Legacy checkout",
	})
	if err != nil {
		t.Fatal(err)
	}
	var wire map[string]any
	if err := msgpack.Unmarshal(encoded, &wire); err != nil {
		t.Fatal(err)
	}
	if _, ok := wire["idempotency_key"]; ok {
		t.Fatal("legacy checkout unexpectedly emitted idempotency_key")
	}
}
