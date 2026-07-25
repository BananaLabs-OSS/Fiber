// Package enginecmd defines state-change requests a gene may return to its
// engine alongside an HTTP response. The engine validates and applies them
// only after the sibling call has returned, avoiding forbidden A->B->A calls.
package enginecmd

import "fmt"

// Command is a typed one-of. Exactly one field must be non-nil.
type Command struct {
	ScheduleOrder              *ScheduleOrder              `msgpack:"schedule_order,omitempty"`
	UnscheduleOrder            *UnscheduleOrder            `msgpack:"unschedule_order,omitempty"`
	SwapOrderTemplate          *SwapOrderTemplate          `msgpack:"swap_order_template,omitempty"`
	UpdateOrderConfig          *UpdateOrderConfig          `msgpack:"update_order_config,omitempty"`
	UpdateOrderUpgrade         *UpdateOrderUpgrade         `msgpack:"update_order_upgrade,omitempty"`
	RedeemOrder                *RedeemOrder                `msgpack:"redeem_order,omitempty"`
	ReconfigureOrder           *ReconfigureOrder           `msgpack:"reconfigure_order,omitempty"`
	ExtendOrderInstant         *ExtendOrderInstant         `msgpack:"extend_order_instant,omitempty"`
	BeginExtensionCheckout     *BeginExtensionCheckout     `msgpack:"begin_extension_checkout,omitempty"`
	BeginGiftExtensionCheckout *BeginGiftExtensionCheckout `msgpack:"begin_gift_extension_checkout,omitempty"`
	RefundOrder                *RefundOrder                `msgpack:"refund_order,omitempty"`
	Effects                    []EffectIntent              `msgpack:"effects,omitempty"`
}

// ScheduleOrder asks the engine to move an order-backed voucher into its
// scheduled state. Product validation remains in the gene; persistence and
// state-transition ownership remain in the engine.
type ScheduleOrder struct {
	OrderID         string `msgpack:"order_id"`
	ScheduledAtUnix int64  `msgpack:"scheduled_at_unix"`
	ServerType      string `msgpack:"server_type,omitempty"`
	ExtendServerID  string `msgpack:"extend_server_id,omitempty"`
	ExtendMode      string `msgpack:"extend_mode,omitempty"`
}

// UnscheduleOrder asks the engine to restore a scheduled order-backed voucher
// to its purchased state and clear scheduling/extension metadata.
type UnscheduleOrder struct {
	OrderID string `msgpack:"order_id"`
}

// SwapOrderTemplate asks the engine to change the fulfillment template of an
// unredeemed order-backed voucher after the gene has approved the swap.
type SwapOrderTemplate struct {
	OrderID        string `msgpack:"order_id"`
	TargetTemplate string `msgpack:"target_template"`
}

// UpdateOrderConfig asks the engine to persist a gene-validated fulfillment
// configuration on an unredeemed order-backed voucher.
type UpdateOrderConfig struct {
	OrderID      string `msgpack:"order_id"`
	Gamemode     string `msgpack:"gamemode"`
	Difficulty   string `msgpack:"difficulty"`
	PVP          string `msgpack:"pvp"`
	Hardcore     string `msgpack:"hardcore"`
	Seed         string `msgpack:"seed"`
	WorldType    string `msgpack:"world_type"`
	MOTD         string `msgpack:"motd"`
	GameRules    string `msgpack:"game_rules"`
	DatapackURLs string `msgpack:"datapack_urls"`
}

// UpdateOrderUpgrade asks the engine to persist either an approved free
// voucher upgrade or the Stripe intent metadata for a paid upgrade. Empty
// target fields mean "leave unchanged"; the gene must provide at least one
// state change.
type UpdateOrderUpgrade struct {
	OrderID    string `msgpack:"order_id"`
	TierID     string `msgpack:"tier_id,omitempty"`
	ServerType string `msgpack:"server_type,omitempty"`
	IntentID   string `msgpack:"intent_id,omitempty"`
	Target     string `msgpack:"target,omitempty"`
}

