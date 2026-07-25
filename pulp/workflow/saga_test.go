package workflow

import (
	"strings"
	"testing"

	"github.com/vmihailenco/msgpack/v5"
)

type checkoutStart struct {
	OrderID string `msgpack:"order_id"`
	Email   string `msgpack:"email"`
}

type checkoutReady struct {
	CheckoutID   string `msgpack:"checkout_id"`
	ClientSecret string `msgpack:"client_secret"`
}

type paymentIntentCreated struct {
	PaymentIntentID string `msgpack:"payment_intent_id"`
	ClientSecret    string `msgpack:"client_secret"`
}

func TestSagaWireRoundTripCompletedCheckout(t *testing.T) {
	request, err := NewSagaRequest("sessions.checkout.begin.v1", "request-1", "checkout:order-1", checkoutStart{
		OrderID: "order-1", Email: "owner@example.test",
	})
	if err != nil {
		t.Fatalf("NewSagaRequest: %v", err)
	}
	input, err := DecodePayload[checkoutStart](request)
	if err != nil {
		t.Fatalf("DecodePayload: %v", err)
	}
	if input.OrderID != "order-1" || input.Email != "owner@example.test" {
		t.Fatalf("decoded request = %#v", input)
	}

	effectResult, err := msgpack.Marshal(paymentIntentCreated{
		PaymentIntentID: "pi_123", ClientSecret: "pi_secret_123",
	})
	if err != nil {
		t.Fatalf("marshal effect result: %v", err)
	}
	effectPayload, err := msgpack.Marshal(map[string]string{"checkout_id": "checkout-1"})
	if err != nil {
		t.Fatalf("marshal effect payload: %v", err)
	}
	result, err := NewCompletedSagaResult(request, checkoutReady{
		CheckoutID: "checkout-1", ClientSecret: "pi_secret_123",
	}, []EffectIntent{{
		ID: "effect-1", Kind: "stripe.payment_intent.create", IdempotencyKey: "checkout:order-1",
		Payload:         effectPayload,
		Acknowledgement: EffectAcknowledgement{Status: EffectCompleted, Result: effectResult},
	}})
	if err != nil {
		t.Fatalf("NewCompletedSagaResult: %v", err)
	}

	encoded, err := msgpack.Marshal(result)
	if err != nil {
		t.Fatalf("marshal result: %v", err)
	}
	var decoded SagaResult
	if err := msgpack.Unmarshal(encoded, &decoded); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}
	if err := decoded.Validate(); err != nil {
		t.Fatalf("Validate: %v", err)
	}
	ready, err := DecodeResult[checkoutReady](decoded)
	if err != nil {
		t.Fatalf("DecodeResult: %v", err)
	}
	if ready.ClientSecret != "pi_secret_123" || ready.CheckoutID != "checkout-1" {
		t.Fatalf("decoded result = %#v", ready)
	}
	var acknowledgement paymentIntentCreated
	if err := msgpack.Unmarshal(decoded.Effects[0].Acknowledgement.Result, &acknowledgement); err != nil {
		t.Fatalf("decode effect acknowledgement: %v", err)
	}
	if acknowledgement.PaymentIntentID != "pi_123" {
		t.Fatalf("effect acknowledgement = %#v", acknowledgement)
	}
}

