package effect

// StripeSetupIntentGetPayload identifies the previously created SetupIntent
// whose browser-completed payment method is needed for an off-session charge.
type StripeSetupIntentGetPayload struct {
	SetupIntentID string `msgpack:"setup_intent_id"`
}

// StripeSetupIntentGetResult is the stable, non-secret subset required by the
// reservation saga. No card number or credential crosses this boundary.
type StripeSetupIntentGetResult struct {
	SetupIntentID string `msgpack:"setup_intent_id"`
	Status        string `msgpack:"status"`
	Customer      string `msgpack:"customer,omitempty"`
	PaymentMethod string `msgpack:"payment_method,omitempty"`
}

func (p StripeSetupIntentGetPayload) Validate() error {
	return validateField("Stripe setup intent id", p.SetupIntentID)
}

func (r StripeSetupIntentGetResult) Validate() error {
	if err := validateField("Stripe setup intent result id", r.SetupIntentID); err != nil {
		return err
	}
	if err := validateField("Stripe setup intent result status", r.Status); err != nil {
		return err
	}
	if r.Customer != "" {
		if err := validateField("Stripe setup intent result customer", r.Customer); err != nil {
			return err
		}
	}
	if r.PaymentMethod != "" {
		return validateField("Stripe setup intent result payment method", r.PaymentMethod)
	}
	return nil
}
