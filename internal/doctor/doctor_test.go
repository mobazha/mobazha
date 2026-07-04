package doctor

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRunner_CheckDataDir_CurrentDatabaseLayouts_Pass(t *testing.T) {
	tests := []struct {
		name   string
		dbPath string
	}{
		{name: "root database", dbPath: "mobazha.db"},
		{name: "multi-node database", dbPath: filepath.Join("nodes", "default", "mobazha.db")},
		{name: "legacy database", dbPath: filepath.Join("datastore", "mainnet.db")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dataDir := t.TempDir()
			dbPath := filepath.Join(dataDir, tt.dbPath)
			if err := os.MkdirAll(filepath.Dir(dbPath), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(dbPath, []byte("test"), 0o600); err != nil {
				t.Fatal(err)
			}

			result := NewRunner(Config{DataDir: dataDir}).CheckDataDir()
			if result.Status != StatusPass {
				t.Fatalf("expected PASS, got %s: %s", result.Status, result.Detail)
			}
		})
	}
}

func TestRunner_CheckDataDir_LegacyTestnetDatabase_Pass(t *testing.T) {
	dataDir := t.TempDir()
	dbPath := filepath.Join(dataDir, "datastore", "testnet.db")
	if err := os.MkdirAll(filepath.Dir(dbPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dbPath, []byte("test"), 0o600); err != nil {
		t.Fatal(err)
	}

	result := NewRunner(Config{DataDir: dataDir, Testnet: true}).CheckDataDir()
	if result.Status != StatusPass {
		t.Fatalf("expected PASS, got %s: %s", result.Status, result.Detail)
	}
}

func TestRunner_CheckDataDir_ExistingDirectoryWithoutDatabase_Warns(t *testing.T) {
	result := NewRunner(Config{DataDir: t.TempDir()}).CheckDataDir()
	if result.Status != StatusWarn {
		t.Fatalf("expected WARN, got %s: %s", result.Status, result.Detail)
	}
}

func TestRunner_CheckDataDir_MissingDirectory_Fails(t *testing.T) {
	result := NewRunner(Config{DataDir: filepath.Join(t.TempDir(), "missing")}).CheckDataDir()
	if result.Status != StatusFail {
		t.Fatalf("expected FAIL, got %s: %s", result.Status, result.Detail)
	}
}
