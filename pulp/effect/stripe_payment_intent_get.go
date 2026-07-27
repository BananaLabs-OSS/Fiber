package effect

import (
	"bytes"
	"fmt"

	"github.com/vmihailenco/msgpack/v5"
)

// StripePaymentIntentGetPayload identifies the PaymentIntent whose provider
// status a state owner needs to verify. It deliberately contains no client
// secret, payment-method, customer, or metadata fields.
type StripePaymentIntentGetPayload struct {
	PaymentIntentID string `msgpack:"payment_intent_id"`
}

// StripePaymentIntentGetResult is the stable, non-secret PaymentIntent status
// surface. The host must not return a client secret, customer data, payment
// method, provider metadata, or provider diagnostics through this receipt.
type StripePaymentIntentGetResult struct {
	PaymentIntentID string `msgpack:"payment_intent_id"`
	Status          string `msgpack:"status"`
	AmountCents     int64  `msgpack:"amount_cents"`
	Currency        string `msgpack:"currency"`
	CaptureMethod   string `msgpack:"capture_method"`
}

func (p StripePaymentIntentGetPayload) Validate() error {
	return validateField("Stripe payment intent id", p.PaymentIntentID)
}

func (r StripePaymentIntentGetResult) Validate() error {
	if err := validateField("Stripe payment intent result payment_intent_id", r.PaymentIntentID); err != nil {
		return err
	}
	if err := validateField("Stripe payment intent result status", r.Status); err != nil {
		return err
	}
	if r.AmountCents < 0 {
		return fmt.Errorf("Stripe payment intent result amount_cents must not be negative")
	}
	if len(r.Currency) != 3 {
		return fmt.Errorf("Stripe payment intent result currency must be a lowercase ISO 4217 code")
	}
	for _, value := range r.Currency {
		if value < 'a' || value > 'z' {
			return fmt.Errorf("Stripe payment intent result currency must be a lowercase ISO 4217 code")
		}
	}
	return validateField("Stripe payment intent result capture_method", r.CaptureMethod)
}

func decodeStripePaymentIntentGetPayload(raw msgpack.RawMessage) (StripePaymentIntentGetPayload, error) {
	var payload StripePaymentIntentGetPayload
	decoder := msgpack.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields(true)
	if err := decoder.Decode(&payload); err != nil {
		return payload, fmt.Errorf("decode Stripe payment intent get payload: %w", err)
	}
	return payload, nil
}

func decodeStripePaymentIntentGetResult(raw msgpack.RawMessage) (StripePaymentIntentGetResult, error) {
	var result StripePaymentIntentGetResult
	decoder := msgpack.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields(true)
	if err := decoder.Decode(&result); err != nil {
		return result, fmt.Errorf("decode Stripe payment intent get result: %w", err)
	}
	return result, nil
}
