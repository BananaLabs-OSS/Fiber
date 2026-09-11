package pulp

import (
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// TestOpenSourceBoundary prevents product-owned contracts and implementation
// names from leaking back into Fiber. Those belong in Marrow or a product repo.
func TestOpenSourceBoundary(t *testing.T) {
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("locate boundary test")
	}
	root := filepath.Dir(filepath.Dir(filename))
	forbidden := []string{
		"github.com/MonkeyLabs-LLC/",
		"MonkeyLabs",
		"Bananagine",
		"Bananauth",
		"BananAuth",
		"BananaKit",
		"Potassium",
		`"sessions.`,
		`"evolution.`,
		"sessions.gg",
	}

	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			if entry.Name() == ".git" {
				return filepath.SkipDir
			}
			return nil
		}
		if filepath.Ext(path) != ".go" || path == filename {
			return nil
		}
		contents, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		for _, token := range forbidden {
			if strings.Contains(string(contents), token) {
				relative, _ := filepath.Rel(root, path)
				t.Errorf("%s contains product-owned token %q", relative, token)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("scan Fiber boundary: %v", err)
	}
}
