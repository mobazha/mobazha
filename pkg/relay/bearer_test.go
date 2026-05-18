package relay

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBearerFromConfigOrEnv_PrefersConfig(t *testing.T) {
	t.Setenv(EnvPlatformRelayToken, "env")
	assert.Equal(t, "flag", BearerFromConfigOrEnv("flag"))
}

func TestBearerFromConfigOrEnv_Fallback(t *testing.T) {
	t.Setenv(EnvPlatformRelayToken, "env")
	assert.Equal(t, "env", BearerFromConfigOrEnv(""))
	assert.Equal(t, "env", BearerFromConfigOrEnv("   "))
}

func TestBearerFromConfigOrEnv_Empty(t *testing.T) {
	t.Setenv(EnvPlatformRelayToken, "")
	assert.Equal(t, "", BearerFromConfigOrEnv(""))
}
