package effect

import (
	"reflect"
	"strings"
	"testing"

	"github.com/vmihailenco/msgpack/v5"
)

func TestFleetRuntimeObservationCanonicalContract(t *testing.T) {
	intent, err := NewIntent(
		"fleet-observe-1",
		KindFleetRuntimeObservationExecute,
		"fleet-observe:server-1:settings",
		fleetRuntimeObservationIntentForTest(FleetRuntimeObservationFieldSettingsV1),
	)
	if err != nil {
		t.Fatalf("NewIntent: %v", err)
	}
	result := fleetRuntimeObservationResultForTest(FleetRuntimeObservationFieldSettingsV1)
	result.Data.Settings = map[string]string{"difficulty": "hard"}
	receipt, err := NewCompletedReceipt(intent, result)
	if err != nil {
		t.Fatalf("NewCompletedReceipt: %v", err)
	}
	if err := receipt.ValidateFor(intent); err != nil {
		t.Fatalf("ValidateFor: %v", err)
	}
}

func TestFleetRuntimeObservationAllowsOnlyTypedFields(t *testing.T) {
	for _, field := range []FleetRuntimeObservationFieldV1{
		FleetRuntimeObservationFieldSettingsV1,
		FleetRuntimeObservationFieldGameRulesV1,
		FleetRuntimeObservationFieldPlayersV1,
		FleetRuntimeObservationFieldPlayerHistoryV1,
		FleetRuntimeObservationFieldAccessV1,
		FleetRuntimeObservationFieldAccessSnapshotV1,
		FleetRuntimeObservationFieldArtifactsV1,
		FleetRuntimeObservationFieldStatusV1,
	} {
		payload := fleetRuntimeObservationIntentForTest(field)
		intent, err := NewIntent("fleet-observe-"+string(field), KindFleetRuntimeObservationExecute, "fleet-observe-"+string(field), payload)
		if err != nil {
			t.Errorf("field %q: %v", field, err)
			continue
		}
		result := fleetRuntimeObservationResultForTest(field)
		switch field {
		case FleetRuntimeObservationFieldSettingsV1:
			result.Data.Settings = map[string]string{"difficulty": "hard"}
		case FleetRuntimeObservationFieldGameRulesV1:
			result.Data.GameRules = map[string]string{"keepInventory": "true"}
		case FleetRuntimeObservationFieldPlayersV1:
			result.Data.Players = []string{"alice"}
		case FleetRuntimeObservationFieldPlayerHistoryV1:
			result.Data.PlayerHistory = []FleetRuntimePlayerHistoryEntryV1{{
				UUID: "12345678-1234-1234-1234-123456789abc", Name: "alice",
				ExpiresOn: "2026-08-26 01:00:00 +0000",
			}}
		case FleetRuntimeObservationFieldAccessV1:
			result.Data.Access = &FleetRuntimeObservationAccessV1{ServerID: payload.ServerID, Whitelist: []string{"alice"}}
		case FleetRuntimeObservationFieldAccessSnapshotV1:
			result.Data.AccessSnapshot = fleetRuntimeObservationAccessSnapshotForTest(payload.ServerID)
		case FleetRuntimeObservationFieldArtifactsV1:
			result.Data.Artifacts = []FleetRuntimeObservedArtifactV1{{
				Name: "fabric-api-1.0.0.jar", Kind: "mod", HashSHA256: strings.Repeat("b", 64),
			}}
		case FleetRuntimeObservationFieldStatusV1:
			result.Data.Status = "healthy"
		}
		if _, err := NewCompletedReceipt(intent, result); err != nil {
			t.Errorf("result field %q: %v", field, err)
		}
	}
	for _, field := range []FleetRuntimeObservationFieldV1{"endpoint", "rcon", "files", "shell"} {
		payload := fleetRuntimeObservationIntentForTest(field)
		if _, err := NewIntent("fleet-observe-invalid", KindFleetRuntimeObservationExecute, "fleet-observe-invalid", payload); err == nil {
			t.Errorf("field %q validated", field)
		}
	}
}

