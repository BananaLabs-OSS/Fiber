// Package stripe is the plugin-side wrapper for the payment.stripe
// capability provided by Pulp-ext-stripe (stripe-go host-side). Plugin
// code calls these methods to create Checkout sessions, verify
// webhook signatures, read payment intents, and issue refunds without
// touching host imports directly.
//
//	import "github.com/BananaLabs-OSS/Fiber/pulp/stripe"
//
//	sess, err := stripe.CreateCheckoutSession(stripe.CheckoutRequest{
//	    AmountCents: 1400,
//	    Currency:    "usd",
//	    ...
//	})
//
// Manifest must declare:
//
//	capabilities = ["payment.stripe"]
//
// The host binary must link Pulp-ext-stripe via blank import and set
// STRIPE_SECRET_KEY (and STRIPE_WEBHOOK_SECRET if using webhooks).
package stripe

import (
	"fmt"
	"runtime"
	"unsafe"

	"github.com/vmihailenco/msgpack/v5"
)

//go:wasmimport pulp stripe_checkout_session_create
func hostCheckoutSessionCreate(reqPtr, reqLen, respPtrOut, respLenOut uint32) uint32

//go:wasmimport pulp stripe_webhook_verify
func hostWebhookVerify(reqPtr, reqLen uint32) uint32

//go:wasmimport pulp stripe_payment_intent_get
func hostPaymentIntentGet(reqPtr, reqLen, respPtrOut, respLenOut uint32) uint32

//go:wasmimport pulp stripe_refund_create
func hostRefundCreate(reqPtr, reqLen, respPtrOut, respLenOut uint32) uint32

// CheckoutRequest is the input to CreateCheckoutSession.
type CheckoutRequest struct {
	AmountCents        int64             `msgpack:"amount_cents"`
	Currency           string            `msgpack:"currency"`
	SuccessURL         string            `msgpack:"success_url"`
	CancelURL          string            `msgpack:"cancel_url"`
	ProductName        string            `msgpack:"product_name"`
	ProductDescription string            `msgpack:"product_description,omitempty"`
	Metadata           map[string]string `msgpack:"metadata,omitempty"`
}

// CheckoutSession is the decoded response — the Session ID (for later
// lookup) and the redirect URL to send the customer to.
type CheckoutSession struct {
	ID  string `msgpack:"id"`
	URL string `msgpack:"url"`
}

// WebhookVerifyRequest carries the raw request body + Stripe-Signature
// header from an inbound webhook request. Use Context.Body() in pulpgin
// to get the raw bytes; Stripe requires the un-parsed payload.
type WebhookVerifyRequest struct {
	Payload         []byte `msgpack:"payload"`
	SignatureHeader string `msgpack:"signature_header"`
}

// PaymentIntent is the decoded response of Get — a subset of Stripe's
// PaymentIntent object with the fields order reconciliation needs.
type PaymentIntent struct {
	ID       string            `msgpack:"id"`
	Status   string            `msgpack:"status"`
	Amount   int64             `msgpack:"amount"`
	Currency string            `msgpack:"currency"`
	Metadata map[string]string `msgpack:"metadata"`
}

// RefundRequest is the input to CreateRefund. AmountCents 0 means
// full refund. Reason is optional — "duplicate", "fraudulent",
// "requested_by_customer" are the valid Stripe values.
type RefundRequest struct {
	PaymentIntentID string `msgpack:"payment_intent_id"`
	AmountCents     int64  `msgpack:"amount_cents,omitempty"`
	Reason          string `msgpack:"reason,omitempty"`
}

// Refund is the decoded response from CreateRefund.
type Refund struct {
	ID     string `msgpack:"id"`
	Status string `msgpack:"status"`
}

// CreateCheckoutSession creates a new Stripe Checkout Session and
// returns the hosted URL the client should be redirected to.
func CreateCheckoutSession(req CheckoutRequest) (CheckoutSession, error) {
	data, err := msgpack.Marshal(req)
	if err != nil {
		return CheckoutSession{}, fmt.Errorf("encode checkout: %w", err)
	}
	var respPtr, respLen uint32
	code := hostCheckoutSessionCreate(
		uint32(uintptr(unsafe.Pointer(&data[0]))),
		uint32(len(data)),
		uint32(uintptr(unsafe.Pointer(&respPtr))),
		uint32(uintptr(unsafe.Pointer(&respLen))),
	)
	runtime.KeepAlive(data)
	if err := codeToError("stripe_checkout_session_create", code); err != nil {
		return CheckoutSession{}, err
	}
	if respLen == 0 {
		return CheckoutSession{}, fmt.Errorf("stripe_checkout_session_create: empty response")
	}
	respBytes := unsafe.Slice((*byte)(unsafe.Pointer(uintptr(respPtr))), respLen)
	var resp CheckoutSession
	if err := msgpack.Unmarshal(respBytes, &resp); err != nil {
		return CheckoutSession{}, fmt.Errorf("decode checkout: %w", err)
	}
	return resp, nil
}