// RedeemOrder asks the engine to turn a purchased voucher into a paid,
// provisionable order and persist the gene-validated fulfillment inputs.
// Optional fields are sparse: an empty value leaves the existing order value
// unchanged, matching the deploy route's historical behavior.
type RedeemOrder struct {
	OrderID        string `msgpack:"order_id"`
	ServerType     string `msgpack:"server_type"`
	ExtendServerID string `msgpack:"extend_server_id,omitempty"`
	ExtendMode     string `msgpack:"extend_mode,omitempty"`
	Gamemode       string `msgpack:"gamemode,omitempty"`
	Difficulty     string `msgpack:"difficulty,omitempty"`
	PVP            string `msgpack:"pvp,omitempty"`
	Hardcore       string `msgpack:"hardcore,omitempty"`
	Seed           string `msgpack:"seed,omitempty"`
	WorldType      string `msgpack:"world_type,omitempty"`
	MOTD           string `msgpack:"motd,omitempty"`
	GameRules      string `msgpack:"game_rules,omitempty"`
	DatapackURLs   string `msgpack:"datapack_urls,omitempty"`
	DatapackIDs    string `msgpack:"datapack_ids,omitempty"`
	ModsJSON       string `msgpack:"mods_json,omitempty"`
	UploadID       string `msgpack:"upload_id,omitempty"`
	Username       string `msgpack:"username,omitempty"`
	Whitelist      string `msgpack:"whitelist,omitempty"`
	Engine         string `msgpack:"engine,omitempty"`
	Version        string `msgpack:"version,omitempty"`
}

// ReconfigureOrder asks the engine to atomically apply a validated session
// configuration, retire the current server, and enqueue a replacement. The
// engine owns the transition, the paused-time credit, and the audit record.
// Whitelist remains a pointer so an omitted value is distinct from an explicit
// request to clear it.
type ReconfigureOrder struct {
	OrderID      string  `msgpack:"order_id"`
	ServerID     string  `msgpack:"server_id"`
	ServerType   string  `msgpack:"server_type"`
	Engine       string  `msgpack:"engine,omitempty"`
	Version      string  `msgpack:"version,omitempty"`
	Gamemode     string  `msgpack:"gamemode,omitempty"`
	Difficulty   string  `msgpack:"difficulty,omitempty"`
	PVP          string  `msgpack:"pvp,omitempty"`
	Hardcore     string  `msgpack:"hardcore,omitempty"`
	Seed         string  `msgpack:"seed,omitempty"`
	WorldType    string  `msgpack:"world_type,omitempty"`
	MOTD         string  `msgpack:"motd,omitempty"`
	GameRules    string  `msgpack:"game_rules,omitempty"`
	DatapackURLs string  `msgpack:"datapack_urls,omitempty"`
	DatapackIDs  string  `msgpack:"datapack_ids,omitempty"`
	ModsJSON     string  `msgpack:"mods_json,omitempty"`
	UploadID     string  `msgpack:"upload_id,omitempty"`
	Whitelist    *string `msgpack:"whitelist,omitempty"`
}

// ExtendOrderInstant asks the engine to consume an eligible order and extend
// a running server immediately. The engine resolves the duration from its
// authoritative product state and records the transition idempotently.
type ExtendOrderInstant struct {
	OrderID        string `msgpack:"order_id"`
	ServerID       string `msgpack:"server_id"`
	IdempotencyKey string `msgpack:"idempotency_key"`
}

// BeginExtensionCheckout asks the engine to create the persisted extension
// checkout state. The engine owns the server lock, claim token, order, coupon
// reservation, and payment effect intent. AutoRedeem is a pointer because an
// omitted value retains the historical default of true.
type BeginExtensionCheckout struct {
	ServerID       string `msgpack:"server_id"`
	Email          string `msgpack:"email"`
	PromoCode      string `msgpack:"promo_code,omitempty"`
	AutoRedeem     *bool  `msgpack:"auto_redeem,omitempty"`
	IdempotencyKey string `msgpack:"idempotency_key"`
}

// BeginGiftExtensionCheckout asks the engine to create an extension purchase
// for a shared server. ServerID is the already-authorized target; the engine
// owns all persisted checkout and payment state.
type BeginGiftExtensionCheckout struct {
	ServerID       string `msgpack:"server_id"`
	Email          string `msgpack:"email"`
	PromoCode      string `msgpack:"promo_code,omitempty"`
	IdempotencyKey string `msgpack:"idempotency_key"`
}

// RefundOrder asks the engine to transition an order through its refund saga.
// The engine is the sole owner of payment-provider lookup, order state, and
// retry-safe host effect emission.
type RefundOrder struct {
	OrderID        string `msgpack:"order_id"`
	Reason         string `msgpack:"reason,omitempty"`
	IdempotencyKey string `msgpack:"idempotency_key"`
}

// EffectIntent is a durable request for a privileged host extension. A gene
// may describe an effect, but only the host executes it. The idempotency key
// must be stable across retries of the same logical operation.
type EffectIntent struct {
	Kind           string            `msgpack:"kind"`
	IdempotencyKey string            `msgpack:"idempotency_key"`
	Payload        map[string]string `msgpack:"payload,omitempty"`
}