func TestFleetRuntimeObservationPlayerHistoryIsDistinctBoundedEvidence(t *testing.T) {
	intent, err := NewIntent(
		"fleet-observe-player-history",
		KindFleetRuntimeObservationExecute,
		"fleet-observe-player-history",
		fleetRuntimeObservationIntentForTest(FleetRuntimeObservationFieldPlayerHistoryV1),
	)
	if err != nil {
		t.Fatal(err)
	}
	valid := fleetRuntimeObservationResultForTest(FleetRuntimeObservationFieldPlayerHistoryV1)
	valid.Data.PlayerHistory = []FleetRuntimePlayerHistoryEntryV1{{
		UUID: "12345678-1234-1234-1234-123456789abc", Name: "Player_One",
		ExpiresOn: "2026-08-26 01:00:00 +0000",
	}, {
		UUID: "abcdefab-cdef-abcd-efab-cdefabcdefab", Name: ".Bedrock Player",
		ExpiresOn: "2026-08-27 01:00:00 +0000",
	}}
	if _, err := NewCompletedReceipt(intent, valid); err != nil {
		t.Fatalf("valid player history: %v", err)
	}

	onlineOnly := fleetRuntimeObservationResultForTest(FleetRuntimeObservationFieldPlayerHistoryV1)
	onlineOnly.Data.Players = []string{"Player_One"}
	if _, err := NewCompletedReceipt(intent, onlineOnly); err == nil {
		t.Fatal("online players result substituted for player history")
	}

	tests := []FleetRuntimePlayerHistoryEntryV1{
		{UUID: "1234", Name: "Player_One", ExpiresOn: "2026-08-26 01:00:00 +0000"},
		{UUID: "12345678-1234-1234-1234-123456789ABC", Name: "Player_One", ExpiresOn: "2026-08-26 01:00:00 +0000"},
		{UUID: "12345678-1234-1234-1234-123456789abc", Name: "bad/name", ExpiresOn: "2026-08-26 01:00:00 +0000"},
		{UUID: "12345678-1234-1234-1234-123456789abc", Name: "Player_One", ExpiresOn: ""},
		{UUID: "12345678-1234-1234-1234-123456789abc", Name: "Player_One", ExpiresOn: "bad\nvalue"},
	}
	for _, entry := range tests {
		result := fleetRuntimeObservationResultForTest(FleetRuntimeObservationFieldPlayerHistoryV1)
		result.Data.PlayerHistory = []FleetRuntimePlayerHistoryEntryV1{entry}
		if _, err := NewCompletedReceipt(intent, result); err == nil {
			t.Errorf("invalid player history entry %#v validated", entry)
		}
	}

	duplicate := fleetRuntimeObservationResultForTest(FleetRuntimeObservationFieldPlayerHistoryV1)
	duplicate.Data.PlayerHistory = []FleetRuntimePlayerHistoryEntryV1{
		{UUID: "12345678-1234-1234-1234-123456789abc", Name: "Player_One", ExpiresOn: "2026-08-26 01:00:00 +0000"},
		{UUID: "12345678-1234-1234-1234-123456789abc", Name: "Player_Two", ExpiresOn: "2026-08-27 01:00:00 +0000"},
	}
	if _, err := NewCompletedReceipt(intent, duplicate); err == nil {
		t.Fatal("duplicate player history UUID validated")
	}

	oversize := fleetRuntimeObservationResultForTest(FleetRuntimeObservationFieldPlayerHistoryV1)
	oversize.Data.PlayerHistory = make([]FleetRuntimePlayerHistoryEntryV1, 1001)
	for index := range oversize.Data.PlayerHistory {
		oversize.Data.PlayerHistory[index] = FleetRuntimePlayerHistoryEntryV1{
			UUID: "12345678-1234-1234-1234-123456789abc", Name: "Player_One",
			ExpiresOn: "2026-08-26 01:00:00 +0000",
		}
	}
	if _, err := NewCompletedReceipt(intent, oversize); err == nil {
		t.Fatal("oversize player history validated")
	}
}

