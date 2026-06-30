package repo

import (
	"os"
	"path/filepath"
	"testing"
)

func TestEnsureInitializedCreatesApplicationAndNodeRepositories(t *testing.T) {
	dataDir := t.TempDir()
	if err := EnsureInitialized(dataDir, DefaultNodeID, true); err != nil {
		t.Fatalf("EnsureInitialized: %v", err)
	}
	if !IsInitialized(dataDir) {
		t.Fatal("application repository should be initialized")
	}
	for _, versionPath := range []string{
		filepath.Join(dataDir, "version"),
		filepath.Join(dataDir, "nodes", DefaultNodeID, "version"),
	} {
		if _, err := os.Stat(versionPath); err != nil {
			t.Fatalf("expected version marker %s: %v", versionPath, err)
		}
	}
	if err := EnsureInitialized(dataDir, DefaultNodeID, true); err != nil {
		t.Fatalf("EnsureInitialized should be idempotent: %v", err)
	}
}
