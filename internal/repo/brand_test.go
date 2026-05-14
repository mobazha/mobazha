package repo

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadBrandConfig_NetworkFieldsRoundTrip(t *testing.T) {
	dir := t.TempDir()
	yaml := `brand:
  name: "Test Brand"
  network:
    allowUserCustomNode: true
    showAdvancedDiagnostics: true
    showNodePoolUI: true
    allowDiscoverToggle: false
`
	require.NoError(t, os.WriteFile(filepath.Join(dir, "brand.yaml"), []byte(yaml), 0o600))

	cfg, err := LoadBrandConfig(dir)
	require.NoError(t, err)
	require.NotNil(t, cfg)
	assert.Equal(t, "Test Brand", cfg.Name)
	assert.True(t, cfg.Network.AllowUserCustomNode)
	assert.True(t, cfg.Network.ShowAdvancedDiagnostics)
	assert.True(t, cfg.Network.ShowNodePoolUI)
	assert.False(t, cfg.Network.AllowDiscoverToggle, "AllowDiscoverToggle defaults false when explicitly set to false")
}

func TestLoadBrandConfig_NetworkFieldsDefaultFalse(t *testing.T) {
	dir := t.TempDir()
	// brand.yaml without a network: section — every NetworkFields entry
	// must default to false (the locked-down baseline).
	yaml := `brand:
  name: "Bare Brand"
`
	require.NoError(t, os.WriteFile(filepath.Join(dir, "brand.yaml"), []byte(yaml), 0o600))

	cfg, err := LoadBrandConfig(dir)
	require.NoError(t, err)
	require.NotNil(t, cfg)
	assert.Equal(t, "Bare Brand", cfg.Name)
	assert.False(t, cfg.Network.AllowUserCustomNode)
	assert.False(t, cfg.Network.ShowAdvancedDiagnostics)
	assert.False(t, cfg.Network.ShowNodePoolUI)
	assert.False(t, cfg.Network.AllowDiscoverToggle)
}

func TestLoadBrandConfig_ExampleExampleParses(t *testing.T) {
	// Sanity check: the bundled examples must parse and reflect the
	// Example "everything hidden" baseline. If a partner accidentally
	// flips a flag on we want CI to surface that, not production.
	cfg, err := LoadBrandConfig("../../deploy/private_distribution/examples/example")
	require.NoError(t, err)
	require.NotNil(t, cfg)
	assert.Equal(t, "Example Market", cfg.Name)
	assert.False(t, cfg.Network.AllowUserCustomNode,
		"Example baseline must keep custom-node entry hidden")
	assert.False(t, cfg.Network.ShowAdvancedDiagnostics,
		"Example baseline must keep diagnostics hidden")
	assert.False(t, cfg.Network.ShowNodePoolUI,
		"Example baseline must keep node pool UI hidden")
	assert.False(t, cfg.Network.AllowDiscoverToggle,
		"Example baseline must keep discover toggle hidden")
}

func TestLoadBrandConfig_ExampleMarketExampleParses(t *testing.T) {
	cfg, err := LoadBrandConfig("../../deploy/private_distribution/examples/examplemarket")
	require.NoError(t, err)
	require.NotNil(t, cfg)
	assert.Equal(t, "Example Market", cfg.Name)
	// TMP opts in to diagnostics + node pool UI but NOT to custom node.
	assert.False(t, cfg.Network.AllowUserCustomNode,
		"TMP keeps custom-node entry off — arbitrary RPC paste is a phishing vector")
	assert.True(t, cfg.Network.ShowAdvancedDiagnostics)
	assert.True(t, cfg.Network.ShowNodePoolUI)
	assert.True(t, cfg.Network.AllowDiscoverToggle)
}
