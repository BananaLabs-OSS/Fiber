package effect

import (
	"fmt"
	"math"
)

// StripeCouponUpsertPayload is the immutable Stripe discount shape owned by a
// Commerce row. ExternalKey is the stable Commerce identity used to reconcile
// provider results; the enclosing Intent.IdempotencyKey protects the provider
// mutation itself.
type StripeCouponUpsertPayload struct {
	ExternalKey    string            `msgpack:"external_key"`
	AmountOffCents int64             `msgpack:"amount_off_cents,omitempty"`
	PercentOff     float64           `msgpack:"percent_off,omitempty"`
	Currency       string            `msgpack:"currency,omitempty"`
	Duration       string            `msgpack:"duration"`
	DurationMonths int64             `msgpack:"duration_months,omitempty"`
	MaxRedemptions int64             `msgpack:"max_redemptions,omitempty"`
	RedeemByUnix   int64             `msgpack:"redeem_by_unix,omitempty"`
	Name           string            `msgpack:"name,omitempty"`
	Metadata       map[string]string `msgpack:"metadata,omitempty"`
}

type StripeCouponUpsertResult struct {
	ExternalKey    string  `msgpack:"external_key"`
	CouponID       string  `msgpack:"coupon_id"`
	Valid          bool    `msgpack:"valid"`
	AmountOff      int64   `msgpack:"amount_off,omitempty"`
	PercentOff     float64 `msgpack:"percent_off,omitempty"`
	Currency       string  `msgpack:"currency,omitempty"`
	Duration       string  `msgpack:"duration"`
	DurationMonths int64   `msgpack:"duration_months,omitempty"`
}

type StripeCouponDeletePayload struct {
	ExternalKey string `msgpack:"external_key"`
	CouponID    string `msgpack:"coupon_id"`
}

type StripeCouponDeleteResult struct {
	ExternalKey string `msgpack:"external_key"`
	CouponID    string `msgpack:"coupon_id"`
	Deleted     bool   `msgpack:"deleted"`
}

// StripePromotionCodeUpsertPayload binds a customer-facing code to a Stripe
// Coupon. The Commerce external key remains stable even if a provider object
// is replaced during reconciliation.
type StripePromotionCodeUpsertPayload struct {
	ExternalKey    string            `msgpack:"external_key"`
	CouponID       string            `msgpack:"coupon_id"`
	Code           string            `msgpack:"code"`
	Active         bool              `msgpack:"active"`
	MaxRedemptions int64             `msgpack:"max_redemptions,omitempty"`
	ExpiresAtUnix  int64             `msgpack:"expires_at_unix,omitempty"`
	CustomerID     string            `msgpack:"customer_id,omitempty"`
	Metadata       map[string]string `msgpack:"metadata,omitempty"`
}

type StripePromotionCodeUpsertResult struct {
	ExternalKey     string  `msgpack:"external_key"`
	PromotionCodeID string  `msgpack:"promotion_code_id"`
	CouponID        string  `msgpack:"coupon_id"`
	Code            string  `msgpack:"code"`
	Active          bool    `msgpack:"active"`
	MaxRedemptions  int64   `msgpack:"max_redemptions,omitempty"`
	TimesRedeemed   int64   `msgpack:"times_redeemed"`
	ExpiresAtUnix   int64   `msgpack:"expires_at_unix,omitempty"`
	AmountOff       int64   `msgpack:"amount_off,omitempty"`
	PercentOff      float64 `msgpack:"percent_off,omitempty"`
	Currency        string  `msgpack:"currency,omitempty"`
}

type StripePromotionCodeDeactivatePayload struct {
	ExternalKey     string `msgpack:"external_key"`
	PromotionCodeID string `msgpack:"promotion_code_id"`
}

type StripePromotionCodeDeactivateResult struct {
	ExternalKey     string `msgpack:"external_key"`
	PromotionCodeID string `msgpack:"promotion_code_id"`
	Active          bool   `msgpack:"active"`
}

