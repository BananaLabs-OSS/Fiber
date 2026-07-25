package effect

import (
	"fmt"
	"strings"
	"unicode"

	"github.com/vmihailenco/msgpack/v5"
)

// StripeCustomerCreatePayload is the typed payload for
// KindStripeCustomerCreate. The enclosing Intent supplies the stable Stripe
// idempotency key; it must not be duplicated in this provider payload.
type StripeCustomerCreatePayload struct {
	Email       string            `msgpack:"email"`
	Name        string            `msgpack:"name,omitempty"`
	Description string            `msgpack:"description,omitempty"`
	Metadata    map[string]string `msgpack:"metadata,omitempty"`
}

// StripeCustomerCreateResult is the stable subset of a created customer that
// checkout state owners are permitted to persist.
type StripeCustomerCreateResult struct {
	CustomerID string `msgpack:"customer_id"`
	Email      string `msgpack:"email,omitempty"`
}

// StripeFreeInvoiceFinalizePayload asks the Stripe host to execute the full
// free-order audit trail as one replay-safe effect: create or replay the
// customer, invoice item, draft invoice, and finalized invoice. The host must
// derive a distinct stable provider idempotency key for every step from the
// enclosing Intent.IdempotencyKey.
type StripeFreeInvoiceFinalizePayload struct {
	Customer    StripeCustomerCreatePayload `msgpack:"customer"`
	InvoiceItem StripeFreeInvoiceItem       `msgpack:"invoice_item"`
	Invoice     StripeFreeInvoice           `msgpack:"invoice"`
}

// StripeFreeInvoiceItem is the gross-price line placed on the free invoice.
// Discounting belongs on the invoice as a Stripe promotion code, not as a
// negative companion line.
type StripeFreeInvoiceItem struct {
	AmountCents int64  `msgpack:"amount_cents"`
	Currency    string `msgpack:"currency"`
	Description string `msgpack:"description"`
}

// StripeFreeInvoice describes the draft invoice before the compound effect
// finalizes it. CollectionMethod may be omitted; the host then uses
// charge_automatically.
type StripeFreeInvoice struct {
	Description      string            `msgpack:"description,omitempty"`
	CollectionMethod string            `msgpack:"collection_method,omitempty"`
	Metadata         map[string]string `msgpack:"metadata,omitempty"`
	PromotionCodeID  string            `msgpack:"promotion_code_id,omitempty"`
}

// StripeFreeInvoiceFinalizeResult is the complete durable acknowledgement of
// the compound effect. A result with a non-zero AmountDue is invalid and must
// never allow commerce to mark the order free or paid.
type StripeFreeInvoiceFinalizeResult struct {
	CustomerID       string `msgpack:"customer_id"`
	InvoiceItemID    string `msgpack:"invoice_item_id"`
	InvoiceID        string `msgpack:"invoice_id"`
	Status           string `msgpack:"status"`
	HostedInvoiceURL string `msgpack:"hosted_invoice_url,omitempty"`
	InvoicePDF       string `msgpack:"invoice_pdf,omitempty"`
	AmountDue        int64  `msgpack:"amount_due"`
	AmountPaid       int64  `msgpack:"amount_paid"`
}

func (p StripeCustomerCreatePayload) Validate() error {
	if err := validateEmail(p.Email); err != nil {
		return err
	}
	if err := validateOptionalText("Stripe customer name", p.Name); err != nil {
		return err
	}
	if err := validateOptionalText("Stripe customer description", p.Description); err != nil {
		return err
	}
	return validateMetadata(p.Metadata)
}

func (r StripeCustomerCreateResult) Validate() error {
	if err := validateField("Stripe customer result customer_id", r.CustomerID); err != nil {
		return err
	}
	if r.Email != "" {
		return validateEmail(r.Email)
	}
	return nil
}

func (p StripeFreeInvoiceFinalizePayload) Validate() error {
	if err := p.Customer.Validate(); err != nil {
		return fmt.Errorf("free invoice customer: %w", err)
	}
	if err := p.InvoiceItem.Validate(); err != nil {
		return fmt.Errorf("free invoice item: %w", err)
	}
	if err := p.Invoice.Validate(); err != nil {
		return fmt.Errorf("free invoice: %w", err)
	}
	return nil
}

func (i StripeFreeInvoiceItem) Validate() error {
	if i.AmountCents <= 0 {
		return fmt.Errorf("amount_cents must be positive")
	}
	if len(i.Currency) != 3 || strings.ToLower(i.Currency) != i.Currency {
		return fmt.Errorf("currency must be a lowercase ISO 4217 code")
	}
	for _, r := range i.Currency {
		if r < 'a' || r > 'z' {
			return fmt.Errorf("currency must be a lowercase ISO 4217 code")
		}
	}
	if strings.TrimSpace(i.Description) == "" {
		return fmt.Errorf("description is required")
	}
	return validateOptionalText("free invoice item description", i.Description)
}

