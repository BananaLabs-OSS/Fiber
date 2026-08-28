//go:build wasip1

package jwt

//go:wasmimport pulp jwt_sign
func hostSign(uint32, uint32, uint32, uint32) uint32

//go:wasmimport pulp jwt_verify
func hostVerify(uint32, uint32, uint32, uint32) uint32
