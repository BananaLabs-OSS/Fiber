//go:build wasip1

package effect

import "testing"

func TestStorageExactObjectDownloadReferenceWASIABI(t *testing.T) {
	var _ func(uint32, uint32, uint32, uint32) uint32 = hostStorageExactObjectDownloadReference
	if StorageExactObjectDownloadReferenceImport != "s3_exact_object_download_reference" {
		t.Fatalf("import = %q", StorageExactObjectDownloadReferenceImport)
	}
}