// VerifyWebhook validates a Stripe-Signature header against the raw
// request payload using STRIPE_WEBHOOK_SECRET on the host. Returns
// nil if valid, an error otherwise. Callers should abort the handler
// and respond 400 on signature error.
func VerifyWebhook(payload []byte, signatureHeader string) error {
	req := WebhookVerifyRequest{Payload: payload, SignatureHeader: signatureHeader}
	data, err := msgpack.Marshal(req)
	if err != nil {
		return fmt.Errorf("encode webhook: %w", err)
	}
	code := hostWebhookVerify(uint32(uintptr(unsafe.Pointer(&data[0]))), uint32(len(data)))
	runtime.KeepAlive(data)
	return codeToError("stripe_webhook_verify", code)
}

// GetPaymentIntent fetches a payment intent by ID. Used for order
// reconciliation and refund flows.
func GetPaymentIntent(id string) (PaymentIntent, error) {
	data, err := msgpack.Marshal(struct {
		ID string `msgpack:"id"`
	}{ID: id})
	if err != nil {
		return PaymentIntent{}, fmt.Errorf("encode payment intent get: %w", err)
	}
	var respPtr, respLen uint32
	code := hostPaymentIntentGet(
		uint32(uintptr(unsafe.Pointer(&data[0]))),
		uint32(len(data)),
		uint32(uintptr(unsafe.Pointer(&respPtr))),
		uint32(uintptr(unsafe.Pointer(&respLen))),
	)
	runtime.KeepAlive(data)
	if err := codeToError("stripe_payment_intent_get", code); err != nil {
		return PaymentIntent{}, err
	}
	if respLen == 0 {
		return PaymentIntent{}, fmt.Errorf("stripe_payment_intent_get: empty response")
	}
	respBytes := unsafe.Slice((*byte)(unsafe.Pointer(uintptr(respPtr))), respLen)
	var resp PaymentIntent
	if err := msgpack.Unmarshal(respBytes, &resp); err != nil {
		return PaymentIntent{}, fmt.Errorf("decode payment intent: %w", err)
	}
	return resp, nil
}

// CreateRefund issues a refund against a payment intent. AmountCents
// of 0 means full refund. Returns the created refund with its Stripe
// ID and status (usually "succeeded" or "pending").
func CreateRefund(req RefundRequest) (Refund, error) {
	data, err := msgpack.Marshal(req)
	if err != nil {
		return Refund{}, fmt.Errorf("encode refund: %w", err)
	}
	var respPtr, respLen uint32
	code := hostRefundCreate(
		uint32(uintptr(unsafe.Pointer(&data[0]))),
		uint32(len(data)),
		uint32(uintptr(unsafe.Pointer(&respPtr))),
		uint32(uintptr(unsafe.Pointer(&respLen))),
	)
	runtime.KeepAlive(data)
	if err := codeToError("stripe_refund_create", code); err != nil {
		return Refund{}, err
	}
	if respLen == 0 {
		return Refund{}, fmt.Errorf("stripe_refund_create: empty response")
	}
	respBytes := unsafe.Slice((*byte)(unsafe.Pointer(uintptr(respPtr))), respLen)
	var resp Refund
	if err := msgpack.Unmarshal(respBytes, &resp); err != nil {
		return Refund{}, fmt.Errorf("decode refund: %w", err)
	}
	return resp, nil
}

// codeToError maps Pulp-ext-stripe host error codes to Go errors.
// 99 = capability not declared; 10 = missing STRIPE_SECRET_KEY on host;
// 6 = webhook signature invalid; 4 = Stripe API error.
func codeToError(op string, code uint32) error {
	switch code {
	case 0:
		return nil
	case 99:
		return fmt.Errorf("%s: capability payment.stripe not declared in manifest", op)
	case 10:
		return fmt.Errorf("%s: host missing STRIPE_SECRET_KEY", op)
	case 6:
		return fmt.Errorf("%s: signature invalid", op)
	case 4:
		return fmt.Errorf("%s: stripe api error", op)
	default:
		return fmt.Errorf("%s: host code %d", op, code)
	}
}
