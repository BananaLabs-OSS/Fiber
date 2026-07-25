package gene

import (
	"fmt"

	"github.com/BananaLabs-OSS/Fiber/pulp"
	"github.com/vmihailenco/msgpack/v5"
)

// Gene is the contract a product-specific cell implements. The
// engine calls these methods via sibling calls; Register() wires them
// up so the cell's pulp_on_call dispatches correctly.
//
// Every method is optional — implementations can return an empty
// RegistrationInfo and nil errors for hooks they don't care about.
// Engine tolerates "no-op" genes during early development.
type Gene interface {
	// Catalog is called once at boot. Returns SKUs, routes, admin
	// surface. Engine caches this; call SKUsChanged() to invalidate.
	Catalog() RegistrationInfo

	// ValidatePurchase runs before the engine creates the Order row.
	// Return an error to reject; the engine surfaces the message to
	// the client as a 400.
	ValidatePurchase(PurchaseRequest) (ValidatedOrder, error)

	// OnOrderPaid fires after Stripe confirms payment (or immediately
	// for SkipPayment=true orders). The gene may update product-owned
	// records, but must not synchronously call back into the engine:
	// Pulp rejects A->B->A sibling cycles. Engine-owned state changes
	// are returned as enginecmd values from HandleRoute or performed
	// by the engine before it invokes a lifecycle hook.
	OnOrderPaid(OrderView) error

	// FulfillmentSpec returns the container shape to hand to
	// Bananagine. Engine calls this when it's time to allocate — for
	// Sessions that's at voucher redemption, for Matches that's at
	// room creation. Return an error if the order isn't ready for
	// fulfillment yet (gene holds the state machine).
	FulfillmentSpec(orderID string) (ServerSpec, error)

	// OnServerReady fires when Bananagine reports the allocated
	// container as accepting connections. Gene typically emails
	// the customer and flips its internal state (voucher.redeemed,
	// room.ready, etc.).
	OnServerReady(orderID, serverID string) error

	// OnOrderRefunded fires when engine issues a Stripe refund.
	// Gene marks its records accordingly.
	OnOrderRefunded(OrderView) error

	// OnOrderExpired fires when the engine's grace window passes for
	// an unfulfilled paid order, or a fulfilled order's TTL expires.
	// Gene decides what cleanup means (refund? expire voucher?).
	OnOrderExpired(OrderView) error

	// HandleRoute serves requests for paths declared in Catalog().Routes.
	HandleRoute(HTTPRequest) (HTTPResponse, error)

	// AdminFragment returns the HTML body for one admin tab. The
	// engine wraps it in the admin shell (nav, auth, layout).
	AdminFragment(tab string) (string, error)

	// AdminAction handles form POSTs from admin pages. Payload is
	// the raw request body; response gets wrapped by engine.
	AdminAction(action string, payload []byte) ([]byte, error)

	// EmailTemplate renders one email event for the given order. The
	// engine sends the actual email via Resend; this only builds the
	// subject + HTML body.
	EmailTemplate(event string, order OrderView) (EmailTemplate, error)
}

// Sibling function names. Engine and gene both import these
// constants to stay in sync — changing one of these strings is a
// breaking wire change requiring both sides rebuild.
const (
	FnCatalog          = "gene.catalog"
	FnValidatePurchase = "gene.validate_purchase"
	FnOnOrderPaid      = "gene.on_order_paid"
	FnFulfillmentSpec  = "gene.fulfillment_spec"
	FnOnServerReady    = "gene.on_server_ready"
	FnOnOrderRefunded  = "gene.on_order_refunded"
	FnOnOrderExpired   = "gene.on_order_expired"
	FnHandleRoute      = "gene.handle_route"
	FnAdminFragment    = "gene.admin_fragment"
	FnAdminAction      = "gene.admin_action"
	FnEmailTemplate    = "gene.email_template"
)