func TestFleetRuntimeObservationAccessSnapshotIsDistinctBoundedEvidence(t *testing.T) {
	payload := fleetRuntimeObservationIntentForTest(FleetRuntimeObservationFieldAccessSnapshotV1)
	intent, err := NewIntent(
		"fleet-observe-access-snapshot",
		KindFleetRuntimeObservationExecute,
		"fleet-observe-access-snapshot",
		payload,
	)
	if err != nil {
		t.Fatal(err)
	}
	valid := fleetRuntimeObservationResultForTest(FleetRuntimeObservationFieldAccessSnapshotV1)
	valid.Data.AccessSnapshot = fleetRuntimeObservationAccessSnapshotForTest(payload.ServerID)
	if _, err := NewCompletedReceipt(intent, valid); err != nil {
		t.Fatalf("valid access snapshot: %v", err)
	}

	nameOnly := fleetRuntimeObservationResultForTest(FleetRuntimeObservationFieldAccessSnapshotV1)
	nameOnly.Data.Access = &FleetRuntimeObservationAccessV1{
		ServerID: payload.ServerID, Whitelist: []string{"Player_One"},
		UpdatedAt: "2026-07-26T01:00:00Z",
	}
	if _, err := NewCompletedReceipt(intent, nameOnly); err == nil {
		t.Fatal("name-only access result substituted for access snapshot")
	}

	tests := []func(*FleetRuntimeObservationAccessSnapshotV1){
		func(value *FleetRuntimeObservationAccessSnapshotV1) { value.ServerID = "server-other" },
		func(value *FleetRuntimeObservationAccessSnapshotV1) { value.UpdatedAt = "not-a-time" },
		func(value *FleetRuntimeObservationAccessSnapshotV1) { value.Whitelist[0].UUID = "bad" },
		func(value *FleetRuntimeObservationAccessSnapshotV1) { value.Whitelist[0].Name = "bad/name" },
		func(value *FleetRuntimeObservationAccessSnapshotV1) { value.Operators[0].Level = 0 },
		func(value *FleetRuntimeObservationAccessSnapshotV1) { value.Operators[0].Level = 5 },
		func(value *FleetRuntimeObservationAccessSnapshotV1) { value.Bans[0].Created = "" },
		func(value *FleetRuntimeObservationAccessSnapshotV1) { value.Bans[0].Source = "bad\nsource" },
		func(value *FleetRuntimeObservationAccessSnapshotV1) { value.Bans[0].Expires = "" },
		func(value *FleetRuntimeObservationAccessSnapshotV1) { value.Bans[0].Reason = strings.Repeat("x", 513) },
	}
	for index, mutate := range tests {
		result := fleetRuntimeObservationResultForTest(FleetRuntimeObservationFieldAccessSnapshotV1)
		snapshot := fleetRuntimeObservationAccessSnapshotForTest(payload.ServerID)
		mutate(snapshot)
		result.Data.AccessSnapshot = snapshot
		if _, err := NewCompletedReceipt(intent, result); err == nil {
			t.Errorf("invalid access snapshot case %d validated", index)
		}
	}

	duplicates := []func(*FleetRuntimeObservationAccessSnapshotV1){
		func(value *FleetRuntimeObservationAccessSnapshotV1) {
			value.Whitelist = append(value.Whitelist, value.Whitelist[0])
		},
		func(value *FleetRuntimeObservationAccessSnapshotV1) {
			value.Operators = append(value.Operators, value.Operators[0])
		},
		func(value *FleetRuntimeObservationAccessSnapshotV1) {
			value.Bans = append(value.Bans, value.Bans[0])
		},
	}
	for index, mutate := range duplicates {
		result := fleetRuntimeObservationResultForTest(FleetRuntimeObservationFieldAccessSnapshotV1)
		snapshot := fleetRuntimeObservationAccessSnapshotForTest(payload.ServerID)
		mutate(snapshot)
		result.Data.AccessSnapshot = snapshot
		if _, err := NewCompletedReceipt(intent, result); err == nil {
			t.Errorf("duplicate access snapshot case %d validated", index)
		}
	}

	for _, group := range []string{"whitelist", "operators", "bans"} {
		result := fleetRuntimeObservationResultForTest(FleetRuntimeObservationFieldAccessSnapshotV1)
		snapshot := fleetRuntimeObservationAccessSnapshotForTest(payload.ServerID)
		switch group {
		case "whitelist":
			snapshot.Whitelist = make([]FleetRuntimeAccessSnapshotIdentityV1, 1001)
		case "operators":
			snapshot.Operators = make([]FleetRuntimeAccessSnapshotOperatorV1, 1001)
		case "bans":
			snapshot.Bans = make([]FleetRuntimeAccessSnapshotBanV1, 1001)
		}
		result.Data.AccessSnapshot = snapshot
		if _, err := NewCompletedReceipt(intent, result); err == nil {
			t.Errorf("oversize access snapshot %s validated", group)
		}
	}
}

