package effect

import (
	"encoding/hex"
	"reflect"
	"strings"
	"testing"

	"github.com/vmihailenco/msgpack/v5"
)

func TestStripeCustomerCreateContract(t *testing.T) {
	payload := StripeCustomerCreatePayload{
		Email: "owner@example.com", Name: "Owner",
		Description: "Sessions customer",
	}
	intent, err := NewIntent("customer:order-1", "stripe.customer.create", "checkout:order-1:customer", payload)
	if err != nil {
		t.Fatalf("NewIntent: %v", err)
	}
	if intent.Kind != KindStripeCustomerCreate {
		t.Fatalf("kind = %q", intent.Kind)
	}
	decoded, err := DecodePayload[StripeCustomerCreatePayload](intent)
	if err != nil || !reflect.DeepEqual(decoded, payload) {
		t.Fatalf("payload = %#v, %v", decoded, err)
	}
	receipt, err := NewCompletedReceipt(intent, StripeCustomerCreateResult{
		CustomerID: "cus_1", Email: payload.Email,
	})
	if err != nil {
		t.Fatalf("NewCompletedReceipt: %v", err)
	}
	result, err := DecodeResult[StripeCustomerCreateResult](receipt)
	if err != nil || result.CustomerID != "cus_1" {
		t.Fatalf("result = %#v, %v", result, err)
	}
}

func TestStripeFreeInvoiceFinalizeContract(t *testing.T) {
	payload := StripeFreeInvoiceFinalizePayload{
		Customer: StripeCustomerCreatePayload{Email: "owner@example.com"},
		InvoiceItem: StripeFreeInvoiceItem{
			AmountCents: 1200, Currency: "usd", Description: "Sessions server",
		},
		Invoice: StripeFreeInvoice{
			CollectionMethod: "charge_automatically", PromotionCodeID: "promo_1",
		},
	}
	intent, err := NewIntent("invoice:order-1", "stripe.free_invoice.finalize", "checkout:order-1:free-invoice", payload)
	if err != nil {
		t.Fatalf("NewIntent: %v", err)
	}
	if intent.Kind != KindStripeFreeInvoiceFinalize {
		t.Fatalf("kind = %q", intent.Kind)
	}
	receipt, err := NewCompletedReceipt(intent, StripeFreeInvoiceFinalizeResult{
		CustomerID: "cus_1", InvoiceItemID: "ii_1", InvoiceID: "in_1",
		Status: "paid", AmountDue: 0, AmountPaid: 0,
	})
	if err != nil {
		t.Fatalf("NewCompletedReceipt: %v", err)
	}
	result, err := DecodeResult[StripeFreeInvoiceFinalizeResult](receipt)
	if err != nil || result.InvoiceID != "in_1" || result.AmountDue != 0 {
		t.Fatalf("result = %#v, %v", result, err)
	}

	nonFree := result
	nonFree.AmountDue = 1
	if _, err := NewCompletedReceipt(intent, nonFree); err == nil || !strings.Contains(err.Error(), "amount_due") {
		t.Fatalf("non-free result error = %v", err)
	}
}

func TestStripeCheckoutTypedValidation(t *testing.T) {
	invalidCustomers := []StripeCustomerCreatePayload{
		{},
		{Email: "not-an-email"},
		{Email: " owner@example.com"},
	}
	for _, payload := range invalidCustomers {
		if _, err := NewIntent("customer:1", KindStripeCustomerCreate, "customer:1", payload); err == nil {
			t.Fatalf("invalid customer %#v accepted", payload)
		}
	}

	base := StripeFreeInvoiceFinalizePayload{
		Customer: StripeCustomerCreatePayload{Email: "owner@example.com"},
		InvoiceItem: StripeFreeInvoiceItem{
			AmountCents: 1200, Currency: "usd", Description: "Sessions server",
		},
	}
	for name, mutate := range map[string]func(*StripeFreeInvoiceFinalizePayload){
		"non-positive amount": func(p *StripeFreeInvoiceFinalizePayload) { p.InvoiceItem.AmountCents = 0 },
		"uppercase currency":  func(p *StripeFreeInvoiceFinalizePayload) { p.InvoiceItem.Currency = "USD" },
		"missing description": func(p *StripeFreeInvoiceFinalizePayload) { p.InvoiceItem.Description = "" },
		"manual collection":   func(p *StripeFreeInvoiceFinalizePayload) { p.Invoice.CollectionMethod = "send_invoice" },
	} {
		t.Run(name, func(t *testing.T) {
			payload := base
			mutate(&payload)
			if _, err := NewIntent("invoice:1", KindStripeFreeInvoiceFinalize, "invoice:1", payload); err == nil {
				t.Fatalf("invalid payload %#v accepted", payload)
			}
		})
	}
}

func TestStripeCheckoutTypedGoldenWire(t *testing.T) {
	payload := StripeFreeInvoiceFinalizePayload{
		Customer:    StripeCustomerCreatePayload{Email: "a@b.co"},
		InvoiceItem: StripeFreeInvoiceItem{AmountCents: 10, Currency: "usd", Description: "x"},
		Invoice:     StripeFreeInvoice{PromotionCodeID: "promo_1"},
	}
	wire, err := msgpack.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	const wantPayloadHex = "83a8637573746f6d657281a5656d61696ca66140622e636fac696e766f6963655f6974656d83ac616d6f756e745f63656e7473d3000000000000000aa863757272656e6379a3757364ab6465736372697074696f6ea178a7696e766f69636581b170726f6d6f74696f6e5f636f64655f6964a770726f6d6f5f31"
	if got := hex.EncodeToString(wire); got != wantPayloadHex {
		t.Fatalf("payload wire = %s, want %s", got, wantPayloadHex)
	}
	resultWire, err := msgpack.Marshal(StripeFreeInvoiceFinalizeResult{
		CustomerID: "cus_1", InvoiceItemID: "ii_1", InvoiceID: "in_1", Status: "paid",
	})
	if err != nil {
		t.Fatal(err)
	}
	const wantResultHex = "86ab637573746f6d65725f6964a56375735f31af696e766f6963655f6974656d5f6964a469695f31aa696e766f6963655f6964a4696e5f31a6737461747573a470616964aa616d6f756e745f647565d30000000000000000ab616d6f756e745f70616964d30000000000000000"
	if got := hex.EncodeToString(resultWire); got != wantResultHex {
		t.Fatalf("result wire = %s, want %s", got, wantResultHex)
	}
}

func TestStripeCheckoutKindAliases(t *testing.T) {
	aliases := map[string]string{
		"customer.create":                 KindStripeCustomerCreate,
		"stripe.customer.create":          KindStripeCustomerCreate,
		"stripe.customer.create.v1":       KindStripeCustomerCreate,
		"free_invoice.finalize":           KindStripeFreeInvoiceFinalize,
		"stripe.free_invoice.finalize":    KindStripeFreeInvoiceFinalize,
		"stripe.free_invoice.finalize.v1": KindStripeFreeInvoiceFinalize,
		"stripe.free-invoice.finalize":    KindStripeFreeInvoiceFinalize,
	}
	for alias, want := range aliases {
		if got, err := NormalizeKind(alias); err != nil || got != want {
			t.Errorf("NormalizeKind(%q) = %q, %v; want %q", alias, got, err, want)
		}
	}
}
