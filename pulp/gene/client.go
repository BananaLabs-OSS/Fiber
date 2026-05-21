package gene

import (
	"fmt"

	"github.com/BananaLabs-OSS/Fiber/pulp"
	"github.com/vmihailenco/msgpack/v5"
)

// Client is the engine-side wrapper for calling a specific gene.
// Instantiate one per loaded gene (engine does this at boot from
// its manifest's `genes` config list).
//
//	sessions := gene.NewClient("sessions")
//	info := sessions.Catalog()
//	validated, err := sessions.ValidatePurchase(req)
type Client struct {
	// Name is the target cell's identifier, matching its manifest
	// `name` field and appearing in the engine manifest's
	// `consumes = [...]` list so the Pulp host authorizes the call.
	Name string
}

// NewClient wires a client for the named gene.
func NewClient(name string) *Client {
	return &Client{Name: name}
}

func (c *Client) call(fn string, args, out any) error {
	var argBytes []byte
	if args != nil {
		b, err := msgpack.Marshal(args)
		if err != nil {
			return fmt.Errorf("gene.%s: encode: %w", fn, err)
		}
		argBytes = b
	}
	resp, err := pulp.Call(c.Name, fn, argBytes)
	if err != nil {
		return fmt.Errorf("gene.%s: %w", fn, err)
	}
	if out == nil || len(resp) == 0 {
		return nil
	}
	if err := msgpack.Unmarshal(resp, out); err != nil {
		return fmt.Errorf("gene.%s: decode: %w", fn, err)
	}
	return nil
}

// Catalog fetches the gene's SKUs + routes + admin surface. Engine
// calls this once at boot; gene caches the response client-side.
func (c *Client) Catalog() (RegistrationInfo, error) {
	var info RegistrationInfo
	err := c.call(FnCatalog, nil, &info)
	return info, err
}

// ValidatePurchase runs before engine creates an Order row.
func (c *Client) ValidatePurchase(req PurchaseRequest) (ValidatedOrder, error) {
	var out ValidatedOrder
	err := c.call(FnValidatePurchase, req, &out)
	return out, err
}

// OnOrderPaid notifies the gene a payment succeeded. Gene
// creates its records here.
func (c *Client) OnOrderPaid(order OrderView) error {
	return c.call(FnOnOrderPaid, order, nil)
}

// FulfillmentSpec asks the gene for the container shape to spawn.
func (c *Client) FulfillmentSpec(orderID string) (ServerSpec, error) {
	var out ServerSpec
	err := c.call(FnFulfillmentSpec, orderID, &out)
	return out, err
}

// OnServerReady signals the gene that Bananagine reports the
// allocated container as live. Gene typically emails + updates
// internal state.
func (c *Client) OnServerReady(orderID, serverID string) error {
	return c.call(FnOnServerReady, map[string]string{
		"order_id":  orderID,
		"server_id": serverID,
	}, nil)
}

// OnOrderRefunded notifies the gene of a refund.
func (c *Client) OnOrderRefunded(order OrderView) error {
	return c.call(FnOnOrderRefunded, order, nil)
}

// OnOrderExpired notifies the gene of expiry.
func (c *Client) OnOrderExpired(order OrderView) error {
	return c.call(FnOnOrderExpired, order, nil)
}

// HandleRoute proxies a gene-owned HTTP request.
func (c *Client) HandleRoute(req HTTPRequest) (HTTPResponse, error) {
	var out HTTPResponse
	err := c.call(FnHandleRoute, req, &out)
	return out, err
}

// AdminFragment fetches HTML for one admin tab.
func (c *Client) AdminFragment(tab string) (string, error) {
	var html string
	err := c.call(FnAdminFragment, tab, &html)
	return html, err
}

// AdminAction dispatches an admin form POST.
func (c *Client) AdminAction(action string, payload []byte) ([]byte, error) {
	var out []byte
	err := c.call(FnAdminAction, map[string]any{
		"action":  action,
		"payload": payload,
	}, &out)
	return out, err
}

// EmailTemplate asks the gene to render one email for an order.
func (c *Client) EmailTemplate(event string, order OrderView) (EmailTemplate, error) {
	var tmpl EmailTemplate
	err := c.call(FnEmailTemplate, map[string]any{
		"event": event,
		"order": order,
	}, &tmpl)
	return tmpl, err
}