func TestFleetRuntimeObservationDetailedFieldsSurviveCanonicalWire(t *testing.T) {
	tests := []struct {
		field FleetRuntimeObservationFieldV1
		data  FleetRuntimeObservationDataV1
	}{
		{
			field: FleetRuntimeObservationFieldPlayerHistoryV1,
			data: FleetRuntimeObservationDataV1{PlayerHistory: []FleetRuntimePlayerHistoryEntryV1{{
				UUID: "12345678-1234-1234-1234-123456789abc",
				Name: "Player_One", ExpiresOn: "2026-08-26 01:00:00 +0000",
			}}},
		},
		{
			field: FleetRuntimeObservationFieldAccessSnapshotV1,
			data: FleetRuntimeObservationDataV1{
				AccessSnapshot: fleetRuntimeObservationAccessSnapshotForTest("server-1"),
			},
		},
	}
	for _, test := range tests {
		intent, err := NewIntent(
			"fleet-observe-wire-"+string(test.field),
			KindFleetRuntimeObservationExecute,
			"fleet-observe-wire-"+string(test.field),
			fleetRuntimeObservationIntentForTest(test.field),
		)
		if err != nil {
			t.Fatal(err)
		}
		result := fleetRuntimeObservationResultForTest(test.field)
		result.Data = test.data
		receipt, err := NewCompletedReceipt(intent, result)
		if err != nil {
			t.Fatalf("%s completed receipt: %v", test.field, err)
		}
		wire, err := MarshalReceipt(receipt)
		if err != nil {
			t.Fatalf("%s marshal receipt: %v", test.field, err)
		}
		decodedReceipt, err := UnmarshalReceipt(wire)
		if err != nil {
			t.Fatalf("%s unmarshal receipt: %v", test.field, err)
		}
		decodedResult, err := DecodeResult[FleetRuntimeObservationReceiptV1](decodedReceipt)
		if err != nil {
			t.Fatalf("%s decode result: %v", test.field, err)
		}
		if !reflect.DeepEqual(decodedResult.Data, test.data) {
			t.Fatalf("%s wire data = %#v, want %#v", test.field, decodedResult.Data, test.data)
		}
	}
}

