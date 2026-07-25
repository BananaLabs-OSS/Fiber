package enginecmd

import (
	"testing"

	"github.com/vmihailenco/msgpack/v5"
)

func TestCommandValidate(t *testing.T) {
	t.Run("schedule", func(t *testing.T) {
		cmd := Command{ScheduleOrder: &ScheduleOrder{
			OrderID:         "order-1",
			ScheduledAtUnix: 1,
		}}
		if err := cmd.Validate(); err != nil {
			t.Fatalf("valid schedule rejected: %v", err)
		}
	})

	t.Run("unschedule", func(t *testing.T) {
		cmd := Command{UnscheduleOrder: &UnscheduleOrder{OrderID: "order-1"}}
		if err := cmd.Validate(); err != nil {
			t.Fatalf("valid unschedule rejected: %v", err)
		}
	})

	t.Run("swap template", func(t *testing.T) {
		cmd := Command{SwapOrderTemplate: &SwapOrderTemplate{
			OrderID:        "order-1",
			TargetTemplate: "minecraft",
		}}
		if err := cmd.Validate(); err != nil {
			t.Fatalf("valid template swap rejected: %v", err)
		}
	})

	t.Run("update config", func(t *testing.T) {
		cmd := Command{UpdateOrderConfig: &UpdateOrderConfig{OrderID: "order-1"}}
		if err := cmd.Validate(); err != nil {
			t.Fatalf("valid config update rejected: %v", err)
		}
	})

	t.Run("free upgrade", func(t *testing.T) {
		cmd := Command{UpdateOrderUpgrade: &UpdateOrderUpgrade{
			OrderID: "order-1",
			TierID:  "plus",
		}}
		if err := cmd.Validate(); err != nil {
			t.Fatalf("valid free upgrade rejected: %v", err)
		}
	})

	t.Run("paid upgrade intent", func(t *testing.T) {
		cmd := Command{UpdateOrderUpgrade: &UpdateOrderUpgrade{
			OrderID:  "order-1",
			IntentID: "pi_123",
			Target:   "plus",
		}}
		if err := cmd.Validate(); err != nil {
			t.Fatalf("valid paid upgrade rejected: %v", err)
		}
	})

	t.Run("redeem order", func(t *testing.T) {
		cmd := Command{RedeemOrder: &RedeemOrder{
			OrderID:    "order-1",
			ServerType: "minecraft",
		}}
		if err := cmd.Validate(); err != nil {
			t.Fatalf("valid redeem rejected: %v", err)
		}
	})

	t.Run("reconfigure order", func(t *testing.T) {
		whitelist := "friend-one"
		cmd := Command{ReconfigureOrder: &ReconfigureOrder{
			OrderID: "order-1", ServerID: "server-1", ServerType: "minecraft",
			Whitelist: &whitelist,
		}}
		if err := cmd.Validate(); err != nil {
			t.Fatalf("valid reconfigure rejected: %v", err)
		}
	})

	t.Run("instant extend", func(t *testing.T) {
		cmd := Command{ExtendOrderInstant: &ExtendOrderInstant{
			OrderID: "order-1", ServerID: "server-1", IdempotencyKey: "order-1:extend",
		}}
		if err := cmd.Validate(); err != nil {
			t.Fatalf("valid instant extend rejected: %v", err)
		}
	})

	t.Run("begin extension checkout", func(t *testing.T) {
		cmd := Command{BeginExtensionCheckout: &BeginExtensionCheckout{
			ServerID: "server-1", Email: "owner@example.test", IdempotencyKey: "request-1",
		}}
		if err := cmd.Validate(); err != nil {
			t.Fatalf("valid extension checkout rejected: %v", err)
		}
	})

	t.Run("begin gift extension checkout", func(t *testing.T) {
		cmd := Command{BeginGiftExtensionCheckout: &BeginGiftExtensionCheckout{
			ServerID: "server-1", Email: "buyer@example.test", IdempotencyKey: "request-1",
		}}
		if err := cmd.Validate(); err != nil {
			t.Fatalf("valid gift extension checkout rejected: %v", err)
		}
	})

	t.Run("refund order", func(t *testing.T) {
		cmd := Command{RefundOrder: &RefundOrder{
			OrderID: "order-1", IdempotencyKey: "order-1:refund",
		}}
		if err := cmd.Validate(); err != nil {
			t.Fatalf("valid refund rejected: %v", err)
		}
	})

	t.Run("effect intent", func(t *testing.T) {
		cmd := Command{
			RefundOrder: &RefundOrder{OrderID: "order-1", IdempotencyKey: "order-1:refund"},
			Effects: []EffectIntent{{
				Kind: "stripe.refund", IdempotencyKey: "order-1:stripe-refund",
				Payload: map[string]string{"order_id": "order-1"},
			}},
		}
		if err := cmd.Validate(); err != nil {
			t.Fatalf("valid effect intent rejected: %v", err)
		}
	})

	t.Run("empty", func(t *testing.T) {
		if err := (Command{}).Validate(); err == nil {
			t.Fatal("empty command accepted")
		}
	})

	t.Run("ambiguous", func(t *testing.T) {
		cmd := Command{
			ScheduleOrder: &ScheduleOrder{
				OrderID:         "order-1",
				ScheduledAtUnix: 1,
			},
			UnscheduleOrder: &UnscheduleOrder{OrderID: "order-1"},
		}
		if err := cmd.Validate(); err == nil {
			t.Fatal("ambiguous command accepted")
		}
	})

	t.Run("missing required command fields", func(t *testing.T) {
		cases := []Command{
			{ReconfigureOrder: &ReconfigureOrder{OrderID: "order-1", ServerID: "server-1"}},
			{ExtendOrderInstant: &ExtendOrderInstant{OrderID: "order-1", ServerID: "server-1"}},
			{BeginExtensionCheckout: &BeginExtensionCheckout{ServerID: "server-1", Email: "owner@example.test"}},
			{BeginGiftExtensionCheckout: &BeginGiftExtensionCheckout{ServerID: "server-1", Email: "buyer@example.test"}},
			{RefundOrder: &RefundOrder{OrderID: "order-1"}},
		}
		for _, cmd := range cases {
			if err := cmd.Validate(); err == nil {
				t.Fatalf("missing required field accepted: %#v", cmd)
			}
		}
	})

	t.Run("invalid effect intent", func(t *testing.T) {
		base := Command{RefundOrder: &RefundOrder{OrderID: "order-1", IdempotencyKey: "order-1:refund"}}
		base.Effects = []EffectIntent{{IdempotencyKey: "effect-1"}}
		if err := base.Validate(); err == nil {
			t.Fatal("effect without kind accepted")
		}
		base.Effects = []EffectIntent{{Kind: "stripe.refund"}}
		if err := base.Validate(); err == nil {
			t.Fatal("effect without idempotency key accepted")
		}
		base.Effects = []EffectIntent{
			{Kind: "stripe.refund", IdempotencyKey: "effect-1"},
			{Kind: "email.send", IdempotencyKey: "effect-1"},
		}
		if err := base.Validate(); err == nil {
			t.Fatal("duplicate effect idempotency keys accepted")
		}
	})
}

