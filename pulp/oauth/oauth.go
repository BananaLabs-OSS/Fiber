// Package oauth wraps Pulp's host-owned identity.oauth.provider capability.
// Provider credentials never appear in these request or response types.
package oauth

import (
	"fmt"
	"runtime"
	"unsafe"

	"github.com/BananaLabs-OSS/Fiber/pulp"
	"github.com/vmihailenco/msgpack/v5"
)

type AuthorizationRequest struct {
	Provider        string `msgpack:"provider"`
	RedirectBinding string `msgpack:"redirect_binding"`
	State           string `msgpack:"state"`
}
type AuthorizationResponse struct {
	URL string `msgpack:"url"`
}
type ExchangeRequest struct {
	Provider        string `msgpack:"provider"`
	Code            string `msgpack:"code"`
	RedirectBinding string `msgpack:"redirect_binding"`
}
type VerifiedIdentity struct {
	Provider string `msgpack:"provider"`
	Subject  string `msgpack:"subject"`
	Email    string `msgpack:"email,omitempty"`
}

func AuthorizationURL(req AuthorizationRequest) (AuthorizationResponse, error) {
	var out AuthorizationResponse
	return out, roundtrip("oauth_authorize_url", hostAuthorizationURL, req, &out)
}
func Exchange(req ExchangeRequest) (VerifiedIdentity, error) {
	var out VerifiedIdentity
	return out, roundtrip("oauth_exchange", hostExchange, req, &out)
}

func roundtrip(name string, host func(uint32, uint32, uint32, uint32) uint32, request, response any) error {
	data, err := msgpack.Marshal(request)
	if err != nil {
		return fmt.Errorf("%s encode: %w", name, err)
	}
	var ptr, length uint32
	code := host(uint32(uintptr(unsafe.Pointer(&data[0]))), uint32(len(data)), uint32(uintptr(unsafe.Pointer(&ptr))), uint32(uintptr(unsafe.Pointer(&length))))
	runtime.KeepAlive(data)
	if code == 99 {
		return pulp.ErrCapabilityUnavailable
	}
	if code != 0 {
		return fmt.Errorf("%s host code %d", name, code)
	}
	if length == 0 {
		return fmt.Errorf("%s empty response", name)
	}
	buf := append([]byte(nil), unsafe.Slice((*byte)(unsafe.Pointer(uintptr(ptr))), length)...)
	pulp.ReleaseHostAlloc(ptr, length)
	if err := msgpack.Unmarshal(buf, response); err != nil {
		return fmt.Errorf("%s decode: %w", name, err)
	}
	return nil
}