func (i StripeFreeInvoice) Validate() error {
	if i.CollectionMethod != "" && i.CollectionMethod != "charge_automatically" {
		return fmt.Errorf("collection_method must be charge_automatically")
	}
	if err := validateOptionalText("free invoice description", i.Description); err != nil {
		return err
	}
	if err := validateMetadata(i.Metadata); err != nil {
		return err
	}
	if i.PromotionCodeID != "" {
		return validateField("free invoice promotion_code_id", i.PromotionCodeID)
	}
	return nil
}

func (r StripeFreeInvoiceFinalizeResult) Validate() error {
	if err := validateField("free invoice result customer_id", r.CustomerID); err != nil {
		return err
	}
	if err := validateField("free invoice result invoice_item_id", r.InvoiceItemID); err != nil {
		return err
	}
	if err := validateField("free invoice result invoice_id", r.InvoiceID); err != nil {
		return err
	}
	if err := validateField("free invoice result status", r.Status); err != nil {
		return err
	}
	if r.AmountDue != 0 {
		return fmt.Errorf("free invoice result amount_due must be zero")
	}
	if r.AmountPaid < 0 {
		return fmt.Errorf("free invoice result amount_paid must not be negative")
	}
	return nil
}

func validateTypedPayload(kind string, raw msgpack.RawMessage) error {
	switch kind {
	case KindStripePaymentIntentCapture:
		var payload StripePaymentIntentCapturePayload
		if err := msgpack.Unmarshal(raw, &payload); err != nil {
			return fmt.Errorf("decode Stripe payment intent capture payload: %w", err)
		}
		return payload.Validate()
	case KindStripePaymentIntentCancel:
		var payload StripePaymentIntentCancelPayload
		if err := msgpack.Unmarshal(raw, &payload); err != nil {
			return fmt.Errorf("decode Stripe payment intent cancel payload: %w", err)
		}
		return payload.Validate()
	case KindStripeCustomerCreate:
		var payload StripeCustomerCreatePayload
		if err := msgpack.Unmarshal(raw, &payload); err != nil {
			return fmt.Errorf("decode Stripe customer payload: %w", err)
		}
		return payload.Validate()
	case KindStripeFreeInvoiceFinalize:
		var payload StripeFreeInvoiceFinalizePayload
		if err := msgpack.Unmarshal(raw, &payload); err != nil {
			return fmt.Errorf("decode Stripe free invoice payload: %w", err)
		}
		return payload.Validate()
	case KindStripeCouponUpsert:
		var payload StripeCouponUpsertPayload
		if err := msgpack.Unmarshal(raw, &payload); err != nil {
			return fmt.Errorf("decode Stripe coupon upsert payload: %w", err)
		}
		return payload.Validate()
	case KindStripeCouponDelete:
		var payload StripeCouponDeletePayload
		if err := msgpack.Unmarshal(raw, &payload); err != nil {
			return fmt.Errorf("decode Stripe coupon delete payload: %w", err)
		}
		return payload.Validate()
	case KindStripePromotionCodeUpsert:
		var payload StripePromotionCodeUpsertPayload
		if err := msgpack.Unmarshal(raw, &payload); err != nil {
			return fmt.Errorf("decode Stripe promotion-code upsert payload: %w", err)
		}
		return payload.Validate()
	case KindStripePromotionCodeDeactivate:
		var payload StripePromotionCodeDeactivatePayload
		if err := msgpack.Unmarshal(raw, &payload); err != nil {
			return fmt.Errorf("decode Stripe promotion-code deactivate payload: %w", err)
		}
		return payload.Validate()
	default:
		return nil
	}
}

func validateTypedResult(kind string, raw msgpack.RawMessage) error {
	switch kind {
	case KindStripePaymentIntentCapture, KindStripePaymentIntentCancel:
		var result StripePaymentIntentMutationResult
		if err := msgpack.Unmarshal(raw, &result); err != nil {
			return fmt.Errorf("decode Stripe payment intent mutation result: %w", err)
		}
		return result.Validate()
	case KindStripeCustomerCreate:
		var result StripeCustomerCreateResult
		if err := msgpack.Unmarshal(raw, &result); err != nil {
			return fmt.Errorf("decode Stripe customer result: %w", err)
		}
		return result.Validate()
	case KindStripeFreeInvoiceFinalize:
		var result StripeFreeInvoiceFinalizeResult
		if err := msgpack.Unmarshal(raw, &result); err != nil {
			return fmt.Errorf("decode Stripe free invoice result: %w", err)
		}
		return result.Validate()
	case KindStripeCouponUpsert:
		var result StripeCouponUpsertResult
		if err := msgpack.Unmarshal(raw, &result); err != nil {
			return fmt.Errorf("decode Stripe coupon upsert result: %w", err)
		}
		return result.Validate()
	case KindStripeCouponDelete:
		var result StripeCouponDeleteResult
		if err := msgpack.Unmarshal(raw, &result); err != nil {
			return fmt.Errorf("decode Stripe coupon delete result: %w", err)
		}
		return result.Validate()
	case KindStripePromotionCodeUpsert:
		var result StripePromotionCodeUpsertResult
		if err := msgpack.Unmarshal(raw, &result); err != nil {
			return fmt.Errorf("decode Stripe promotion-code upsert result: %w", err)
		}
		return result.Validate()
	case KindStripePromotionCodeDeactivate:
		var result StripePromotionCodeDeactivateResult
		if err := msgpack.Unmarshal(raw, &result); err != nil {
			return fmt.Errorf("decode Stripe promotion-code deactivate result: %w", err)
		}
		return result.Validate()
	default:
		return nil
	}
}