func TestCommandMessagePackRoundTrip(t *testing.T) {
	clearWhitelist := ""
	want := Command{
		ReconfigureOrder: &ReconfigureOrder{
			OrderID: "order-1", ServerID: "server-1", ServerType: "minecraft",
			Whitelist: &clearWhitelist,
		},
		Effects: []EffectIntent{{
			Kind:           "host.email.send",
			IdempotencyKey: "order-1:reconfigure:email",
			Payload:        map[string]string{"template": "reconfigured"},
		}},
	}

	encoded, err := msgpack.Marshal(want)
	if err != nil {
		t.Fatalf("marshal command: %v", err)
	}
	var got Command
	if err := msgpack.Unmarshal(encoded, &got); err != nil {
		t.Fatalf("unmarshal command: %v", err)
	}
	if err := got.Validate(); err != nil {
		t.Fatalf("round-tripped command invalid: %v", err)
	}
	if got.ReconfigureOrder == nil || got.ReconfigureOrder.Whitelist == nil || *got.ReconfigureOrder.Whitelist != "" {
		t.Fatalf("explicit empty whitelist was not preserved: %#v", got.ReconfigureOrder)
	}
	if len(got.Effects) != 1 || got.Effects[0].Payload["template"] != "reconfigured" {
		t.Fatalf("effect intent did not round-trip: %#v", got.Effects)
	}
}
