// Package jwt wraps Pulp's host-owned identity.jwt.hs256 capability.
package jwt

import (
	"fmt"
	"runtime"
	"unsafe"

	"github.com/BananaLabs-OSS/Fiber/pulp"
	"github.com/vmihailenco/msgpack/v5"
)

type SignRequest struct {
	AccountID string `msgpack:"account_id"`
	SessionID string `msgpack:"session_id"`
	ExpiresAt int64  `msgpack:"expires_at"`
}
type VerifyResponse struct {
	AccountID string `msgpack:"account_id"`
	SessionID string `msgpack:"session_id"`
}

func Sign(req SignRequest) (string, error) {
	var out struct {
		Token string `msgpack:"token"`
	}
	err := roundtrip("jwt_sign", hostSign, req, &out)
	return out.Token, err
}
func Verify(token string) (VerifyResponse, error) {
	var out VerifyResponse
	return out, roundtrip("jwt_verify", hostVerify, struct {
		Token string `msgpack:"token"`
	}{token}, &out)
}
func roundtrip(name string, f func(uint32, uint32, uint32, uint32) uint32, req, out any) error {
	data, err := msgpack.Marshal(req)
	if err != nil {
		return err
	}
	var p, n uint32
	code := f(uint32(uintptr(unsafe.Pointer(&data[0]))), uint32(len(data)), uint32(uintptr(unsafe.Pointer(&p))), uint32(uintptr(unsafe.Pointer(&n))))
	runtime.KeepAlive(data)
	if code == 99 {
		return pulp.ErrCapabilityUnavailable
	}
	if code != 0 {
		return fmt.Errorf("%s host code %d", name, code)
	}
	if n == 0 {
		return fmt.Errorf("%s empty response", name)
	}
	b := append([]byte(nil), unsafe.Slice((*byte)(unsafe.Pointer(uintptr(p))), n)...)
	pulp.ReleaseHostAlloc(p, n)
	return msgpack.Unmarshal(b, out)
}
