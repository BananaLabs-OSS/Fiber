// Package wire owns the stable MessagePack ABI shared by Fiber's Stripe guest
// wrapper and Pulp-ext-stripe's host import.
package wire

// CheckoutRequest is the input to CreateCheckoutSession.
type CheckoutRequest struct {
	AmountCents        int64             `msgpack:"amount_cents"`
	Currency           string            `msgpack:"currency"`
	SuccessURL         string            `msgpack:"success_url"`
	CancelURL          string            `msgpack:"cancel_url"`
	ProductName        string            `msgpack:"product_name"`
	ProductDescription string            `msgpack:"product_description,omitempty"`
	Metadata           map[string]string `msgpack:"metadata,omitempty"`
	AutomaticTax       bool              `msgpack:"automatic_tax,omitempty"`
	// IdempotencyKey is the stable identity of the durable checkout effect.
	// Leave it empty for the legacy behavior. Retrying with the same non-empty
	// key asks Stripe to return the first Checkout Session instead of creating
	// a duplicate.
	IdempotencyKey string `msgpack:"idempotency_key,omitempty"`
}
