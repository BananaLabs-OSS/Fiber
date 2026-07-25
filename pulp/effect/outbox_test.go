package effect

import (
	"strings"
	"testing"
)

func testLease(t *testing.T) Lease {
	t.Helper()
	intent, err := NewIntent("effect-1", KindFleetServerProvision, "provision:order-1", map[string]string{"order_id": "order-1"})
	if err != nil {
		t.Fatal(err)
	}
	return Lease{
		Version: OutboxVersionV1, Owner: "sessions-commerce", ConsumerID: "dispatcher-a",
		LeaseID: "lease-1", Attempt: 1, LeasedUntilUnixMilli: 1000, Intent: intent,
	}
}

func TestOutboxClaimLeaseContract(t *testing.T) {
	request, err := NewClaimRequest("sessions-commerce", "dispatcher-a", 25, 30_000)
	if err != nil {
		t.Fatalf("NewClaimRequest: %v", err)
	}
	if request.Version != OutboxVersionV1 {
		t.Fatalf("version = %q", request.Version)
	}
	lease := testLease(t)
	result := ClaimResult{
		Version: OutboxVersionV1, Owner: request.Owner, ConsumerID: request.ConsumerID,
		Leases: []Lease{lease},
	}
	if err := result.Validate(); err != nil {
		t.Fatalf("claim result: %v", err)
	}
	result.Leases = append(result.Leases, lease)
	if err := result.Validate(); err == nil || !strings.Contains(err.Error(), "duplicated") {
		t.Fatalf("duplicate lease error = %v", err)
	}
}

func TestOutboxAcknowledgeAndRetryFencing(t *testing.T) {
	lease := testLease(t)
	receipt, err := NewCompletedReceipt(lease.Intent, map[string]bool{"accepted": true})
	if err != nil {
		t.Fatal(err)
	}
	ack := AcknowledgeRequest{
		Version: OutboxVersionV1, Owner: lease.Owner, ConsumerID: lease.ConsumerID,
		LeaseID: lease.LeaseID, Receipt: receipt,
	}
	if err := ack.ValidateFor(lease); err != nil {
		t.Fatalf("acknowledge: %v", err)
	}
	ack.LeaseID = "stale-lease"
	if err := ack.ValidateFor(lease); err == nil {
		t.Fatal("stale acknowledgement lease validated")
	}

	retry := RetryRequest{
		Version: OutboxVersionV1, Owner: lease.Owner, ConsumerID: lease.ConsumerID,
		LeaseID: lease.LeaseID, Failure: Failure{Code: "provider_unavailable", Message: "provider unavailable"},
		RetryAtUnixMilli: 2000,
	}
	if err := retry.ValidateFor(lease); err != nil {
		t.Fatalf("retry: %v", err)
	}
	retry.ConsumerID = "dispatcher-b"
	if err := retry.ValidateFor(lease); err == nil {
		t.Fatal("cross-consumer retry validated")
	}

	settlement := SettlementResult{
		Version: OutboxVersionV1, Owner: lease.Owner, ConsumerID: lease.ConsumerID,
		LeaseID: lease.LeaseID, Settled: false,
	}
	if err := settlement.ValidateFor(lease); err != nil {
		t.Fatalf("lost-lease settlement: %v", err)
	}
	settlement.LeaseID = "other-lease"
	if err := settlement.ValidateFor(lease); err == nil {
		t.Fatal("cross-lease settlement validated")
	}
}

func TestOutboxValidationRejectsUnsafeBounds(t *testing.T) {
	for _, request := range []ClaimRequest{
		{},
		{Version: OutboxVersionV1, Owner: "owner", ConsumerID: "consumer", Limit: 0, LeaseDurationMillis: 1},
		{Version: OutboxVersionV1, Owner: "owner", ConsumerID: "consumer", Limit: maxClaimLimit + 1, LeaseDurationMillis: 1},
		{Version: OutboxVersionV1, Owner: "owner", ConsumerID: "consumer", Limit: 1, LeaseDurationMillis: maxLeaseDurationMillis + 1},
	} {
		if err := request.Validate(); err == nil {
			t.Fatalf("invalid request %#v validated", request)
		}
	}

	lease := testLease(t)
	pending, err := NewPendingReceipt(lease.Intent)
	if err != nil {
		t.Fatal(err)
	}
	ack := AcknowledgeRequest{
		Version: OutboxVersionV1, Owner: lease.Owner, ConsumerID: lease.ConsumerID,
		LeaseID: lease.LeaseID, Receipt: pending,
	}
	if err := ack.Validate(); err == nil {
		t.Fatal("pending receipt acknowledgement validated")
	}
}
