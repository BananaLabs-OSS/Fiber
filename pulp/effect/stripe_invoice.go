package effect

import "fmt"

// StripeInvoiceItemCreatePayload creates one invoice item. The enclosing
// intent supplies the provider idempotency key.
type StripeInvoiceItemCreatePayload struct {
	CustomerID  string `msgpack:"customer_id"`
	InvoiceID   string `msgpack:"invoice_id,omitempty"`
	AmountCents int64  `msgpack:"amount_cents"`
	Currency    string `msgpack:"currency"`
	Description string `msgpack:"description,omitempty"`
}

type StripeInvoiceItemCreateResult struct {
	InvoiceItemID string `msgpack:"invoice_item_id"`
}

// StripeInvoiceCreatePayload creates one draft invoice. The application owns
// the ordering of this action relative to all other invoice actions.
type StripeInvoiceCreatePayload struct {
	CustomerID       string            `msgpack:"customer_id"`
	Description      string            `msgpack:"description,omitempty"`
	AutoAdvance      bool              `msgpack:"auto_advance"`
	CollectionMethod string            `msgpack:"collection_method,omitempty"`
	Metadata         map[string]string `msgpack:"metadata,omitempty"`
	PromotionCodeID  string            `msgpack:"promotion_code_id,omitempty"`
}

// StripeInvoiceFinalizePayload finalizes exactly one existing invoice.
type StripeInvoiceFinalizePayload struct {
	InvoiceID string `msgpack:"invoice_id"`
}

// StripeInvoiceMarkPaidPayload marks exactly one existing invoice paid out of
// band, matching the host's Stripe operation.
type StripeInvoiceMarkPaidPayload struct {
	InvoiceID string `msgpack:"invoice_id"`
}

// StripeInvoiceResult is the non-secret acknowledgement returned by create,
// finalize, and mark-paid invoice effects.
type StripeInvoiceResult struct {
	InvoiceID        string `msgpack:"invoice_id"`
	Status           string `msgpack:"status"`
	HostedInvoiceURL string `msgpack:"hosted_invoice_url,omitempty"`
	InvoicePDF       string `msgpack:"invoice_pdf,omitempty"`
	AmountDue        int64  `msgpack:"amount_due"`
	AmountPaid       int64  `msgpack:"amount_paid"`
}

func (p StripeInvoiceItemCreatePayload) Validate() error {
	if err := validateField("Stripe invoice item customer_id", p.CustomerID); err != nil {
		return err
	}
	if p.InvoiceID != "" {
		if err := validateField("Stripe invoice item invoice_id", p.InvoiceID); err != nil {
			return err
		}
	}
	if p.AmountCents <= 0 {
		return fmt.Errorf("Stripe invoice item amount_cents must be positive")
	}
	if err := validateCurrency("Stripe invoice item currency", p.Currency); err != nil {
		return err
	}
	return validateOptionalText("Stripe invoice item description", p.Description)
}

func (r StripeInvoiceItemCreateResult) Validate() error {
	return validateField("Stripe invoice item result invoice_item_id", r.InvoiceItemID)
}

func (p StripeInvoiceCreatePayload) Validate() error {
	if err := validateField("Stripe invoice customer_id", p.CustomerID); err != nil {
		return err
	}
	if p.CollectionMethod != "" && p.CollectionMethod != "charge_automatically" && p.CollectionMethod != "send_invoice" {
		return fmt.Errorf("Stripe invoice collection_method must be charge_automatically or send_invoice")
	}
	if err := validateOptionalText("Stripe invoice description", p.Description); err != nil {
		return err
	}
	if err := validateMetadata(p.Metadata); err != nil {
		return err
	}
	if p.PromotionCodeID != "" {
		return validateField("Stripe invoice promotion_code_id", p.PromotionCodeID)
	}
	return nil
}

func (p StripeInvoiceFinalizePayload) Validate() error {
	return validateField("Stripe invoice finalize invoice_id", p.InvoiceID)
}

func (p StripeInvoiceMarkPaidPayload) Validate() error {
	return validateField("Stripe invoice mark-paid invoice_id", p.InvoiceID)
}

func (r StripeInvoiceResult) Validate() error {
	if err := validateField("Stripe invoice result invoice_id", r.InvoiceID); err != nil {
		return err
	}
	if err := validateField("Stripe invoice result status", r.Status); err != nil {
		return err
	}
	if r.AmountDue < 0 || r.AmountPaid < 0 {
		return fmt.Errorf("Stripe invoice result amounts must not be negative")
	}
	return nil
}
