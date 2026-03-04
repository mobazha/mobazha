package stripe

import (
	"net/http"

	gostripe "github.com/stripe/stripe-go/v82"
	"github.com/stripe/stripe-go/v82/client"
)

// Mode determines how the Stripe provider operates.
type Mode string

const (
	// ModeConnected uses a platform key with Stripe-Account header (SaaS, Direct Charge).
	ModeConnected Mode = "connected"
	// ModeDirect uses the seller's own key directly (standalone).
	ModeDirect Mode = "direct"
)

// Config holds Stripe provider configuration.
type Config struct {
	SecretKey      string
	PublishableKey string
	WebhookSecret  string
	Mode           Mode
	BackendURL     string // testing only; empty = real Stripe API
}

// newAPI creates a per-request Stripe API client, avoiding global stripe.Key mutation.
// This is critical for standalone mode where multiple sellers may have different keys,
// and for SaaS mode to keep the global state clean.
func newAPI(secretKey, backendURL string) *client.API {
	apiCfg := &gostripe.BackendConfig{
		MaxNetworkRetries: gostripe.Int64(2),
		HTTPClient:        http.DefaultClient,
	}
	uploadCfg := &gostripe.BackendConfig{
		MaxNetworkRetries: gostripe.Int64(2),
		HTTPClient:        http.DefaultClient,
	}
	if backendURL != "" {
		apiCfg.URL = gostripe.String(backendURL)
		uploadCfg.URL = gostripe.String(backendURL)
	}

	api := &client.API{}
	api.Init(secretKey, &gostripe.Backends{
		API:     gostripe.GetBackendWithConfig(gostripe.APIBackend, apiCfg),
		Uploads: gostripe.GetBackendWithConfig(gostripe.UploadsBackend, uploadCfg),
	})
	return api
}