// Register wires a Gene implementation's methods to the matching
// sibling-call function names. Gene cells call this once in
// pulp.OnInit; after this returns, the engine can invoke any method
// via pulp.Call(geneName, FnX, args).
func Register(r Gene) {
	pulp.Provide(FnCatalog, func(_ []byte) ([]byte, error) {
		return msgpack.Marshal(r.Catalog())
	})
	pulp.Provide(FnValidatePurchase, wrap1(r.ValidatePurchase))
	pulp.Provide(FnOnOrderPaid, wrap1E(r.OnOrderPaid))
	pulp.Provide(FnFulfillmentSpec, wrapStr1(r.FulfillmentSpec))
	pulp.Provide(FnOnServerReady, func(in []byte) ([]byte, error) {
		var req struct {
			OrderID  string `msgpack:"order_id"`
			ServerID string `msgpack:"server_id"`
		}
		if err := msgpack.Unmarshal(in, &req); err != nil {
			return nil, err
		}
		if err := r.OnServerReady(req.OrderID, req.ServerID); err != nil {
			return nil, err
		}
		return nil, nil
	})
	pulp.Provide(FnOnOrderRefunded, wrap1E(r.OnOrderRefunded))
	pulp.Provide(FnOnOrderExpired, wrap1E(r.OnOrderExpired))
	pulp.Provide(FnHandleRoute, wrap1(r.HandleRoute))
	pulp.Provide(FnAdminFragment, func(in []byte) ([]byte, error) {
		var tab string
		if err := msgpack.Unmarshal(in, &tab); err != nil {
			return nil, err
		}
		out, err := r.AdminFragment(tab)
		if err != nil {
			return nil, err
		}
		return msgpack.Marshal(out)
	})
	pulp.Provide(FnAdminAction, func(in []byte) ([]byte, error) {
		var req struct {
			Action  string `msgpack:"action"`
			Payload []byte `msgpack:"payload"`
		}
		if err := msgpack.Unmarshal(in, &req); err != nil {
			return nil, err
		}
		return r.AdminAction(req.Action, req.Payload)
	})
	pulp.Provide(FnEmailTemplate, func(in []byte) ([]byte, error) {
		var req struct {
			Event string    `msgpack:"event"`
			Order OrderView `msgpack:"order"`
		}
		if err := msgpack.Unmarshal(in, &req); err != nil {
			return nil, err
		}
		tmpl, err := r.EmailTemplate(req.Event, req.Order)
		if err != nil {
			return nil, err
		}
		return msgpack.Marshal(tmpl)
	})
}

// wrap1 builds a sibling handler that msgpack-decodes one input
// struct T, calls fn, msgpack-encodes the output. Used for methods
// like ValidatePurchase that take one struct and return one struct.
func wrap1[T, R any](fn func(T) (R, error)) pulp.Provider {
	return func(in []byte) ([]byte, error) {
		var req T
		if err := msgpack.Unmarshal(in, &req); err != nil {
			return nil, fmt.Errorf("decode: %w", err)
		}
		resp, err := fn(req)
		if err != nil {
			return nil, err
		}
		return msgpack.Marshal(resp)
	}
}

// wrap1E is wrap1 for methods that return only an error.
func wrap1E[T any](fn func(T) error) pulp.Provider {
	return func(in []byte) ([]byte, error) {
		var req T
		if err := msgpack.Unmarshal(in, &req); err != nil {
			return nil, fmt.Errorf("decode: %w", err)
		}
		if err := fn(req); err != nil {
			return nil, err
		}
		return nil, nil
	}
}

// wrapStr1 is wrap1 for methods taking a single string argument.
func wrapStr1[R any](fn func(string) (R, error)) pulp.Provider {
	return func(in []byte) ([]byte, error) {
		var s string
		if err := msgpack.Unmarshal(in, &s); err != nil {
			return nil, err
		}
		resp, err := fn(s)
		if err != nil {
			return nil, err
		}
		return msgpack.Marshal(resp)
	}
}
