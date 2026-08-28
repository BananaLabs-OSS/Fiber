//go:build !wasip1

package jwt

func hostSign(uint32, uint32, uint32, uint32) uint32   { return 99 }
func hostVerify(uint32, uint32, uint32, uint32) uint32 { return 99 }