func TestFleetRuntimeObservationNewFieldsRejectUnknownNestedFields(t *testing.T) {
	tests := []struct {
		field FleetRuntimeObservationFieldV1
		data  map[string]any
	}{
		{
			field: FleetRuntimeObservationFieldPlayerHistoryV1,
			data: map[string]any{"player_history": []map[string]any{{
				"uuid": "12345678-1234-1234-1234-123456789abc", "name": "Player_One",
				"expires_on": "2026-08-26 01:00:00 +0000", "path": "/data/usercache.json",
			}}},
		},
		{
			field: FleetRuntimeObservationFieldAccessSnapshotV1,
			data: map[string]any{"access_snapshot": map[string]any{
				"server_id": "server-1", "updated_at": "2026-07-26T01:00:00Z",
				"whitelist": []map[string]any{{
					"uuid": "12345678-1234-1234-1234-123456789abc",
					"name": "Player_One", "approved": true,
				}},
			}},
		},
	}
	for _, test := range tests {
		payload := fleetRuntimeObservationIntentForTest(test.field)
		intent, err := NewIntent("fleet-observe-unknown-"+string(test.field), KindFleetRuntimeObservationExecute, "fleet-observe-unknown-"+string(test.field), payload)
		if err != nil {
			t.Fatal(err)
		}
		result := map[string]any{
			"contract": payload.Contract, "server_id": payload.ServerID, "node_id": payload.NodeID,
			"container_id": payload.ContainerID, "field": string(payload.Field),
			"generation": payload.Generation, "source_revision": payload.SourceRevision,
			"observed_at": "2026-07-26T01:00:00Z", "data": test.data,
		}
		raw, err := msgpack.Marshal(result)
		if err != nil {
			t.Fatal(err)
		}
		receipt := receiptFor(intent, Completed)
		receipt.Result = raw
		if err := receipt.ValidateFor(intent); err == nil {
			t.Errorf("unknown nested %s field validated", test.field)
		}
	}
}

func TestFleetRuntimeObservedArtifactIsReadOnlyBoundedEvidence(t *testing.T) {
	for _, item := range []FleetRuntimeObservedArtifactV1{
		{Name: "worldgen.zip", Kind: "datapack"},
		{Name: "fabric-api-1.0.0.jar", Kind: "mod", HashSHA256: strings.Repeat("a", 64)},
	} {
		if err := item.Validate(); err != nil {
			t.Errorf("valid observed artifact %#v: %v", item, err)
		}
	}
	for _, item := range []FleetRuntimeObservedArtifactV1{
		{Name: "../mod.jar", Kind: "mod"},
		{Name: "mods/mod.jar", Kind: "mod"},
		{Name: "mod.jar", Kind: "plugin"},
		{Name: "mod.jar", Kind: "mod", HashSHA256: "sha256:" + strings.Repeat("a", 64)},
		{Name: "mod.jar", Kind: "mod", HashSHA256: strings.Repeat("G", 64)},
	} {
		if err := item.Validate(); err == nil {
			t.Errorf("invalid observed artifact %#v validated", item)
		}
	}

	intent, err := NewIntent("fleet-observe-artifacts", KindFleetRuntimeObservationExecute, "fleet-observe-artifacts", fleetRuntimeObservationIntentForTest(FleetRuntimeObservationFieldArtifactsV1))
	if err != nil {
		t.Fatal(err)
	}
	result := fleetRuntimeObservationResultForTest(FleetRuntimeObservationFieldArtifactsV1)
	result.Data.Artifacts = make([]FleetRuntimeObservedArtifactV1, 257)
	for index := range result.Data.Artifacts {
		result.Data.Artifacts[index] = FleetRuntimeObservedArtifactV1{Name: "mod.jar", Kind: "mod"}
	}
	if _, err := NewCompletedReceipt(intent, result); err == nil {
		t.Fatal("oversize observed artifact inventory validated")
	}
}

func TestFleetRuntimeObservationArtifactsRejectMutationApprovalEnvelope(t *testing.T) {
	payload := fleetRuntimeObservationIntentForTest(FleetRuntimeObservationFieldArtifactsV1)
	intent, err := NewIntent("fleet-observe-artifact-envelope", KindFleetRuntimeObservationExecute, "fleet-observe-artifact-envelope", payload)
	if err != nil {
		t.Fatal(err)
	}
	result := map[string]any{
		"contract": payload.Contract, "server_id": payload.ServerID, "node_id": payload.NodeID,
		"container_id": payload.ContainerID, "field": string(payload.Field),
		"generation": payload.Generation, "source_revision": payload.SourceRevision,
		"observed_at": "2026-07-26T01:00:00Z",
		"data": map[string]any{"artifacts": []map[string]any{{
			"name": "mod.jar", "kind": "mod", "reference": "catalog/mod.jar",
			"digest": "sha256:" + strings.Repeat("a", 64), "approval_id": "approval-1", "approved": true,
		}}},
	}
	raw, err := msgpack.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	receipt := receiptFor(intent, Completed)
	receipt.Result = raw
	if err := receipt.ValidateFor(intent); err == nil {
		t.Fatal("mutation approval artifact envelope validated as observation evidence")
	}
}

