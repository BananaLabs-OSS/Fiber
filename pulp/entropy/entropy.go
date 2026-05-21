// Package entropy is the cell-side wrapper for the entropy.read
// capability provided by Pulp-ext-entropy. Cell code calls entropy.Read
// to get cryptographically secure random bytes from the host's
// crypto/rand without having to implement its own CSPRNG inside WASM.
//
//	import "github.com/BananaLabs-OSS/Fiber/pulp/entropy"
//
//	buf, err := entropy.Read(32)
//	// buf is 32 bytes of CSPRNG output
//
// The cell's manifest must declare:
//
//	capabilities = ["entropy.read"]
//
// and the host binary must link Pulp-ext-entropy via blank import.
package entropy

import (
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"runtime"
	"unsafe"

	"github.com/BananaLabs-OSS/Fiber/pulp"
	"github.com/vmihailenco/msgpack/v5"
)

// OTPDefaultAlphabet matches Bananauth's existing OTP alphabet: A-Z0-9.
const OTPDefaultAlphabet = "ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

const maxBytes = 1 << 16 // 64 KiB per call — matches the host-side cap.

//go:wasmimport pulp entropy_read
func hostEntropyRead(reqPtr, reqLen, respPtrOut, respLenOut uint32) uint32

type readRequest struct {
	N uint32 `msgpack:"n"`
}

type readResponse struct {
	Bytes []byte `msgpack:"bytes"`
}

// Read returns n bytes of cryptographically secure randomness sourced
// from the host's crypto/rand. n must be in [1, 65536]. A zero or
// oversized n returns an error without calling the host.
func Read(n uint32) ([]byte, error) {
	if n == 0 {
		return nil, fmt.Errorf("entropy.Read: n must be > 0")
	}
	if n > maxBytes {
		return nil, fmt.Errorf("entropy.Read: n=%d exceeds max %d", n, maxBytes)
	}

	reqBytes, err := msgpack.Marshal(readRequest{N: n})
	if err != nil {
		return nil, fmt.Errorf("entropy.Read: marshal request: %w", err)
	}

	var reqPtr, reqLen uint32
	if len(reqBytes) > 0 {
		reqPtr = uint32(uintptr(unsafe.Pointer(&reqBytes[0])))
		reqLen = uint32(len(reqBytes))
	}

	var respPtr, respLen uint32
	code := hostEntropyRead(
		reqPtr, reqLen,
		uint32(uintptr(unsafe.Pointer(&respPtr))),
		uint32(uintptr(unsafe.Pointer(&respLen))),
	)
	runtime.KeepAlive(reqBytes)
	if code != 0 {
		return nil, codeToError(code)
	}

	if respLen == 0 {
		return nil, fmt.Errorf("entropy.Read: empty response")
	}
	respBytes := unsafe.Slice((*byte)(unsafe.Pointer(uintptr(respPtr))), respLen)
	var resp readResponse
	if err := msgpack.Unmarshal(respBytes, &resp); err != nil {
		return nil, fmt.Errorf("entropy.Read: unmarshal response: %w", err)
	}
	if uint32(len(resp.Bytes)) != n {
		return nil, fmt.Errorf("entropy.Read: expected %d bytes, got %d", n, len(resp.Bytes))
	}
	return resp.Bytes, nil
}

// UUID returns a canonical lowercase RFC 4122 UUIDv4 string sourced from Read.
func UUID() (string, error) {
	b, err := Read(16)
	if err != nil {
		if errors.Is(err, pulp.ErrCapabilityUnavailable) {
			return "", err
		}
		return "", err
	}
	// Set version (4) and variant (RFC 4122) bits.
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		b[0:4], b[4:6], b[6:8], b[8:10], b[10:16]), nil
}

// IntN returns a uniform random int in [0, n) using rejection sampling to avoid modulo bias.
func IntN(n int) (int, error) {
	if n <= 0 {
		return 0, fmt.Errorf("entropy.IntN: n must be > 0")
	}
	un := uint64(n)
	// Largest multiple of n that fits in uint64; reject values >= that.
	limit := (^uint64(0) / un) * un
	for {
		b, err := Read(8)
		if err != nil {
			if errors.Is(err, pulp.ErrCapabilityUnavailable) {
				return 0, err
			}
			return 0, err
		}
		v := binary.BigEndian.Uint64(b)
		if v < limit {
			return int(v % un), nil
		}
	}
}

// Hex returns n random bytes encoded as a lowercase hex string (length 2n).
func Hex(n int) (string, error) {
	if n <= 0 {
		return "", fmt.Errorf("entropy.Hex: n must be > 0")
	}
	b, err := Read(uint32(n))
	if err != nil {
		if errors.Is(err, pulp.ErrCapabilityUnavailable) {
			return "", err
		}
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// Base64URL returns n random bytes encoded as unpadded base64url.
func Base64URL(n int) (string, error) {
	if n <= 0 {
		return "", fmt.Errorf("entropy.Base64URL: n must be > 0")
	}
	b, err := Read(uint32(n))
	if err != nil {
		if errors.Is(err, pulp.ErrCapabilityUnavailable) {
			return "", err
		}
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// OTP returns a length-char one-time password whose characters are drawn uniformly from alphabet.
func OTP(length int, alphabet string) (string, error) {
	if length <= 0 {
		return "", fmt.Errorf("entropy.OTP: length must be > 0")
	}
	if len(alphabet) == 0 {
		return "", fmt.Errorf("entropy.OTP: alphabet must be non-empty")
	}
	out := make([]byte, length)
	for i := 0; i < length; i++ {
		idx, err := IntN(len(alphabet))
		if err != nil {
			if errors.Is(err, pulp.ErrCapabilityUnavailable) {
				return "", err
			}
			return "", err
		}
		out[i] = alphabet[idx]
	}
	return string(out), nil
}

// OTPDefault returns an OTP of the given length over the A-Z0-9 alphabet.
func OTPDefault(length int) (string, error) {
	return OTP(length, OTPDefaultAlphabet)
}

func codeToError(c uint32) error {
	switch c {
	case 1:
		return fmt.Errorf("entropy.Read: empty request")
	case 2:
		return fmt.Errorf("entropy.Read: request memory read failed")
	case 3:
		return fmt.Errorf("entropy.Read: request decode failed")
	case 4:
		return fmt.Errorf("entropy.Read: n=0 rejected by host")
	case 5:
		return fmt.Errorf("entropy.Read: n exceeds host maximum")
	case 6:
		return fmt.Errorf("entropy.Read: host crypto/rand failed")
	case 99:
		return fmt.Errorf("entropy.Read: %w", pulp.ErrCapabilityUnavailable)
	default:
		return fmt.Errorf("entropy.Read: host error code %d", c)
	}
}
