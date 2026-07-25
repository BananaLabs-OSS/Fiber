package effect

import (
	"strings"
	"testing"
)

func TestStripeCouponLifecycleContract(t *testing.T) {
	upsertPayload := StripeCouponUpsertPayload{
		ExternalKey: "coupon-local-1", AmountOffCents: 500, Currency: "usd",
		Duration: "once", MaxRedemptions: 10, Name: "Sessions SAVE5",
		Metadata: map[string]string{"source": "sessions.gg"},
	}
	upsert, err := NewIntent("coupon:local-1:upsert", "stripe.coupon.create", "coupon:local-1:stripe-coupon", upsertPayload)
	if err != nil {
		t.Fatalf("coupon upsert intent: %v", err)
	}
	if upsert.Kind != KindStripeCouponUpsert || upsert.IdempotencyKey != "coupon:local-1:stripe-coupon" {
		t.Fatalf("upsert identity = %q/%q", upsert.Kind, upsert.IdempotencyKey)
	}
	upsertReceipt, err := NewCompletedReceipt(upsert, StripeCouponUpsertResult{
		ExternalKey: upsertPayload.ExternalKey, CouponID: "coupon_1", Valid: true,
		AmountOff: 500, Currency: "usd", Duration: "once",
	})
	if err != nil {
		t.Fatalf("coupon upsert receipt: %v", err)
	}
	if err := upsertReceipt.ValidateFor(upsert); err != nil {
		t.Fatalf("coupon upsert binding: %v", err)
	}

	deleteIntent, err := NewIntent("coupon:local-1:delete", KindStripeCouponDelete, "coupon:local-1:stripe-delete", StripeCouponDeletePayload{
		ExternalKey: upsertPayload.ExternalKey, CouponID: "coupon_1",
	})
	if err != nil {
		t.Fatalf("coupon delete intent: %v", err)
	}
	deleteReceipt, err := NewCompletedReceipt(deleteIntent, StripeCouponDeleteResult{
		ExternalKey: upsertPayload.ExternalKey, CouponID: "coupon_1", Deleted: true,
	})
	if err != nil {
		t.Fatalf("coupon delete receipt: %v", err)
	}
	if err := deleteReceipt.ValidateFor(deleteIntent); err != nil {
		t.Fatalf("coupon delete binding: %v", err)
	}
}

func TestStripePromotionCodeLifecycleContract(t *testing.T) {
	payload := StripePromotionCodeUpsertPayload{
		ExternalKey: "promotion-local-1", CouponID: "coupon_1", Code: "SAVE5", Active: true,
		MaxRedemptions: 10, Metadata: map[string]string{"source": "sessions.gg"},
	}
	upsert, err := NewIntent("promotion:local-1:upsert", "promotion_code.create", "promotion:local-1:stripe-promo", payload)
	if err != nil {
		t.Fatalf("promotion upsert intent: %v", err)
	}
	receipt, err := NewCompletedReceipt(upsert, StripePromotionCodeUpsertResult{
		ExternalKey: payload.ExternalKey, PromotionCodeID: "promo_1", CouponID: payload.CouponID,
		Code: payload.Code, Active: true, MaxRedemptions: 10,
	})
	if err != nil {
		t.Fatalf("promotion upsert receipt: %v", err)
	}
	if err := receipt.ValidateFor(upsert); err != nil {
		t.Fatalf("promotion upsert binding: %v", err)
	}

	deactivate, err := NewIntent("promotion:local-1:deactivate", KindStripePromotionCodeDeactivate, "promotion:local-1:stripe-deactivate", StripePromotionCodeDeactivatePayload{
		ExternalKey: payload.ExternalKey, PromotionCodeID: "promo_1",
	})
	if err != nil {
		t.Fatalf("promotion deactivate intent: %v", err)
	}
	deactivated, err := NewCompletedReceipt(deactivate, StripePromotionCodeDeactivateResult{
		ExternalKey: payload.ExternalKey, PromotionCodeID: "promo_1", Active: false,
	})
	if err != nil {
		t.Fatalf("promotion deactivate receipt: %v", err)
	}
	if err := deactivated.ValidateFor(deactivate); err != nil {
		t.Fatalf("promotion deactivate binding: %v", err)
	}
}

