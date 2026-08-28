//go:build wasip1

package oauth

//go:wasmimport pulp oauth_authorize_url
func hostAuthorizationURL(reqPtr, reqLen, outPtr, outLen uint32) uint32

//go:wasmimport pulp oauth_exchange
func hostExchange(reqPtr, reqLen, outPtr, outLen uint32) uint32
