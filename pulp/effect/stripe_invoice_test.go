package effect

import (
	"strings"
	"testing"
)

func TestStripeInvoiceUnitContracts(t *testing.T) {
	tests := []struct {
		kind    string
		payload any
		result  any
	}{
		{
			kind: KindStripeInvoiceItemCreate,
			payload: StripeInvoiceItemCreatePayload{
				CustomerID: "cus_123", InvoiceID: "in_123", AmountCents: 1200, Currency: "usd", Description: "line item",
			},
			result: StripeInvoiceItemCreateResult{InvoiceItemID: "ii_123"},
		},
		{
			kind: KindStripeInvoiceCreate,
			payload: StripeInvoiceCreatePayload{
				CustomerID: "cus_123", AutoAdvance: true, CollectionMethod: "charge_automatically", PromotionCodeID: "promo_123",
			},
			result: StripeInvoiceResult{InvoiceID: "in_123", Status: "draft", AmountDue: 1200},
		},
		{
			kind:    KindStripeInvoiceFinalize,
			payload: StripeInvoiceFinalizePayload{InvoiceID: "in_123"},
			result:  StripeInvoiceResult{InvoiceID: "in_123", Status: "open", AmountDue: 1200},
		},
		{
			kind:    KindStripeInvoiceMarkPaid,
			payload: StripeInvoiceMarkPaidPayload{InvoiceID: "in_123"},
			result:  StripeInvoiceResult{InvoiceID: "in_123", Status: "paid", AmountPaid: 1200},
		},
	}
	for _, tt := range tests {
		t.Run(tt.kind, func(t *testing.T) {
			intent, err := NewIntent("invoice:"+tt.kind, tt.kind, "provider:"+tt.kind, tt.payload)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := NewCompletedReceipt(intent, tt.result); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestStripeInvoiceUnitContractsRejectInvalidPayloadsAndMismatchedReceipts(t *testing.T) {
	for _, tt := range []struct {
		kind    string
		payload any
	}{
		{KindStripeInvoiceItemCreate, StripeInvoiceItemCreatePayload{CustomerID: "cus_123", AmountCents: 0, Currency: "usd"}},
		{KindStripeInvoiceItemCreate, StripeInvoiceItemCreatePayload{CustomerID: "cus_123", AmountCents: 1, Currency: "USD"}},
		{KindStripeInvoiceCreate, StripeInvoiceCreatePayload{CustomerID: "cus_123", CollectionMethod: "manual"}},
		{KindStripeInvoiceFinalize, StripeInvoiceFinalizePayload{}},
		{KindStripeInvoiceMarkPaid, StripeInvoiceMarkPaidPayload{}},
	} {
		if _, err := NewIntent("invalid:"+tt.kind, tt.kind, "invalid:"+tt.kind, tt.payload); err == nil {
			t.Fatalf("invalid %s payload accepted: %#v", tt.kind, tt.payload)
		}
	}

	for _, tt := range []struct {
		kind    string
		payload any
	}{
		{KindStripeInvoiceFinalize, StripeInvoiceFinalizePayload{InvoiceID: "in_123"}},
		{KindStripeInvoiceMarkPaid, StripeInvoiceMarkPaidPayload{InvoiceID: "in_123"}},
	} {
		intent, err := NewIntent("mismatch:"+tt.kind, tt.kind, "mismatch:"+tt.kind, tt.payload)
		if err != nil {
			t.Fatal(err)
		}
		_, err = NewCompletedReceipt(intent, StripeInvoiceResult{InvoiceID: "in_other", Status: "paid"})
		if err == nil || !strings.Contains(err.Error(), "does not match") {
			t.Fatalf("mismatched %s receipt error = %v", tt.kind, err)
		}
	}
}

func TestStripeInvoiceUnitKindAliases(t *testing.T) {
	aliases := map[string]string{
		"invoice_item.create":        KindStripeInvoiceItemCreate,
		"stripe.invoice-item.create": KindStripeInvoiceItemCreate,
		"invoice.create":             KindStripeInvoiceCreate,
		"invoice.finalize":           KindStripeInvoiceFinalize,
		"invoice.mark_paid":          KindStripeInvoiceMarkPaid,
		"stripe.invoice.mark-paid":   KindStripeInvoiceMarkPaid,
	}
	for alias, want := range aliases {
		if got, err := NormalizeKind(alias); err != nil || got != want {
			t.Errorf("NormalizeKind(%q) = %q, %v; want %q", alias, got, err, want)
		}
	}
}