// Validate rejects malformed or ambiguous one-of values before an engine
// touches persistent state.
func (c Command) Validate() error {
	count := 0
	if c.ScheduleOrder != nil {
		count++
		if c.ScheduleOrder.OrderID == "" {
			return fmt.Errorf("schedule_order.order_id is required")
		}
		if c.ScheduleOrder.ScheduledAtUnix <= 0 {
			return fmt.Errorf("schedule_order.scheduled_at_unix must be positive")
		}
	}
	if c.UnscheduleOrder != nil {
		count++
		if c.UnscheduleOrder.OrderID == "" {
			return fmt.Errorf("unschedule_order.order_id is required")
		}
	}
	if c.SwapOrderTemplate != nil {
		count++
		if c.SwapOrderTemplate.OrderID == "" {
			return fmt.Errorf("swap_order_template.order_id is required")
		}
		if c.SwapOrderTemplate.TargetTemplate == "" {
			return fmt.Errorf("swap_order_template.target_template is required")
		}
	}
	if c.UpdateOrderConfig != nil {
		count++
		if c.UpdateOrderConfig.OrderID == "" {
			return fmt.Errorf("update_order_config.order_id is required")
		}
	}
	if c.UpdateOrderUpgrade != nil {
		count++
		if c.UpdateOrderUpgrade.OrderID == "" {
			return fmt.Errorf("update_order_upgrade.order_id is required")
		}
		if c.UpdateOrderUpgrade.TierID == "" &&
			c.UpdateOrderUpgrade.ServerType == "" &&
			c.UpdateOrderUpgrade.IntentID == "" {
			return fmt.Errorf("update_order_upgrade requires an upgrade target or intent")
		}
		if (c.UpdateOrderUpgrade.IntentID == "") != (c.UpdateOrderUpgrade.Target == "") {
			return fmt.Errorf("update_order_upgrade intent_id and target must be provided together")
		}
	}
	if c.RedeemOrder != nil {
		count++
		if c.RedeemOrder.OrderID == "" {
			return fmt.Errorf("redeem_order.order_id is required")
		}
	}
	if c.ReconfigureOrder != nil {
		count++
		if c.ReconfigureOrder.OrderID == "" {
			return fmt.Errorf("reconfigure_order.order_id is required")
		}
		if c.ReconfigureOrder.ServerID == "" {
			return fmt.Errorf("reconfigure_order.server_id is required")
		}
		if c.ReconfigureOrder.ServerType == "" {
			return fmt.Errorf("reconfigure_order.server_type is required")
		}
	}
	if c.ExtendOrderInstant != nil {
		count++
		if c.ExtendOrderInstant.OrderID == "" {
			return fmt.Errorf("extend_order_instant.order_id is required")
		}
		if c.ExtendOrderInstant.ServerID == "" {
			return fmt.Errorf("extend_order_instant.server_id is required")
		}
		if c.ExtendOrderInstant.IdempotencyKey == "" {
			return fmt.Errorf("extend_order_instant.idempotency_key is required")
		}
	}
	if c.BeginExtensionCheckout != nil {
		count++
		if c.BeginExtensionCheckout.ServerID == "" {
			return fmt.Errorf("begin_extension_checkout.server_id is required")
		}
		if c.BeginExtensionCheckout.Email == "" {
			return fmt.Errorf("begin_extension_checkout.email is required")
		}
		if c.BeginExtensionCheckout.IdempotencyKey == "" {
			return fmt.Errorf("begin_extension_checkout.idempotency_key is required")
		}
	}
	if c.BeginGiftExtensionCheckout != nil {
		count++
		if c.BeginGiftExtensionCheckout.ServerID == "" {
			return fmt.Errorf("begin_gift_extension_checkout.server_id is required")
		}
		if c.BeginGiftExtensionCheckout.Email == "" {
			return fmt.Errorf("begin_gift_extension_checkout.email is required")
		}
		if c.BeginGiftExtensionCheckout.IdempotencyKey == "" {
			return fmt.Errorf("begin_gift_extension_checkout.idempotency_key is required")
		}
	}
	if c.RefundOrder != nil {
		count++
		if c.RefundOrder.OrderID == "" {
			return fmt.Errorf("refund_order.order_id is required")
		}
		if c.RefundOrder.IdempotencyKey == "" {
			return fmt.Errorf("refund_order.idempotency_key is required")
		}
	}
	if count != 1 {
		return fmt.Errorf("engine command must contain exactly one operation")
	}
	seenEffectKeys := make(map[string]struct{}, len(c.Effects))
	for i, effect := range c.Effects {
		if effect.Kind == "" {
			return fmt.Errorf("effects[%d].kind is required", i)
		}
		if effect.IdempotencyKey == "" {
			return fmt.Errorf("effects[%d].idempotency_key is required", i)
		}
		if _, exists := seenEffectKeys[effect.IdempotencyKey]; exists {
			return fmt.Errorf("effects[%d].idempotency_key must be unique", i)
		}
		seenEffectKeys[effect.IdempotencyKey] = struct{}{}
	}
	return nil
}
