// Package gene defines the contract between Evolution's engine and
// the per-product genes (Sessions, Matches, Rooms) that plug into
// it. Engine and gene are separate WASM cells; they communicate
// via Pulp sibling calls through this package's types.
//
// # Architecture
//
// The engine owns everything product-agnostic: orders, Stripe
// payments, auth, admin desk shell, server lifecycle coordination
// with Bananagine, email sending, SSE broadcasting, persistent
// storage. Genes own product-specific logic: what SKUs exist, how
// a purchase translates into fulfillment, what emails to send at
// which lifecycle points, what admin pages to show.
//
// A gene registers itself at init by calling gene.Register in
// its pulp.OnInit. The engine discovers loaded genes via its
// manifest's `genes` config list and calls Catalog() on each to
// build the global SKU catalog and admin tab list.
//
// # Flow of a purchase
//
//	1. Browser → engine POST /api/checkout {gene:"sessions", sku, config}
//	2. Engine → gene.ValidatePurchase(req) returns ValidatedOrder
//	3. Engine creates Order row, creates Stripe PaymentIntent
//	4. Browser completes payment
//	5. Stripe webhook → engine → gene.OnOrderPaid(order)
//	6. Gene creates its own records (vouchers, tokens, etc.) and
//	   optionally requests server spawn via engine.SpawnServer
//	7. Engine.Poller detects server ready, calls gene.OnServerReady
//
// # Flow of a gene-owned HTTP request
//
//	1. Browser → engine GET /api/voucher/abc/redeem
//	2. Engine sees path matches a gene's registered routes
//	3. Engine → gene.HandleRoute(method, path, body, headers)
//	4. Gene returns HTTPResponse, engine forwards to browser
package gene

// SKU is one purchasable product a gene offers. Price is in the
// smallest currency unit (cents for USD). Metadata flows through to
// the engine's Order row unchanged.
type SKU struct {
	ID          string            `msgpack:"id"`
	Name        string            `msgpack:"name"`
	Description string            `msgpack:"description,omitempty"`
	PriceCents  int64             `msgpack:"price_cents"`
	Currency    string            `msgpack:"currency"`
	Metadata    map[string]string `msgpack:"metadata,omitempty"`
}

// AdminTab describes one UI page the gene contributes to the
// engine's admin desk. The engine's admin shell renders the sidebar
// entries and routes /admin/<gene>/<tab> requests to the gene's
// AdminFragment(tab) for the HTML body.
type AdminTab struct {
	Key    string `msgpack:"key"`    // URL slug, e.g. "tiers"
	Label  string `msgpack:"label"`  // Display name, e.g. "Tier Management"
	Icon   string `msgpack:"icon,omitempty"`
	Order  int    `msgpack:"order,omitempty"` // sort position within gene section
}

// RouteDecl is one HTTP route the gene owns. The engine mounts the
// path in its router; incoming requests proxy to gene.HandleRoute.
type RouteDecl struct {
	Method string `msgpack:"method"`
	Path   string `msgpack:"path"`
}

// RegistrationInfo is what Catalog() returns. Captures everything
// the engine needs to know about a gene at boot — SKUs, admin
// surface, HTTP routes, recognized lifecycle event subscriptions.
type RegistrationInfo struct {
	// Name is the gene identifier, matching the cell name. Used
	// by the engine to route sibling calls to this gene.
	Name string `msgpack:"name"`

	// Version is informational — engine logs it at boot.
	Version string `msgpack:"version"`

	// SKUs the gene can fulfill. Engine merges these with other
	// genes' catalogs; an order's sku field selects which gene
	// handles fulfillment.
	SKUs []SKU `msgpack:"skus"`

	// Routes are HTTP paths the engine proxies to the gene.
	Routes []RouteDecl `msgpack:"routes,omitempty"`

	// AdminTabs contribute sidebar entries under the gene's
	// section in /admin/desk.
	AdminTabs []AdminTab `msgpack:"admin_tabs,omitempty"`

	// EmailTemplates the gene can render via EmailTemplate.
	// Informational — engine logs them and validates event names
	// used by SendEmail dispatch.
	EmailTemplates []string `msgpack:"email_templates,omitempty"`
}

// PurchaseRequest is the payload engine hands to ValidatePurchase.
// Engine has already authenticated the user and confirmed the SKU
// belongs to this gene. Config is the gene-specific fields
// submitted by the purchase form (e.g. for Sessions: game, flavor,
// seed, server_name).
type PurchaseRequest struct {
	SKU         string            `msgpack:"sku"`
	Email       string            `msgpack:"email"`
	AccountID   string            `msgpack:"account_id,omitempty"`
	CouponCode  string            `msgpack:"coupon_code,omitempty"`
	Config      map[string]any    `msgpack:"config,omitempty"`
	IsGift      bool              `msgpack:"is_gift,omitempty"`
	Metadata    map[string]string `msgpack:"metadata,omitempty"`
}

