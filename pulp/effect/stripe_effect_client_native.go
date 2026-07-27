//go:build !wasip1

package effect

// Native builds have no Pulp host. Tests may replace this narrow seam.
var stripeEffectExecuteWire = func(request []byte) ([]byte, uint32) { return nil, 99 }
