package repo

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestCreateDefaultConfigFile(t *testing.T) {
	// Setup a temporary directory
	tmpDir, err := os.MkdirTemp("", "bchd")
	if err != nil {
		t.Fatalf("Failed creating a temporary directory: %v", err)
	}
	testpath := filepath.Join(tmpDir, "test.conf")

	// Clean-up
	defer func() {
		os.Remove(testpath)
		os.Remove(tmpDir)
	}()

	err = createDefaultConfigFile(testpath, false)
	if err != nil {
		t.Fatalf("Failed to create a default config file: %v", err)
	}

	_, err = os.ReadFile(testpath)
	if err != nil {
		t.Fatalf("Failed to read generated default config file: %v", err)
	}
}

func TestConfigManagedEscrowCapabilityConfig(t *testing.T) {
	t.Run("defaults to all ready ManagedEscrow chains", func(t *testing.T) {
		cfg := &Config{}
		got := cfg.ManagedEscrowCapabilityConfig()
		if got == nil {
			t.Fatal("expected non-nil default ManagedEscrow config")
		}
		if len(got.ManagedEscrowChains) == 0 {
			t.Fatal("expected ready ManagedEscrow chains in default config")
		}
	})

	t.Run("copies configured chains", func(t *testing.T) {
		cfg := &Config{ManagedEscrowChains: []string{"ETH", "BASE"}}
		got := cfg.ManagedEscrowCapabilityConfig()
		if got == nil {
			t.Fatal("expected non-nil config")
		}
		want := []string{"ETH", "BASE"}
		if !reflect.DeepEqual(got.ManagedEscrowChains, want) {
			t.Fatalf("unexpected safe chains: got %v want %v", got.ManagedEscrowChains, want)
		}
		cfg.ManagedEscrowChains[0] = "BSC"
		if !reflect.DeepEqual(got.ManagedEscrowChains, want) {
			t.Fatalf("expected returned config to be detached copy, got %v want %v", got.ManagedEscrowChains, want)
		}
	})
}
