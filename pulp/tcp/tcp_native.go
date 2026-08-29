//go:build !wasip1

package tcp

func hostListen(uint32, uint32, uint32, uint32) uint32  { return 99 }
func hostConnect(uint32, uint32, uint32, uint32) uint32 { return 99 }
func hostWrite(uint32, uint32, uint32, uint32) uint32   { return 99 }
func hostHalfClose(uint32, uint32) uint32               { return 99 }
func hostClose(uint32, uint32) uint32                   { return 99 }
func hostListenerClose(uint32, uint32) uint32           { return 99 }
