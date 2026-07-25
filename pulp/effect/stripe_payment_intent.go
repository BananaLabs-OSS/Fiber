package effect

import "fmt"

// StripePaymentIntentCapturePayload is the typed payload for
// KindStripePaymentIntentCapture. The enclosing effect carries the stable
// provider idempotency key.
type StripePaymentIntentCapturePayload struct {
	PaymentIntentID string `msgpack:"payment_intent_id"`
}

// StripePaymentIntentCancelPayload is the typed payload for
// KindStripePaymentIntentCancel.
type StripePaymentIntentCancelPayload struct {
	PaymentIntentID string `msgpack:"payment_intent_id"`
}

// StripePaymentIntentMutationResult is the stable capture/cancel result
// surface. Callers must inspect Status; a successful host call does not imply a
// business state such as paid or canceled.
type StripePaymentIntentMutationResult struct {
	PaymentIntentID string `msgpack:"payment_intent_id"`
	Status          string `msgpack:"status"`
	Amount          int64  `msgpack:"amount"`
	Currency        string `msgpack:"currency"`
	LatestCharge    string `msgpack:"latest_charge,omitempty"`
	LastErrorCode   string `msgpack:"last_error_code,omitempty"`
	LastError       string `msgpack:"last_error,omitempty"`
}

func (p StripePaymentIntentCapturePayload) Validate() error {
	return validateField("Stripe capture payment_intent_id", p.PaymentIntentID)
}

func (p StripePaymentIntentCancelPayload) Validate() error {
	return validateField("Stripe cancel payment_intent_id", p.PaymentIntentID)
}

func (r StripePaymentIntentMutationResult) Validate() error {
	if err := validateField("Stripe payment intent result payment_intent_id", r.PaymentIntentID); err != nil {
		return err
	}
	if err := validateField("Stripe payment intent result status", r.Status); err != nil {
		return err
	}
	if r.Amount < 0 {
		return fmt.Errorf("Stripe payment intent result amount must not be negative")
	}
	if len(r.Currency) != 3 {
		return fmt.Errorf("Stripe payment intent result currency must be a lowercase ISO 4217 code")
	}
	for _, value := range r.Currency {
		if value < 'a' || value > 'z' {
			return fmt.Errorf("Stripe payment intent result currency must be a lowercase ISO 4217 code")
		}
	}
	return nil
}
