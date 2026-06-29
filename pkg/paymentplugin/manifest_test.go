package paymentplugin

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const validManifestYAML = `
schemaVersion: 1
id: org.example.dogecoin
name: Dogecoin Payment Plugin
version: 1.0.0
apiVersion: payment.mobazha.io/v1
license: Apache-2.0
chains:
  - chainId: DOGE
    assets: [DOGE]
capabilities:
  - chain.metadata
  - address.validate
  - payment.setup
  - payment.observe
optionalCapabilities:
  - fee.estimate
permissions:
  network:
    - tcp:example.org:50002
  signing:
    - algorithm: secp256k1
      purpose: transaction
`

func TestParseManifestValid(t *testing.T) {
	manifest, err := ParseManifest([]byte(validManifestYAML))
	require.NoError(t, err)
	assert.Equal(t, "org.example.dogecoin", manifest.ID)
	assert.Equal(t, []Chain{{ChainID: "DOGE", Assets: []string{"DOGE"}}}, manifest.Chains)
}

func TestParseManifestRejectsUnknownRequiredCapability(t *testing.T) {
	data := []byte(`
schemaVersion: 1
id: org.example.bad
name: Bad Plugin
version: 1.0.0
apiVersion: payment.mobazha.io/v1
license: Apache-2.0
chains:
  - chainId: BAD
    assets: [BAD]
capabilities: [core.database.raw]
`)
	_, err := ParseManifest(data)
	require.ErrorContains(t, err, "unknown capability")
}

func TestParseManifestRejectsWildcardNetworkPermission(t *testing.T) {
	data := []byte(`
schemaVersion: 1
id: org.example.badnetwork
name: Bad Network Plugin
version: 1.0.0
apiVersion: payment.mobazha.io/v1
license: Apache-2.0
chains:
  - chainId: BAD
    assets: [BAD]
capabilities: [chain.metadata]
permissions:
  network: ["tcp:*:443"]
`)
	_, err := ParseManifest(data)
	require.ErrorContains(t, err, "invalid network permission host")
}

func TestParseManifestRejectsUnknownFields(t *testing.T) {
	data := []byte(validManifestYAML + "unknownPolicy: permissive\n")
	_, err := ParseManifest(data)
	require.ErrorContains(t, err, "field unknownPolicy not found")
}

func TestRegistryFailsClosedUntilPluginIsHealthy(t *testing.T) {
	manifest, err := ParseManifest([]byte(validManifestYAML))
	require.NoError(t, err)
	registry := NewRegistry()
	require.NoError(t, registry.Register(manifest))
	assert.Empty(t, registry.Active())

	checkedAt := time.Unix(1_750_000_000, 0).UTC()
	require.NoError(t, registry.SetHealth(manifest.ID, HealthHealthy, "", checkedAt))
	assert.Equal(t, []Manifest{manifest}, registry.Active())

	require.NoError(t, registry.SetHealth(manifest.ID, HealthDegraded, "behind tip", checkedAt))
	assert.Empty(t, registry.Active())
}

func TestRegistryRejectsDuplicatePluginID(t *testing.T) {
	manifest, err := ParseManifest([]byte(validManifestYAML))
	require.NoError(t, err)
	registry := NewRegistry()
	require.NoError(t, registry.Register(manifest))
	require.ErrorContains(t, registry.Register(manifest), "already registered")
}

func TestNilRegistryFailsClosed(t *testing.T) {
	var registry *Registry
	require.ErrorContains(t, registry.SetHealth("org.example.plugin", HealthHealthy, "", time.Now()), "registry is nil")
	assert.Empty(t, registry.Active())
}

func TestRegistryDoesNotAliasManifestSlices(t *testing.T) {
	manifest, err := ParseManifest([]byte(validManifestYAML))
	require.NoError(t, err)
	registry := NewRegistry()
	require.NoError(t, registry.Register(manifest))

	manifest.Chains[0].Assets[0] = "MUTATED"
	require.NoError(t, registry.SetHealth(manifest.ID, HealthHealthy, "", time.Now()))
	active := registry.Active()
	require.Len(t, active, 1)
	assert.Equal(t, "DOGE", active[0].Chains[0].Assets[0])

	active[0].Chains[0].Assets[0] = "MUTATED_AGAIN"
	assert.Equal(t, "DOGE", registry.Active()[0].Chains[0].Assets[0])
}