// ValidatedOrder is what ValidatePurchase returns on success. Engine
// uses it to build the Order row + Stripe PaymentIntent. Error is
// non-nil when validation fails (bad config, price mismatch, etc.).
type ValidatedOrder struct {
	AmountCents int64             `msgpack:"amount_cents"`
	Currency    string            `msgpack:"currency"`
	Description string            `msgpack:"description,omitempty"`
	Metadata    map[string]string `msgpack:"metadata,omitempty"`

	// CaptureMethod override; empty means Stripe default. Genes
	// like pools use "manual" to split charge + capture.
	CaptureMethod string `msgpack:"capture_method,omitempty"`

	// SkipPayment=true for free orders (full-discount coupons). Engine
	// skips Stripe entirely and fires OnOrderPaid immediately.
	SkipPayment bool `msgpack:"skip_payment,omitempty"`
}

// OrderView is engine's projection of an Order into a gene-
// accessible struct. Genes get this in lifecycle hooks instead of
// direct DB access to avoid coupling gene code to engine's schema.
type OrderView struct {
	ID              string            `msgpack:"id"`
	Gene         string            `msgpack:"gene"`
	SKU             string            `msgpack:"sku"`
	Email           string            `msgpack:"email"`
	AccountID       string            `msgpack:"account_id,omitempty"`
	AmountCents     int64             `msgpack:"amount_cents"`
	Currency        string            `msgpack:"currency"`
	Status          string            `msgpack:"status"`
	StripePI        string            `msgpack:"stripe_pi,omitempty"`
	CouponCode      string            `msgpack:"coupon_code,omitempty"`
	IsGift          bool              `msgpack:"is_gift,omitempty"`
	GiftToken       string            `msgpack:"gift_token,omitempty"`
	Metadata        map[string]string `msgpack:"metadata,omitempty"`
	CreatedAtUnix   int64             `msgpack:"created_at_unix"`
	PaidAtUnix      int64             `msgpack:"paid_at_unix,omitempty"`
}

// ServerSpec is the shape the gene returns from FulfillmentSpec.
// Engine translates it into a Bananagine allocate request. Keep this
// a superset of what Bananagine accepts, so multiple genes can
// target different container templates.
type ServerSpec struct {
	Template     string            `msgpack:"template"`      // template name mounted in Bananagine
	Name         string            `msgpack:"name"`          // container name (e.g. "match-K7FX2M")
	Image        string            `msgpack:"image,omitempty"`
	Environment  map[string]string `msgpack:"environment,omitempty"`
	CPULimit     float64           `msgpack:"cpu_limit,omitempty"`
	MemoryLimit  int64             `msgpack:"memory_limit,omitempty"`
	MemorySwap   int64             `msgpack:"memory_swap,omitempty"`
	DiskSizeMB   int64             `msgpack:"disk_size_mb,omitempty"`
	PidsLimit    int64             `msgpack:"pids_limit,omitempty"`
	LifetimeMin  int               `msgpack:"lifetime_min,omitempty"` // auto-destroy after N minutes; 0 = persistent
	Labels       map[string]string `msgpack:"labels,omitempty"`
}

// HTTPRequest / HTTPResponse shuttle gene-owned endpoints through
// the engine's router. Mirror of pulp.HTTPRequest/Response but
// decoupled so the gene package doesn't pull in the full pulp
// runtime for callers that only want the interface.
type HTTPRequest struct {
	Method  string            `msgpack:"method"`
	Path    string            `msgpack:"path"`
	Params  map[string]string `msgpack:"params,omitempty"`
	Query   map[string]string `msgpack:"query,omitempty"`
	Headers map[string]string `msgpack:"headers,omitempty"`
	Body    []byte            `msgpack:"body,omitempty"`
}

type HTTPResponse struct {
	Status  uint32            `msgpack:"status"`
	Headers map[string]string `msgpack:"headers,omitempty"`
	Cookies []string          `msgpack:"cookies,omitempty"`
	Body    []byte            `msgpack:"body,omitempty"`
}

// EmailTemplate is the gene's response to the engine's request for
// email content. Engine handles the Resend call; gene only renders.
type EmailTemplate struct {
	Subject string `msgpack:"subject"`
	Body    string `msgpack:"body"` // rendered HTML
	// Brand optionally routes to a specific From address configured
	// in the engine's email_brands table (e.g. "sessions@sessions.gg"
	// vs "matches@sessions.gg"). Empty = engine default brand.
	Brand string `msgpack:"brand,omitempty"`
}