func TestStripePromotionValidation(t *testing.T) {
	validCoupon := StripeCouponUpsertPayload{
		ExternalKey: "coupon-1", AmountOffCents: 100, Currency: "usd", Duration: "once",
	}
	invalidCoupons := []StripeCouponUpsertPayload{
		{},
		{ExternalKey: "coupon-1", Duration: "once"},
		{ExternalKey: "coupon-1", AmountOffCents: 100, PercentOff: 10, Currency: "usd", Duration: "once"},
		{ExternalKey: "coupon-1", AmountOffCents: 100, Currency: "USD", Duration: "once"},
		{ExternalKey: "coupon-1", PercentOff: 101, Duration: "once"},
		{ExternalKey: "coupon-1", PercentOff: 10, Currency: "usd", Duration: "once"},
		{ExternalKey: "coupon-1", PercentOff: 10, Duration: "repeating"},
		{ExternalKey: "coupon-1", PercentOff: 10, Duration: "forever", DurationMonths: 2},
	}
	for _, payload := range invalidCoupons {
		if _, err := NewIntent("coupon:1", KindStripeCouponUpsert, "coupon:1", payload); err == nil {
			t.Fatalf("invalid coupon %#v validated", payload)
		}
	}
	if _, err := NewIntent("coupon:1", KindStripeCouponUpsert, "coupon:1", validCoupon); err != nil {
		t.Fatalf("valid coupon: %v", err)
	}
	if _, err := NewIntent("promotion:1", KindStripePromotionCodeUpsert, "promotion:1", StripePromotionCodeUpsertPayload{}); err == nil {
		t.Fatal("empty promotion upsert validated")
	}
	if _, err := NewIntent("promotion:1", KindStripePromotionCodeDeactivate, "promotion:1", StripePromotionCodeDeactivatePayload{}); err == nil {
		t.Fatal("empty promotion deactivate validated")
	}
}

func TestStripePromotionReceiptBindingAndTerminalState(t *testing.T) {
	intent, err := NewIntent("coupon:1", KindStripeCouponDelete, "coupon:1", StripeCouponDeletePayload{
		ExternalKey: "local-1", CouponID: "coupon_1",
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := NewCompletedReceipt(intent, StripeCouponDeleteResult{ExternalKey: "local-1", CouponID: "coupon_1", Deleted: false}); err == nil {
		t.Fatal("non-deleted coupon result validated")
	}
	if _, err := NewCompletedReceipt(intent, StripeCouponDeleteResult{ExternalKey: "other", CouponID: "coupon_1", Deleted: true}); err == nil || !strings.Contains(err.Error(), "does not match") {
		t.Fatalf("mismatched external key error = %v", err)
	}

	deactivate, err := NewIntent("promotion:1", KindStripePromotionCodeDeactivate, "promotion:1", StripePromotionCodeDeactivatePayload{
		ExternalKey: "local-1", PromotionCodeID: "promo_1",
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := NewCompletedReceipt(deactivate, StripePromotionCodeDeactivateResult{
		ExternalKey: "local-1", PromotionCodeID: "promo_1", Active: true,
	}); err == nil {
		t.Fatal("active deactivate result validated")
	}
}

func TestStripePromotionKindAliases(t *testing.T) {
	aliases := map[string]string{
		"coupon.create":                    KindStripeCouponUpsert,
		"stripe.coupon.upsert.v1":          KindStripeCouponUpsert,
		"stripe.coupon.delete":             KindStripeCouponDelete,
		"promotion_code.create":            KindStripePromotionCodeUpsert,
		"stripe.promotion_code.upsert.v1":  KindStripePromotionCodeUpsert,
		"promotion_code.deactivate":        KindStripePromotionCodeDeactivate,
		"stripe.promotion_code.deactivate": KindStripePromotionCodeDeactivate,
	}
	for alias, want := range aliases {
		if got, err := NormalizeKind(alias); err != nil || got != want {
			t.Errorf("NormalizeKind(%q) = %q, %v; want %q", alias, got, err, want)
		}
	}
	if _, err := NormalizeKind("stripe.promotion_code.update"); err == nil {
		t.Fatal("ambiguous promotion-code update alias validated")
	}
}