func TestFleetRuntimeObservationRejectsStaleOrMalformedEvidence(t *testing.T) {
	payload := fleetRuntimeObservationIntentForTest(FleetRuntimeObservationFieldStatusV1)
	intent, err := NewIntent("fleet-observe-evidence", KindFleetRuntimeObservationExecute, "fleet-observe-evidence", payload)
	if err != nil {
		t.Fatal(err)
	}
	tests := []FleetRuntimeObservationReceiptV1{
		func() FleetRuntimeObservationReceiptV1 {
			value := fleetRuntimeObservationResultForTest(FleetRuntimeObservationFieldStatusV1)
			value.Data.Status = "healthy"
			value.ObservedAt = "not-a-time"
			return value
		}(),
		func() FleetRuntimeObservationReceiptV1 {
			value := fleetRuntimeObservationResultForTest(FleetRuntimeObservationFieldStatusV1)
			value.Data.Status = "healthy"
			value.SourceRevision = "fleet-live-v1:" + strings.Repeat("b", 64)
			return value
		}(),
		func() FleetRuntimeObservationReceiptV1 {
			value := fleetRuntimeObservationResultForTest(FleetRuntimeObservationFieldStatusV1)
			value.Data.Status = "healthy"
			value.Generation = "fleet-live-v1:short"
			value.SourceRevision = value.Generation
			return value
		}(),
	}
	for _, result := range tests {
		if _, err := NewCompletedReceipt(intent, result); err == nil {
			t.Fatalf("malformed evidence %#v validated", result)
		}
	}
}

func TestFleetRuntimeObservationRejectsUnknownControlFields(t *testing.T) {
	base := map[string]any{
		"contract": FleetRuntimeObservationContractV1, "server_id": "server-1",
		"node_id": "node-1", "container_id": "container-1", "field": "status",
		"generation": fleetRuntimeObservationRevisionForTest(), "source_revision": fleetRuntimeObservationRevisionForTest(),
	}
	for _, forbidden := range []string{"endpoint", "command", "rcon", "path", "files", "query"} {
		payload := make(map[string]any, len(base)+1)
		for key, value := range base {
			payload[key] = value
		}
		payload[forbidden] = "attacker-controlled"
		raw, err := msgpack.Marshal(payload)
		if err != nil {
			t.Fatal(err)
		}
		intent := Intent{Version: VersionV1, ID: "fleet-observe-forbidden", Kind: KindFleetRuntimeObservationExecute, IdempotencyKey: "fleet-observe-forbidden", Payload: raw}
		if err := intent.Validate(); err == nil {
			t.Errorf("payload field %q validated", forbidden)
		}
	}
}

func TestFleetRuntimeObservationResultBindingAndBounds(t *testing.T) {
	intent, err := NewIntent("fleet-observe-bound", KindFleetRuntimeObservationExecute, "fleet-observe-bound", fleetRuntimeObservationIntentForTest(FleetRuntimeObservationFieldPlayersV1))
	if err != nil {
		t.Fatal(err)
	}
	oversize := fleetRuntimeObservationResultForTest(FleetRuntimeObservationFieldPlayersV1)
	oversize.Data.Players = make([]string, 1001)
	for index := range oversize.Data.Players {
		oversize.Data.Players[index] = "player"
	}
	if _, err := NewCompletedReceipt(intent, oversize); err == nil {
		t.Fatal("oversize players result validated")
	}

	mismatch := fleetRuntimeObservationResultForTest(FleetRuntimeObservationFieldPlayersV1)
	mismatch.Data.Players = []string{"alice"}
	mismatch.ContainerID = "container-other"
	if _, err := NewCompletedReceipt(intent, mismatch); err == nil {
		t.Fatal("mismatched runtime identity validated")
	}

	twoFields := fleetRuntimeObservationResultForTest(FleetRuntimeObservationFieldPlayersV1)
	twoFields.Data.Players = []string{"alice"}
	twoFields.Data.Status = "healthy"
	if _, err := NewCompletedReceipt(intent, twoFields); err == nil {
		t.Fatal("multi-field observation result validated")
	}
}