func TestSagaPendingAndFailedSemantics(t *testing.T) {
	request, err := NewSagaRequest("sessions.checkout.begin.v1", "request-1", "checkout:order-1", checkoutStart{OrderID: "order-1", Email: "owner@example.test"})
	if err != nil {
		t.Fatalf("NewSagaRequest: %v", err)
	}
	payload, err := msgpack.Marshal(map[string]string{"checkout_id": "checkout-1"})
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	pending, err := NewPendingSagaResult(request, []EffectIntent{{
		ID: "effect-1", Kind: "stripe.payment_intent.create", IdempotencyKey: "checkout:order-1", Payload: payload,
		Acknowledgement: EffectAcknowledgement{Status: EffectPending},
	}})
	if err != nil {
		t.Fatalf("NewPendingSagaResult: %v", err)
	}
	if pending.Status != SagaPending || len(pending.Result) != 0 {
		t.Fatalf("pending result = %#v", pending)
	}
	if _, err := DecodeResult[checkoutReady](pending); err == nil {
		t.Fatal("DecodeResult accepted a pending saga")
	}

	failed, err := NewFailedSagaResult(request, SagaError{Code: "payment_unavailable", Message: "payment provider unavailable"}, []EffectIntent{{
		ID: "effect-1", Kind: "stripe.payment_intent.create", IdempotencyKey: "checkout:order-1", Payload: payload,
		Acknowledgement: EffectAcknowledgement{Status: EffectFailed, Error: &SagaError{Code: "provider_error", Message: "declined"}},
	}})
	if err != nil {
		t.Fatalf("NewFailedSagaResult: %v", err)
	}
	if failed.Status != SagaFailed || failed.Error == nil || failed.Error.Code != "payment_unavailable" {
		t.Fatalf("failed result = %#v", failed)
	}
}

func TestSagaValidationRejectsInconsistentStates(t *testing.T) {
	payload, err := msgpack.Marshal(map[string]string{"order_id": "order-1"})
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	request := SagaRequest{Version: SagaVersionV1, Name: "sessions.checkout.begin.v1", RequestID: "request-1", IdempotencyKey: "checkout:order-1", Payload: payload}
	if err := request.Validate(); err != nil {
		t.Fatalf("valid request rejected: %v", err)
	}

	validEffect := EffectIntent{ID: "effect-1", Kind: "stripe.payment_intent.create", IdempotencyKey: "checkout:order-1", Payload: payload, Acknowledgement: EffectAcknowledgement{Status: EffectPending}}
	cases := []SagaResult{
		{Version: SagaVersionV1, Name: request.Name, RequestID: request.RequestID, IdempotencyKey: request.IdempotencyKey, Status: SagaPending, Result: payload},
		{Version: SagaVersionV1, Name: request.Name, RequestID: request.RequestID, IdempotencyKey: request.IdempotencyKey, Status: SagaCompleted, Result: payload, Effects: []EffectIntent{validEffect}},
		{Version: SagaVersionV1, Name: request.Name, RequestID: request.RequestID, IdempotencyKey: request.IdempotencyKey, Status: SagaFailed},
		{Version: SagaVersionV1, Name: request.Name, RequestID: request.RequestID, IdempotencyKey: request.IdempotencyKey, Status: SagaCompleted, Result: []byte{0xc1}},
		{Version: SagaVersionV1, Name: request.Name, RequestID: request.RequestID, IdempotencyKey: request.IdempotencyKey, Status: SagaPending, Effects: []EffectIntent{validEffect, validEffect}},
		{Version: SagaVersionV1, Name: request.Name, RequestID: request.RequestID, IdempotencyKey: request.IdempotencyKey, Status: SagaPending, Effects: []EffectIntent{validEffect, {ID: "effect-2", Kind: validEffect.Kind, IdempotencyKey: validEffect.IdempotencyKey, Payload: validEffect.Payload, Acknowledgement: validEffect.Acknowledgement}}},
	}
	for _, result := range cases {
		if err := result.Validate(); err == nil {
			t.Fatalf("invalid saga result accepted: %#v", result)
		}
	}

	for _, invalid := range []SagaRequest{
		{Version: "pulp.workflow.saga.v2", Name: request.Name, RequestID: request.RequestID, IdempotencyKey: request.IdempotencyKey, Payload: payload},
		{Version: SagaVersionV1, Name: request.Name, RequestID: request.RequestID, IdempotencyKey: request.IdempotencyKey},
		{Version: SagaVersionV1, Name: request.Name, RequestID: request.RequestID, IdempotencyKey: request.IdempotencyKey, Payload: []byte{0xc1}},
	} {
		if err := invalid.Validate(); err == nil {
			t.Fatalf("invalid saga request accepted: %#v", invalid)
		}
	}

	if err := (EffectAcknowledgement{Status: EffectCompleted, Error: &SagaError{Code: "unexpected", Message: "must not be present"}}).Validate(); err == nil || !strings.Contains(err.Error(), "must not") {
		t.Fatalf("completed acknowledgement validation = %v", err)
	}
}
