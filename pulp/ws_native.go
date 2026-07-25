//go:build !wasip1

package pulp

// Native builds do not have a Pulp host. Every guest import fails closed.
func hostWSRegister(ptr, ln uint32) uint32 { return 99 }

func hostWSSend(ptr, ln uint32) uint32 { return 99 }

func hostWSClose(ptr, ln uint32) uint32 { return 99 }
