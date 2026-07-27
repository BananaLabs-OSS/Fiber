//go:build wasip1

package httpprobe

import "testing"

// Keep the guest declaration aligned with the host's four-pointer MessagePack
// ABI. The import directive beside hostHTTPProbeExecute fixes its module/name.
func TestHTTPProbeWASIABI(t *testing.T) {
	if Capability != "effect.http.probe.v1" || Import != "http_probe_execute" {
		t.Fatalf("capability/import = %q/%q", Capability, Import)
	}
	var _ func(uint32, uint32, uint32, uint32) uint32 = hostHTTPProbeExecute
}
