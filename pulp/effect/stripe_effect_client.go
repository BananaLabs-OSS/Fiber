package effect

import (
	"bytes"
	"fmt"

	"github.com/BananaLabs-OSS/Fiber/pulp"
	"github.com/vmihailenco/msgpack/v5"
)

// ExecuteStripeEffect submits one canonical, durable Stripe intent through
// the effect.stripe.runtime capability. The host performs one privileged
// provider operation only; it never owns application state, acknowledgement,
// Checkout, or free-invoice sequencing.
//
// A cell must explicitly declare the effect.stripe.runtime capability. The
// intent's idempotency_key is forwarded unchanged to Stripe and the returned
// receipt is validated against the exact intent before it is exposed.
func ExecuteStripeEffect(intent Intent) (Receipt, error) {
	if err := intent.Validate(); err != nil {
		return Receipt{}, fmt.Errorf("effect.stripe.runtime: invalid intent: %w", err)
	}
	if !isStripeHostEffectKind(intent.Kind) {
		return Receipt{}, fmt.Errorf("effect.stripe.runtime: unsupported intent kind %q", intent.Kind)
	}
	request, err := MarshalIntent(intent)
	if err != nil {
		return Receipt{}, fmt.Errorf("effect.stripe.runtime: marshal intent: %w", err)
	}
	wire, code := stripeEffectExecuteWire(request)
	if err := stripeEffectCodeError(code); err != nil {
		return Receipt{}, err
	}
	if len(wire) == 0 {
		return Receipt{}, fmt.Errorf("effect.stripe.runtime: empty receipt response")
	}

	// The host is not permitted to introduce aliases or envelope riders. Decode
	// strictly and validate the receipt's stable identity against this intent.
	var receipt Receipt
	decoder := msgpack.NewDecoder(bytes.NewReader(wire))
	decoder.DisallowUnknownFields(true)
	if err := decoder.Decode(&receipt); err != nil {
		return Receipt{}, fmt.Errorf("effect.stripe.runtime: decode receipt: %w", err)
	}
	if err := receipt.ValidateFor(intent); err != nil {
		return Receipt{}, fmt.Errorf("effect.stripe.runtime: invalid receipt: %w", err)
	}
	if receipt.Status != Completed {
		return Receipt{}, fmt.Errorf("effect.stripe.runtime: synchronous receipt must be completed")
	}
	return receipt, nil
}

func isStripeHostEffectKind(kind string) bool {
	switch kind {
	case KindStripePaymentIntentCreate,
		KindStripePaymentIntentGet,
		KindStripePaymentIntentCapture,
		KindStripePaymentIntentCancel,
		KindStripeSetupIntentCreate,
		KindStripeSetupIntentGet,
		KindStripeRefundCreate,
		KindStripeCustomerCreate,
		KindStripeInvoiceItemCreate,
		KindStripeInvoiceCreate,
		KindStripeInvoiceFinalize,
		KindStripeInvoiceMarkPaid,
		KindStripeCouponUpsert,
		KindStripeCouponDelete,
		KindStripePromotionCodeUpsert,
		KindStripePromotionCodeDeactivate:
		return true
	default:
		return false
	}
}

func stripeEffectCodeError(code uint32) error {
	switch code {
	case 0:
		return nil
	case 1:
		return fmt.Errorf("effect.stripe.runtime: empty request")
	case 2:
		return fmt.Errorf("effect.stripe.runtime: request memory read failed")
	case 3:
		return fmt.Errorf("effect.stripe.runtime: invalid or unauthorized intent")
	case 4:
		return fmt.Errorf("effect.stripe.runtime: Stripe execution failed")
	case 5, 6, 7, 8:
		return fmt.Errorf("effect.stripe.runtime: response allocation or write failed")
	case 99:
		return fmt.Errorf("effect.stripe.runtime: %w", pulp.ErrCapabilityUnavailable)
	default:
		return fmt.Errorf("effect.stripe.runtime: host code %d", code)
	}
}