func TestFleetRuntimeObservationResultRejectsUnknownNestedFields(t *testing.T) {
	intentPayload := fleetRuntimeObservationIntentForTest(FleetRuntimeObservationFieldStatusV1)
	intent, err := NewIntent("fleet-observe-result-unknown", KindFleetRuntimeObservationExecute, "fleet-observe-result-unknown", intentPayload)
	if err != nil {
		t.Fatal(err)
	}
	result := map[string]any{
		"contract": FleetRuntimeObservationContractV1, "server_id": intentPayload.ServerID,
		"node_id": intentPayload.NodeID, "container_id": intentPayload.ContainerID,
		"field": "status", "generation": intentPayload.Generation,
		"source_revision": intentPayload.SourceRevision, "observed_at": "2026-07-26T01:00:00Z",
		"data": map[string]any{"status": "healthy", "command": "list"},
	}
	raw, err := msgpack.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	receipt := receiptFor(intent, Completed)
	receipt.Result = raw
	if err := receipt.ValidateFor(intent); err == nil {
		t.Fatal("unknown nested result field validated")
	}
}

func fleetRuntimeObservationIntentForTest(field FleetRuntimeObservationFieldV1) FleetRuntimeObservationIntentV1 {
	revision := fleetRuntimeObservationRevisionForTest()
	return FleetRuntimeObservationIntentV1{
		Contract: FleetRuntimeObservationContractV1, ServerID: "server-1", NodeID: "node-1",
		ContainerID: "container-1", Field: field, Generation: revision, SourceRevision: revision,
	}
}

func fleetRuntimeObservationResultForTest(field FleetRuntimeObservationFieldV1) FleetRuntimeObservationReceiptV1 {
	payload := fleetRuntimeObservationIntentForTest(field)
	return FleetRuntimeObservationReceiptV1{
		Contract: payload.Contract, ServerID: payload.ServerID, NodeID: payload.NodeID,
		ContainerID: payload.ContainerID, Field: field, Generation: payload.Generation,
		SourceRevision: payload.SourceRevision, ObservedAt: "2026-07-26T01:00:00Z",
	}
}

func fleetRuntimeObservationAccessSnapshotForTest(serverID string) *FleetRuntimeObservationAccessSnapshotV1 {
	return &FleetRuntimeObservationAccessSnapshotV1{
		ServerID: serverID,
		Whitelist: []FleetRuntimeAccessSnapshotIdentityV1{{
			UUID: "12345678-1234-1234-1234-123456789abc", Name: "Player_One",
		}},
		Operators: []FleetRuntimeAccessSnapshotOperatorV1{{
			UUID: "abcdefab-cdef-abcd-efab-cdefabcdefab", Name: "Operator_One",
			Level: 4, BypassesPlayerLimit: true,
		}},
		Bans: []FleetRuntimeAccessSnapshotBanV1{{
			UUID: "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee", Name: "Banned_One",
			Created: "2026-07-26 01:00:00 +0000", Source: "Server",
			Expires: "forever", Reason: "Banned by an operator.",
		}},
		UpdatedAt: "2026-07-26T01:00:00Z",
	}
}

func fleetRuntimeObservationRevisionForTest() string {
	return "fleet-live-v1:" + strings.Repeat("a", 64)
}
