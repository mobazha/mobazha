package core

import (
	"context"
	"testing"

	"github.com/mobazha/mobazha3.0/internal/repo"
	"github.com/mobazha/mobazha3.0/pkg/config"
	"github.com/stretchr/testify/assert"
)

func TestConfigNodeFeatureProvider_GuestCheckoutReflectsCLIFlag(t *testing.T) {
	t.Run("flag off → disabled", func(t *testing.T) {
		p := NewConfigNodeFeatureProvider(&repo.Config{GuestCheckout: false})
		assert.False(t, p.IsEnabled(context.Background(), config.FeatureGuestCheckout.Key))
	})

	t.Run("flag on → enabled", func(t *testing.T) {
		p := NewConfigNodeFeatureProvider(&repo.Config{GuestCheckout: true})
		assert.True(t, p.IsEnabled(context.Background(), config.FeatureGuestCheckout.Key))
	})
}

func TestConfigNodeFeatureProvider_UnknownKeyPassthrough(t *testing.T) {
	// Features without a reader must return true so the resolver's
	// AND merge does not veto the other layers. Only Guest Checkout
	// is bound by default today.
	p := NewConfigNodeFeatureProvider(&repo.Config{})
	assert.True(t, p.IsEnabled(context.Background(), "unknownFeatureXyz"))
}

func TestConfigNodeFeatureProvider_NilConfigManagedEscrow(t *testing.T) {
	// Guest Checkout reader must return false when cfg is nil so a
	// mis-wired node does not leak an enabled state.
	p := NewConfigNodeFeatureProvider(nil)
	assert.False(t, p.IsEnabled(context.Background(), config.FeatureGuestCheckout.Key))
}

func TestConfigNodeFeatureProvider_NilReceiverManagedEscrow(t *testing.T) {
	var p *ConfigNodeFeatureProvider
	// Nil-receiver passthrough mirrors AllowAllNodeProvider semantics so
	// callers can chain providers without nil guards.
	assert.True(t, p.IsEnabled(context.Background(), config.FeatureGuestCheckout.Key))
	assert.Nil(t, p.WithReader("x", func(*repo.Config) bool { return true }))
}

func TestConfigNodeFeatureProvider_WithReaderOverrides(t *testing.T) {
	p := NewConfigNodeFeatureProvider(&repo.Config{})
	p.WithReader("customFlag", func(c *repo.Config) bool { return true })
	assert.True(t, p.IsEnabled(context.Background(), "customFlag"))

	// WithReader on empty key / nil fn must be a no-op.
	p.WithReader("", func(c *repo.Config) bool { return true })
	p.WithReader("customFlag2", nil)
	assert.True(t, p.IsEnabled(context.Background(), "customFlag2"), "unregistered key → passthrough true")
}