func (p StripeCouponUpsertPayload) Validate() error {
	if err := validateField("Stripe coupon external_key", p.ExternalKey); err != nil {
		return err
	}
	hasAmount := p.AmountOffCents > 0
	hasPercent := p.PercentOff > 0
	if hasAmount == hasPercent || p.AmountOffCents < 0 || math.IsNaN(p.PercentOff) || math.IsInf(p.PercentOff, 0) || p.PercentOff < 0 || p.PercentOff > 100 {
		return fmt.Errorf("Stripe coupon requires exactly one valid amount_off_cents or percent_off")
	}
	if hasAmount {
		if err := validateCurrency("Stripe coupon currency", p.Currency); err != nil {
			return err
		}
	} else if p.Currency != "" {
		return fmt.Errorf("Stripe percent-off coupon must not contain currency")
	}
	switch p.Duration {
	case "once", "forever":
		if p.DurationMonths != 0 {
			return fmt.Errorf("Stripe coupon duration_months requires repeating duration")
		}
	case "repeating":
		if p.DurationMonths <= 0 {
			return fmt.Errorf("Stripe repeating coupon requires positive duration_months")
		}
	default:
		return fmt.Errorf("Stripe coupon duration must be once, forever, or repeating")
	}
	if p.MaxRedemptions < 0 || p.RedeemByUnix < 0 {
		return fmt.Errorf("Stripe coupon redemption limits must not be negative")
	}
	if err := validateOptionalText("Stripe coupon name", p.Name); err != nil {
		return err
	}
	return validateMetadata(p.Metadata)
}

func (r StripeCouponUpsertResult) Validate() error {
	if err := validateField("Stripe coupon result external_key", r.ExternalKey); err != nil {
		return err
	}
	if err := validateField("Stripe coupon result coupon_id", r.CouponID); err != nil {
		return err
	}
	if !r.Valid {
		return fmt.Errorf("Stripe coupon result must be valid")
	}
	shape := StripeCouponUpsertPayload{
		ExternalKey: r.ExternalKey, AmountOffCents: r.AmountOff,
		PercentOff: r.PercentOff, Currency: r.Currency, Duration: r.Duration,
		DurationMonths: r.DurationMonths,
	}
	return shape.Validate()
}

func (p StripeCouponDeletePayload) Validate() error {
	if err := validateField("Stripe coupon delete external_key", p.ExternalKey); err != nil {
		return err
	}
	return validateField("Stripe coupon delete coupon_id", p.CouponID)
}

func (r StripeCouponDeleteResult) Validate() error {
	if err := validateField("Stripe coupon delete result external_key", r.ExternalKey); err != nil {
		return err
	}
	if err := validateField("Stripe coupon delete result coupon_id", r.CouponID); err != nil {
		return err
	}
	if !r.Deleted {
		return fmt.Errorf("Stripe coupon delete result must be deleted")
	}
	return nil
}

func (p StripePromotionCodeUpsertPayload) Validate() error {
	if err := validateField("Stripe promotion-code external_key", p.ExternalKey); err != nil {
		return err
	}
	if err := validateField("Stripe promotion-code coupon_id", p.CouponID); err != nil {
		return err
	}
	if err := validateField("Stripe promotion-code code", p.Code); err != nil {
		return err
	}
	if p.MaxRedemptions < 0 || p.ExpiresAtUnix < 0 {
		return fmt.Errorf("Stripe promotion-code limits must not be negative")
	}
	if p.CustomerID != "" {
		if err := validateField("Stripe promotion-code customer_id", p.CustomerID); err != nil {
			return err
		}
	}
	return validateMetadata(p.Metadata)
}

func (r StripePromotionCodeUpsertResult) Validate() error {
	if err := validateField("Stripe promotion-code result external_key", r.ExternalKey); err != nil {
		return err
	}
	if err := validateField("Stripe promotion-code result promotion_code_id", r.PromotionCodeID); err != nil {
		return err
	}
	if err := validateField("Stripe promotion-code result coupon_id", r.CouponID); err != nil {
		return err
	}
	if err := validateField("Stripe promotion-code result code", r.Code); err != nil {
		return err
	}
	if r.MaxRedemptions < 0 || r.TimesRedeemed < 0 || r.ExpiresAtUnix < 0 {
		return fmt.Errorf("Stripe promotion-code result counters must not be negative")
	}
	return nil
}

func (p StripePromotionCodeDeactivatePayload) Validate() error {
	if err := validateField("Stripe promotion-code deactivate external_key", p.ExternalKey); err != nil {
		return err
	}
	return validateField("Stripe promotion-code deactivate promotion_code_id", p.PromotionCodeID)
}

func (r StripePromotionCodeDeactivateResult) Validate() error {
	if err := validateField("Stripe promotion-code deactivate result external_key", r.ExternalKey); err != nil {
		return err
	}
	if err := validateField("Stripe promotion-code deactivate result promotion_code_id", r.PromotionCodeID); err != nil {
		return err
	}
	if r.Active {
		return fmt.Errorf("Stripe promotion-code deactivate result must be inactive")
	}
	return nil
}

func validateCurrency(label, currency string) error {
	if len(currency) != 3 {
		return fmt.Errorf("%s must be a lowercase ISO 4217 code", label)
	}
	for _, value := range currency {
		if value < 'a' || value > 'z' {
			return fmt.Errorf("%s must be a lowercase ISO 4217 code", label)
		}
	}
	return nil
}
