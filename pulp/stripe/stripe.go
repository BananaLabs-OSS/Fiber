// Package stripe is the cell-side wrapper for the payment.stripe
// capability provided by Pulp-ext-stripe (stripe-go host-side). Cell
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
	"errors"
	"fmt"
	"runtime"
	"unsafe"

	"github.com/BananaLabs-OSS/Fiber/pulp"
	"github.com/vmihailenco/msgpack/v5"
)

//go:wasmimport pulp stripe_checkout_session_create
func hostCheckoutSessionCreate(reqPtr, reqLen, respPtrOut, respLenOut uint32) uint32

//go:wasmimport pulp stripe_checkout_session_get
func hostCheckoutSessionGet(reqPtr, reqLen, respPtrOut, respLenOut uint32) uint32

//go:wasmimport pulp stripe_webhook_verify
func hostWebhookVerify(reqPtr, reqLen uint32) uint32

//go:wasmimport pulp stripe_payment_intent_create
func hostPaymentIntentCreate(reqPtr, reqLen, respPtrOut, respLenOut uint32) uint32

//go:wasmimport pulp stripe_payment_intent_get
func hostPaymentIntentGet(reqPtr, reqLen, respPtrOut, respLenOut uint32) uint32

//go:wasmimport pulp stripe_payment_intent_capture
func hostPaymentIntentCapture(reqPtr, reqLen, respPtrOut, respLenOut uint32) uint32

//go:wasmimport pulp stripe_payment_intent_cancel
func hostPaymentIntentCancel(reqPtr, reqLen, respPtrOut, respLenOut uint32) uint32

//go:wasmimport pulp stripe_refund_create
func hostRefundCreate(reqPtr, reqLen, respPtrOut, respLenOut uint32) uint32

//go:wasmimport pulp stripe_customer_create
func hostCustomerCreate(reqPtr, reqLen, respPtrOut, respLenOut uint32) uint32

//go:wasmimport pulp stripe_invoice_create
func hostInvoiceCreate(reqPtr, reqLen, respPtrOut, respLenOut uint32) uint32

//go:wasmimport pulp stripe_invoice_finalize
func hostInvoiceFinalize(reqPtr, reqLen, respPtrOut, respLenOut uint32) uint32

//go:wasmimport pulp stripe_invoice_mark_paid_out_of_band
func hostInvoiceMarkPaidOutOfBand(reqPtr, reqLen, respPtrOut, respLenOut uint32) uint32

//go:wasmimport pulp stripe_invoice_item_create
func hostInvoiceItemCreate(reqPtr, reqLen, respPtrOut, respLenOut uint32) uint32

//go:wasmimport pulp stripe_balance_get
func hostBalanceGet(reqPtr, reqLen, respPtrOut, respLenOut uint32) uint32

// ErrStripeSignatureInvalid is returned by VerifyWebhook when the
// Stripe-Signature header did not validate against the raw payload.
var ErrStripeSignatureInvalid = errors.New("pulp/stripe: webhook signature invalid")

// ErrStripeAPI is a wrapped sentinel for a generic Stripe API error
// (e.g. card declined, invalid params). Callers can errors.Is on it.
var ErrStripeAPI = errors.New("pulp/stripe: api error")

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

// CheckoutSessionDetails is the decoded response of RetrieveCheckoutSession.
// Used for reconciling an order when a user returns from the hosted
// Checkout URL — Status/PaymentStatus indicate whether the payment
// completed; PaymentIntent is the underlying PI id for refunds.
type CheckoutSessionDetails struct {
	ID            string `msgpack:"id"`
	URL           string `msgpack:"url,omitempty"`
	Status        string `msgpack:"status"`
	PaymentIntent string `msgpack:"payment_intent,omitempty"`
	PaymentStatus string `msgpack:"payment_status,omitempty"`
	CustomerEmail string `msgpack:"customer_email,omitempty"`
	AmountTotal   int64  `msgpack:"amount_total"`
	Currency      string `msgpack:"currency,omitempty"`
}

// WebhookVerifyRequest carries the raw request body + Stripe-Signature
// header from an inbound webhook request. Use Context.Body() in pulpgin
// to get the raw bytes; Stripe requires the un-parsed payload.
type WebhookVerifyRequest struct {
	Payload         []byte `msgpack:"payload"`
	SignatureHeader string `msgpack:"signature_header"`
}

