// Package cryptorand bridges the entropy.read host capability into
// Go's crypto/rand package. Importing it for side-effects replaces
// crypto/rand.Reader with one backed by entropy.Read, so any code
// that calls crypto/rand.Read (including third-party libraries like
// google/uuid, golang.org/x/crypto, and stdlib TLS) gets real
// cryptographically secure entropy from the host instead of the
// deterministic stub Pulp's wasip1 runtime exposes.
//
//	import _ "github.com/BananaLabs-OSS/Fiber/pulp/entropy/cryptorand"
//
// The cell's manifest must declare:
//
//	capabilities = ["entropy.read"]
//
// and the host binary must link Pulp-ext-entropy via blank import.
//
// Without this bridge, crypto/rand.Read returns the same byte sequence
// after every cell restart (and may repeat within a single run on some
// host configurations), causing UUID collisions, predictable OTPs,
// and other CSPRNG soundness failures.
package cryptorand

import (
	cryptorand "crypto/rand"

	"github.com/BananaLabs-OSS/Fiber/pulp/entropy"
)

// hostMaxBytes mirrors the per-call cap enforced by Pulp-ext-entropy.
// Larger reads are split into multiple host calls.
const hostMaxBytes = 1 << 16

func init() {
	cryptorand.Reader = entropyReader{}
}

type entropyReader struct{}

func (entropyReader) Read(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}
	written := 0
	for written < len(p) {
		n := len(p) - written
		if n > hostMaxBytes {
			n = hostMaxBytes
		}
		b, err := entropy.Read(uint32(n))
		if err != nil {
			return written, err
		}
		copy(p[written:written+n], b)
		written += n
	}
	return written, nil
}
