//go:build wasip1

package effect

import "testing"

// Compile-time ABI assertions keep the three exact-object imports aligned with
// the Pulp host's four-pointer MessagePack convention. The import names live
// in the wasm directives beside these symbols.
func TestStorageExactObjectUploadWASIABISignatures(t *testing.T) {
	var _ func(uint32, uint32, uint32, uint32) uint32 = hostStorageExactObjectPresignPut
	var _ func(uint32, uint32, uint32, uint32) uint32 = hostStorageExactObjectValidatePut
	var _ func(uint32, uint32, uint32, uint32) uint32 = hostStorageExactObjectDelete
	if StorageExactObjectDeleteImport != "s3_exact_object_delete" {
		t.Fatalf("delete import = %q", StorageExactObjectDeleteImport)
	}
}
