package workflow

import (
	"errors"
	"strings"
	"testing"

	"github.com/vmihailenco/msgpack/v5"
)

func TestClientDispatch(t *testing.T) {
	var calledTarget, calledFunction string
	client := &Client{
		Name: "lua-orchestrator",
		call: func(target, function string, payload []byte) ([]byte, error) {
			calledTarget, calledFunction = target, function
			var request DispatchRequest
			if err := msgpack.Unmarshal(payload, &request); err != nil {
				t.Fatalf("decode request: %v", err)
			}
			if request.Event != "evolution.gene.route.requested.v1" {
				t.Fatalf("event = %q", request.Event)
			}
			return msgpack.Marshal(DispatchResult{
				Value:  map[string]any{"status": int64(200)},
				Events: []Action{{Name: "evolution.route.completed.v1"}},
			})
		},
	}

	result, err := client.Dispatch(DispatchRequest{
		Event:   "evolution.gene.route.requested.v1",
		Payload: map[string]any{"gene": "sessions"},
	})
	if err != nil {
		t.Fatalf("Dispatch: %v", err)
	}
	if calledTarget != "lua-orchestrator" || calledFunction != FnDispatch {
		t.Fatalf("call = (%q, %q)", calledTarget, calledFunction)
	}
	if len(result.Events) != 1 || result.Events[0].Name != "evolution.route.completed.v1" {
		t.Fatalf("result = %#v", result)
	}
}

func TestClientRejectsInvalidInputAndOutput(t *testing.T) {
	calls := 0
	client := &Client{
		Name: "lua-orchestrator",
		call: func(_, _ string, _ []byte) ([]byte, error) {
			calls++
			return msgpack.Marshal(DispatchResult{
				Commands: []Action{{Name: ""}},
			})
		},
	}

	if _, err := client.Dispatch(DispatchRequest{}); err == nil {
		t.Fatal("Dispatch accepted an empty event")
	}
	if calls != 0 {
		t.Fatalf("invalid request reached caller %d time(s)", calls)
	}
	if _, err := client.Dispatch(DispatchRequest{Event: "valid"}); err == nil ||
		!strings.Contains(err.Error(), "command 0") {
		t.Fatalf("malformed result error = %v", err)
	}
}

func TestClientSurfacesCallAndDecodeFailures(t *testing.T) {
	client := &Client{
		Name: "lua-orchestrator",
		call: func(_, _ string, _ []byte) ([]byte, error) {
			return nil, errors.New("engine unavailable")
		},
	}
	if _, err := client.Dispatch(DispatchRequest{Event: "health"}); err == nil ||
		!strings.Contains(err.Error(), "engine unavailable") {
		t.Fatalf("call error = %v", err)
	}

	client.call = func(_, _ string, _ []byte) ([]byte, error) {
		return []byte{0xc1}, nil
	}
	if _, err := client.Dispatch(DispatchRequest{Event: "health"}); err == nil ||
		!strings.Contains(err.Error(), "decode dispatch") {
		t.Fatalf("decode error = %v", err)
	}

	client.call = func(_, _ string, _ []byte) ([]byte, error) {
		return nil, nil
	}
	if _, err := client.Dispatch(DispatchRequest{Event: "health"}); err == nil ||
		!strings.Contains(err.Error(), "empty response") {
		t.Fatalf("empty response error = %v", err)
	}
}

func TestClientExecuteSaga(t *testing.T) {
	request, err := NewSagaRequest("sessions.checkout.begin.v1", "request-1", "checkout:order-1", map[string]string{"order_id": "order-1"})
	if err != nil {
		t.Fatalf("NewSagaRequest: %v", err)
	}
	effectPayload, err := msgpack.Marshal(map[string]string{"checkout_id": "checkout-1"})
	if err != nil {
		t.Fatalf("marshal effect payload: %v", err)
	}
	effectResult, err := msgpack.Marshal(map[string]string{"client_secret": "pi_secret_123"})
	if err != nil {
		t.Fatalf("marshal effect result: %v", err)
	}
	result, err := NewCompletedSagaResult(request, map[string]string{"client_secret": "pi_secret_123"}, []EffectIntent{{
		ID: "effect-1", Kind: "stripe.payment_intent.create", IdempotencyKey: "checkout:order-1", Payload: effectPayload,
		Acknowledgement: EffectAcknowledgement{Status: EffectCompleted, Result: effectResult},
	}})
	if err != nil {
		t.Fatalf("NewCompletedSagaResult: %v", err)
	}

	var calledFunction string
	client := &Client{
		Name: "lua-orchestrator",
		call: func(_, function string, payload []byte) ([]byte, error) {
			calledFunction = function
			var received SagaRequest
			if err := msgpack.Unmarshal(payload, &received); err != nil {
				t.Fatalf("decode request: %v", err)
			}
			if received.RequestID != request.RequestID {
				t.Fatalf("request = %#v", received)
			}
			return msgpack.Marshal(result)
		},
	}
	got, err := client.ExecuteSaga(request)
	if err != nil {
		t.Fatalf("ExecuteSaga: %v", err)
	}
	if calledFunction != FnExecuteSaga || got.Status != SagaCompleted {
		t.Fatalf("call=%q result=%#v", calledFunction, got)
	}

	result.RequestID = "wrong-request"
	client.call = func(_, _ string, _ []byte) ([]byte, error) { return msgpack.Marshal(result) }
	if _, err := client.ExecuteSaga(request); err == nil || !strings.Contains(err.Error(), "identity") {
		t.Fatalf("mismatched saga result error = %v", err)
	}
}
