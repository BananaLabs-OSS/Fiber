//go:build !wasip1

package oauth

func hostAuthorizationURL(uint32, uint32, uint32, uint32) uint32 { return 99 }
func hostExchange(uint32, uint32, uint32, uint32) uint32         { return 99 }