// PaymentIntent is the decoded response returned by Create/Get/Capture/
// Cancel — a subset of Stripe's PaymentIntent object with the fields
// the checkout + capture + reconciliation flows need.
type PaymentIntent struct {
	ID            string            `msgpack:"id"`
	Status        string            `msgpack:"status"`
	Amount        int64             `msgpack:"amount"`
	Currency      string            `msgpack:"currency"`
	ClientSecret  string            `msgpack:"client_secret,omitempty"`
	ReceiptEmail  string            `msgpack:"receipt_email,omitempty"`
	CaptureMethod string            `msgpack:"capture_method,omitempty"`
	LatestCharge  string            `msgpack:"latest_charge,omitempty"`
	LastErrorMsg  string            `msgpack:"last_error,omitempty"`
	LastErrorCode string            `msgpack:"last_error_code,omitempty"`
	Metadata      map[string]string `msgpack:"metadata"`
}

// PaymentIntentCreateRequest carries the fields Stripe's PaymentIntent
// create endpoint accepts. CaptureMethod is "automatic" (default) or
// "manual" (for two-phase capture flows like pools).
type PaymentIntentCreateRequest struct {
	AmountCents        int64             `msgpack:"amount_cents"`
	Currency           string            `msgpack:"currency"`
	Description        string            `msgpack:"description,omitempty"`
	ReceiptEmail       string            `msgpack:"receipt_email,omitempty"`
	CaptureMethod      string            `msgpack:"capture_method,omitempty"`
	PaymentMethodTypes []string          `msgpack:"payment_method_types,omitempty"`
	Customer           string            `msgpack:"customer,omitempty"`
	Metadata           map[string]string `msgpack:"metadata,omitempty"`
	IdempotencyKey     string            `msgpack:"idempotency_key,omitempty"`
}

// Customer is a Stripe customer object (ID + email, which are the
// fields billing flows reference).
type Customer struct {
	ID    string `msgpack:"id"`
	Email string `msgpack:"email,omitempty"`
}

// CustomerCreateRequest is the input to CreateCustomer.
type CustomerCreateRequest struct {
	Email       string            `msgpack:"email,omitempty"`
	Name        string            `msgpack:"name,omitempty"`
	Description string            `msgpack:"description,omitempty"`
	Metadata    map[string]string `msgpack:"metadata,omitempty"`
}

// Invoice carries the fields needed to drive the audit-trail flow for
// $0 / free orders.
type Invoice struct {
	ID            string `msgpack:"id"`
	Status        string `msgpack:"status"`
	HostedInvoice string `msgpack:"hosted_invoice_url,omitempty"`
	InvoicePDF    string `msgpack:"invoice_pdf,omitempty"`
	AmountDue     int64  `msgpack:"amount_due"`
	AmountPaid    int64  `msgpack:"amount_paid"`
}

// InvoiceCreateRequest creates a draft invoice for customer; set
// AutoAdvance=true to have Stripe auto-finalize and attempt payment.
type InvoiceCreateRequest struct {
	Customer         string            `msgpack:"customer"`
	Description      string            `msgpack:"description,omitempty"`
	AutoAdvance      bool              `msgpack:"auto_advance,omitempty"`
	CollectionMethod string            `msgpack:"collection_method,omitempty"`
	Metadata         map[string]string `msgpack:"metadata,omitempty"`
}

// InvoiceItem is the response of CreateInvoiceItem — just the ID.
type InvoiceItem struct {
	ID string `msgpack:"id"`
}

// InvoiceItemCreateRequest adds a line item to a customer or an
// existing draft invoice.
type InvoiceItemCreateRequest struct {
	Customer    string `msgpack:"customer"`
	Invoice     string `msgpack:"invoice,omitempty"`
	AmountCents int64  `msgpack:"amount_cents"`
	Currency    string `msgpack:"currency"`
	Description string `msgpack:"description,omitempty"`
}

// Balance is the response of GetBalance.
type Balance struct {
	Available []BalanceAmount `msgpack:"available"`
	Pending   []BalanceAmount `msgpack:"pending"`
}

