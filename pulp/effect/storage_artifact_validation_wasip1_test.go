//go:build wasip1

package effect

import "testing"

func TestStorageArtifactZIPValidationWASIImportContract(t *testing.T) {
	if StorageExactObjectArtifactZIPValidateImport != "s3_exact_object_validate_artifact_zip" {
		t.Fatalf("import = %q", StorageExactObjectArtifactZIPValidateImport)
	}
	var _ func(uint32, uint32, uint32, uint32) uint32 = hostStorageExactObjectArtifactZIPValidate
}