func validateTypedReceiptForIntent(intent Intent, receipt Receipt) error {
	if receipt.Status != Completed {
		return nil
	}
	switch intent.Kind {
	case KindStripePaymentIntentCapture:
		var payload StripePaymentIntentCapturePayload
		var result StripePaymentIntentMutationResult
		if err := msgpack.Unmarshal(intent.Payload, &payload); err != nil {
			return err
		}
		if err := msgpack.Unmarshal(receipt.Result, &result); err != nil {
			return err
		}
		if payload.PaymentIntentID != result.PaymentIntentID {
			return fmt.Errorf("Stripe capture result does not match payment_intent_id")
		}
	case KindStripePaymentIntentCancel:
		var payload StripePaymentIntentCancelPayload
		var result StripePaymentIntentMutationResult
		if err := msgpack.Unmarshal(intent.Payload, &payload); err != nil {
			return err
		}
		if err := msgpack.Unmarshal(receipt.Result, &result); err != nil {
			return err
		}
		if payload.PaymentIntentID != result.PaymentIntentID {
			return fmt.Errorf("Stripe cancel result does not match payment_intent_id")
		}
	case KindStripeCouponUpsert:
		var payload StripeCouponUpsertPayload
		var result StripeCouponUpsertResult
		if err := msgpack.Unmarshal(intent.Payload, &payload); err != nil {
			return err
		}
		if err := msgpack.Unmarshal(receipt.Result, &result); err != nil {
			return err
		}
		if payload.ExternalKey != result.ExternalKey {
			return fmt.Errorf("Stripe coupon result does not match external_key")
		}
	case KindStripeCouponDelete:
		var payload StripeCouponDeletePayload
		var result StripeCouponDeleteResult
		if err := msgpack.Unmarshal(intent.Payload, &payload); err != nil {
			return err
		}
		if err := msgpack.Unmarshal(receipt.Result, &result); err != nil {
			return err
		}
		if payload.ExternalKey != result.ExternalKey || payload.CouponID != result.CouponID {
			return fmt.Errorf("Stripe coupon delete result does not match request")
		}
	case KindStripePromotionCodeUpsert:
		var payload StripePromotionCodeUpsertPayload
		var result StripePromotionCodeUpsertResult
		if err := msgpack.Unmarshal(intent.Payload, &payload); err != nil {
			return err
		}
		if err := msgpack.Unmarshal(receipt.Result, &result); err != nil {
			return err
		}
		if payload.ExternalKey != result.ExternalKey || payload.CouponID != result.CouponID || payload.Code != result.Code {
			return fmt.Errorf("Stripe promotion-code result does not match request")
		}
	case KindStripePromotionCodeDeactivate:
		var payload StripePromotionCodeDeactivatePayload
		var result StripePromotionCodeDeactivateResult
		if err := msgpack.Unmarshal(intent.Payload, &payload); err != nil {
			return err
		}
		if err := msgpack.Unmarshal(receipt.Result, &result); err != nil {
			return err
		}
		if payload.ExternalKey != result.ExternalKey || payload.PromotionCodeID != result.PromotionCodeID {
			return fmt.Errorf("Stripe promotion-code deactivate result does not match request")
		}
	}
	return nil
}

func validateEmail(email string) error {
	if strings.TrimSpace(email) != email || email == "" || !strings.Contains(email, "@") {
		return fmt.Errorf("Stripe customer email is required")
	}
	if len(email) > maxFieldLength {
		return fmt.Errorf("Stripe customer email exceeds %d bytes", maxFieldLength)
	}
	for _, r := range email {
		if unicode.IsControl(r) {
			return fmt.Errorf("Stripe customer email contains a control character")
		}
	}
	return nil
}

func validateOptionalText(label, value string) error {
	if value == "" {
		return nil
	}
	return validateField(label, value)
}

func validateMetadata(metadata map[string]string) error {
	if len(metadata) > 64 {
		return fmt.Errorf("Stripe metadata exceeds 64 entries")
	}
	for key, value := range metadata {
		if err := validateField("Stripe metadata key", key); err != nil {
			return err
		}
		if len(value) > 500 {
			return fmt.Errorf("Stripe metadata value for %q exceeds 500 bytes", key)
		}
		for _, r := range value {
			if unicode.IsControl(r) {
				return fmt.Errorf("Stripe metadata value for %q contains a control character", key)
			}
		}
	}
	return nil
}