// BalanceAmount is a per-currency entry in Balance.
type BalanceAmount struct {
	Amount   int64  `msgpack:"amount"`
	Currency string `msgpack:"currency"`
}

// RefundRequest is the input to CreateRefund. AmountCents 0 means
// full refund. Reason is optional — "duplicate", "fraudulent",
// "requested_by_customer" are the valid Stripe values.
type RefundRequest struct {
	PaymentIntentID string            `msgpack:"payment_intent_id"`
	AmountCents     int64             `msgpack:"amount_cents,omitempty"`
	Reason          string            `msgpack:"reason,omitempty"`
	Metadata        map[string]string `msgpack:"metadata,omitempty"`
	IdempotencyKey  string            `msgpack:"idempotency_key,omitempty"`
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

// RetrieveCheckoutSession fetches a Checkout Session by ID. Use after
// the customer returns from the hosted Checkout URL to reconcile order
// state against Stripe before provisioning.
func RetrieveCheckoutSession(id string) (CheckoutSessionDetails, error) {
	data, err := msgpack.Marshal(struct {
		ID string `msgpack:"id"`
	}{ID: id})
	if err != nil {
		return CheckoutSessionDetails{}, fmt.Errorf("encode checkout get: %w", err)
	}
	var respPtr, respLen uint32
	code := hostCheckoutSessionGet(
		uint32(uintptr(unsafe.Pointer(&data[0]))),
		uint32(len(data)),
		uint32(uintptr(unsafe.Pointer(&respPtr))),
		uint32(uintptr(unsafe.Pointer(&respLen))),
	)
	runtime.KeepAlive(data)
	if err := codeToError("stripe_checkout_session_get", code); err != nil {
		return CheckoutSessionDetails{}, err
	}
	if respLen == 0 {
		return CheckoutSessionDetails{}, fmt.Errorf("stripe_checkout_session_get: empty response")
	}
	respBytes := unsafe.Slice((*byte)(unsafe.Pointer(uintptr(respPtr))), respLen)
	var resp CheckoutSessionDetails
	if err := msgpack.Unmarshal(respBytes, &resp); err != nil {
		return CheckoutSessionDetails{}, fmt.Errorf("decode checkout get: %w", err)
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

// CreatePaymentIntent creates a PaymentIntent directly (no Checkout
// Session). Use this for flows where the client uses Stripe Elements /
// confirmCardPayment with a ClientSecret, or for manual-capture pools.
// Returns the PaymentIntent including ClientSecret which the caller
// passes to the front-end.
func CreatePaymentIntent(req PaymentIntentCreateRequest) (PaymentIntent, error) {
	return paymentIntentRoundtrip("stripe_payment_intent_create", hostPaymentIntentCreate, req)
}

// CapturePaymentIntent captures a previously authorized PaymentIntent.
// Used for two-phase capture flows (pools, reservations) where the
// initial Create used CaptureMethod="manual". For retryable callers
// that need idempotency across network errors, use CapturePaymentIntentWithKey.
func CapturePaymentIntent(id string) (PaymentIntent, error) {
	return CapturePaymentIntentWithKey(id, "")
}

// CapturePaymentIntentWithKey is CapturePaymentIntent with a Stripe
// idempotency key attached. Passing the same (id, key) pair on a
// retry is safe — Stripe short-circuits the second call to the
// result of the first. Use when the caller is driven by a job queue
// that may retry on transient failures.
func CapturePaymentIntentWithKey(id, idempotencyKey string) (PaymentIntent, error) {
	return paymentIntentRoundtrip("stripe_payment_intent_capture", hostPaymentIntentCapture, struct {
		ID             string `msgpack:"id"`
		IdempotencyKey string `msgpack:"idempotency_key,omitempty"`
	}{ID: id, IdempotencyKey: idempotencyKey})
}

// CancelPaymentIntent voids an uncaptured PaymentIntent (releases the
// authorization hold). Safe on any status that allows cancellation.
// For retryable callers that need idempotency, use CancelPaymentIntentWithKey.
func CancelPaymentIntent(id string) (PaymentIntent, error) {
	return CancelPaymentIntentWithKey(id, "")
}

// CancelPaymentIntentWithKey is CancelPaymentIntent with a Stripe
// idempotency key attached. See CapturePaymentIntentWithKey for usage.
func CancelPaymentIntentWithKey(id, idempotencyKey string) (PaymentIntent, error) {
	return paymentIntentRoundtrip("stripe_payment_intent_cancel", hostPaymentIntentCancel, struct {
		ID             string `msgpack:"id"`
		IdempotencyKey string `msgpack:"idempotency_key,omitempty"`
	}{ID: id, IdempotencyKey: idempotencyKey})
}

// paymentIntentRoundtrip is the shared marshal/call/unmarshal
// scaffolding for the three PaymentIntent mutators that share a
// response shape.
func paymentIntentRoundtrip(op string, hostFn func(uint32, uint32, uint32, uint32) uint32, req any) (PaymentIntent, error) {
	data, err := msgpack.Marshal(req)
	if err != nil {
		return PaymentIntent{}, fmt.Errorf("encode %s: %w", op, err)
	}
	var respPtr, respLen uint32
	code := hostFn(
		uint32(uintptr(unsafe.Pointer(&data[0]))),
		uint32(len(data)),
		uint32(uintptr(unsafe.Pointer(&respPtr))),
		uint32(uintptr(unsafe.Pointer(&respLen))),
	)
	runtime.KeepAlive(data)
	if err := codeToError(op, code); err != nil {
		return PaymentIntent{}, err
	}
	if respLen == 0 {
		return PaymentIntent{}, fmt.Errorf("%s: empty response", op)
	}
	respBytes := unsafe.Slice((*byte)(unsafe.Pointer(uintptr(respPtr))), respLen)
	var resp PaymentIntent
	if err := msgpack.Unmarshal(respBytes, &resp); err != nil {
		return PaymentIntent{}, fmt.Errorf("decode %s: %w", op, err)
	}
	return resp, nil
}

// CreateCustomer creates a Stripe customer. Used by free-order flows
// that need an invoice audit trail on a Stripe customer record.
func CreateCustomer(req CustomerCreateRequest) (Customer, error) {
	data, err := msgpack.Marshal(req)
	if err != nil {
		return Customer{}, fmt.Errorf("encode customer: %w", err)
	}
	var respPtr, respLen uint32
	code := hostCustomerCreate(
		uint32(uintptr(unsafe.Pointer(&data[0]))),
		uint32(len(data)),
		uint32(uintptr(unsafe.Pointer(&respPtr))),
		uint32(uintptr(unsafe.Pointer(&respLen))),
	)
	runtime.KeepAlive(data)
	if err := codeToError("stripe_customer_create", code); err != nil {
		return Customer{}, err
	}
	if respLen == 0 {
		return Customer{}, fmt.Errorf("stripe_customer_create: empty response")
	}
	respBytes := unsafe.Slice((*byte)(unsafe.Pointer(uintptr(respPtr))), respLen)
	var resp Customer
	if err := msgpack.Unmarshal(respBytes, &resp); err != nil {
		return Customer{}, fmt.Errorf("decode customer: %w", err)
	}
	return resp, nil
}

// CreateInvoice creates a draft invoice against a customer.
// AutoAdvance=true lets Stripe finalize + attempt payment on its own.
func CreateInvoice(req InvoiceCreateRequest) (Invoice, error) {
	return invoiceRoundtrip("stripe_invoice_create", hostInvoiceCreate, req)
}

// FinalizeInvoice finalizes a draft invoice so it can be paid or
// marked out-of-band.
func FinalizeInvoice(id string) (Invoice, error) {
	return invoiceRoundtrip("stripe_invoice_finalize", hostInvoiceFinalize, struct {
		ID string `msgpack:"id"`
	}{ID: id})
}

// MarkInvoicePaidOutOfBand records an invoice as paid by a means
// outside Stripe ($0 free orders, manual cash handling, etc.). The
// invoice moves to the "paid" state with no money movement.
func MarkInvoicePaidOutOfBand(id string) (Invoice, error) {
	return invoiceRoundtrip("stripe_invoice_mark_paid_out_of_band", hostInvoiceMarkPaidOutOfBand, struct {
		ID string `msgpack:"id"`
	}{ID: id})
}

// invoiceRoundtrip is the shared scaffolding for the three invoice
// operations that share a response shape.
func invoiceRoundtrip(op string, hostFn func(uint32, uint32, uint32, uint32) uint32, req any) (Invoice, error) {
	data, err := msgpack.Marshal(req)
	if err != nil {
		return Invoice{}, fmt.Errorf("encode %s: %w", op, err)
	}
	var respPtr, respLen uint32
	code := hostFn(
		uint32(uintptr(unsafe.Pointer(&data[0]))),
		uint32(len(data)),
		uint32(uintptr(unsafe.Pointer(&respPtr))),
		uint32(uintptr(unsafe.Pointer(&respLen))),
	)
	runtime.KeepAlive(data)
	if err := codeToError(op, code); err != nil {
		return Invoice{}, err
	}
	if respLen == 0 {
		return Invoice{}, fmt.Errorf("%s: empty response", op)
	}
	respBytes := unsafe.Slice((*byte)(unsafe.Pointer(uintptr(respPtr))), respLen)
	var resp Invoice
	if err := msgpack.Unmarshal(respBytes, &resp); err != nil {
		return Invoice{}, fmt.Errorf("decode %s: %w", op, err)
	}
	return resp, nil
}

// CreateInvoiceItem adds a line item to a customer or an existing
// draft invoice.
func CreateInvoiceItem(req InvoiceItemCreateRequest) (InvoiceItem, error) {
	data, err := msgpack.Marshal(req)
	if err != nil {
		return InvoiceItem{}, fmt.Errorf("encode invoice item: %w", err)
	}
	var respPtr, respLen uint32
	code := hostInvoiceItemCreate(
		uint32(uintptr(unsafe.Pointer(&data[0]))),
		uint32(len(data)),
		uint32(uintptr(unsafe.Pointer(&respPtr))),
		uint32(uintptr(unsafe.Pointer(&respLen))),
	)
	runtime.KeepAlive(data)
	if err := codeToError("stripe_invoice_item_create", code); err != nil {
		return InvoiceItem{}, err
	}
	if respLen == 0 {
		return InvoiceItem{}, fmt.Errorf("stripe_invoice_item_create: empty response")
	}
	respBytes := unsafe.Slice((*byte)(unsafe.Pointer(uintptr(respPtr))), respLen)
	var resp InvoiceItem
	if err := msgpack.Unmarshal(respBytes, &resp); err != nil {
		return InvoiceItem{}, fmt.Errorf("decode invoice item: %w", err)
	}
	return resp, nil
}

// GetBalance returns the account's current balance — used for health
// checks against Stripe (matches the /v1/balance native call).
func GetBalance() (Balance, error) {
	var respPtr, respLen uint32
	code := hostBalanceGet(
		0, 0,
		uint32(uintptr(unsafe.Pointer(&respPtr))),
		uint32(uintptr(unsafe.Pointer(&respLen))),
	)
	if err := codeToError("stripe_balance_get", code); err != nil {
		return Balance{}, err
	}
	if respLen == 0 {
		return Balance{}, nil
	}
	respBytes := unsafe.Slice((*byte)(unsafe.Pointer(uintptr(respPtr))), respLen)
	var resp Balance
	if err := msgpack.Unmarshal(respBytes, &resp); err != nil {
		return Balance{}, fmt.Errorf("decode balance: %w", err)
	}
	return resp, nil
}

// codeToError maps Pulp-ext-stripe host error codes to Go errors.
// 99 → pulp.ErrCapabilityUnavailable; 10 = missing STRIPE_SECRET_KEY
// on host; 6 = webhook signature invalid; 4 = Stripe API error.
func codeToError(op string, code uint32) error {
	switch code {
	case 0:
		return nil
	case 99:
		return pulp.ErrCapabilityUnavailable
	case 10:
		return fmt.Errorf("%s: host missing STRIPE_SECRET_KEY", op)
	case 6:
		return fmt.Errorf("%s: %w", op, ErrStripeSignatureInvalid)
	case 4:
		return fmt.Errorf("%s: %w", op, ErrStripeAPI)
	default:
		return fmt.Errorf("%s: host code %d", op, code)
	}
}
