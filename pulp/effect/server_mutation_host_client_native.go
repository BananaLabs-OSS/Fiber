//go:build !wasip1

package effect

var serverMutationHostExecuteWire = func([]byte) ([]byte, uint32) {
	return nil, 99
}
