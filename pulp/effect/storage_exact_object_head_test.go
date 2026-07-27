package effect

import (
	"testing"

	"github.com/vmihailenco/msgpack/v5"
)

func TestStorageExactObjectHeadContract(t *testing.T) {
	payload := StorageExactObjectHeadPayload{
		ExactKey:    "sessions/app-1/inventory/object-1",
		AllowAbsent: true,
	}
	intent, err := NewIntent("storage-head:1", KindStorageExactObjectHead, "storage-head:object-1", payload)
	if err != nil {
		t.Fatalf("NewIntent: %v", err)
	}
	if intent.Kind != KindStorageExactObjectHead {
		t.Fatalf("kind = %q", intent.Kind)
	}
	decodedPayload, err := DecodePayload[StorageExactObjectHeadPayload](intent)
	if err != nil || decodedPayload != payload {
		t.Fatalf("DecodePayload = %#v, %v", decodedPayload, err)
	}

	receipt, err := NewCompletedReceipt(intent, StorageExactObjectHeadResult{
		ExactKey: payload.ExactKey, Generation: 42,
	})
	if err != nil {
		t.Fatalf("NewCompletedReceipt: %v", err)
	}
	if err := receipt.ValidateFor(intent); err != nil {
		t.Fatalf("ValidateFor: %v", err)
	}
	decodedResult, err := DecodeResult[StorageExactObjectHeadResult](receipt)
	if err != nil || decodedResult.Generation != 42 || decodedResult.Absent {
		t.Fatalf("DecodeResult = %#v, %v", decodedResult, err)
	}

	wire, err := MarshalIntent(intent)
	if err != nil {
		t.Fatalf("MarshalIntent: %v", err)
	}
	decodedIntent, err := UnmarshalIntent(wire)
	if err != nil || decodedIntent.Kind != KindStorageExactObjectHead {
		t.Fatalf("UnmarshalIntent = %#v, %v", decodedIntent, err)
	}
}

func TestStorageExactObjectHeadValidation(t *testing.T) {
	for _, key := range []string{"", " /inventory/object", "/inventory/object", "inventory//object", "inventory/../object", "inventory\\object"} {
		if _, err := NewIntent("storage-head:invalid", KindStorageExactObjectHead, "storage-head:invalid", StorageExactObjectHeadPayload{ExactKey: key}); err == nil {
			t.Fatalf("unsafe key %q validated", key)
		}
	}
	if got, err := NormalizeKind(KindStorageExactObjectHead); err != nil || got != KindStorageExactObjectHead {
		t.Fatalf("NormalizeKind = %q, %v", got, err)
	}
	unknownPayload, err := msgpack.Marshal(map[string]any{
		"exact_key": "sessions/app-1/object", "allow_absent": true, "prefix": "sessions/app-1/",
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := (Intent{Version: VersionV1, ID: "storage-head:unknown", Kind: KindStorageExactObjectHead, IdempotencyKey: "storage-head:unknown", Payload: unknownPayload}).Validate(); err == nil {
		t.Fatal("payload with fields outside the exact-object-head contract validated")
	}
	unknownResult, err := msgpack.Marshal(map[string]any{
		"exact_key": "sessions/app-1/object", "generation": int64(1), "absent": false, "etag": "opaque",
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := (Receipt{Version: VersionV1, IntentID: "storage-head:unknown", Kind: KindStorageExactObjectHead, IdempotencyKey: "storage-head:unknown", Status: Completed, Result: unknownResult}).Validate(); err == nil {
		t.Fatal("result with fields outside the exact-object-head contract validated")
	}

	intent, err := NewIntent("storage-head:1", KindStorageExactObjectHead, "storage-head:1", StorageExactObjectHeadPayload{ExactKey: "sessions/app-1/object", AllowAbsent: false})
	if err != nil {
		t.Fatal(err)
	}
	for _, result := range []StorageExactObjectHeadResult{
		{ExactKey: intent.ID, Generation: 0},
		{ExactKey: "sessions/app-1/object", Generation: 1, Absent: true},
	} {
		if _, err := NewCompletedReceipt(intent, result); err == nil {
			t.Fatalf("invalid result %#v validated", result)
		}
	}

	absentRaw, err := msgpack.Marshal(StorageExactObjectHeadResult{ExactKey: "sessions/app-1/object", Absent: true})
	if err != nil {
		t.Fatalf("marshal absent result: %v", err)
	}
	absent := receiptFor(intent, Completed)
	absent.Result = absentRaw
	if err := absent.ValidateFor(intent); err == nil {
		t.Fatal("absent result validated when allow_absent is false")
	}

	mismatchRaw, err := msgpack.Marshal(StorageExactObjectHeadResult{ExactKey: "sessions/app-1/other", Generation: 1})
	if err != nil {
		t.Fatal(err)
	}
	mismatch := receiptFor(intent, Completed)
	mismatch.Result = mismatchRaw
	if err := mismatch.ValidateFor(intent); err == nil {
		t.Fatal("result with a mismatched exact key validated")
	}
}
