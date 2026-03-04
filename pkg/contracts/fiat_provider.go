package contracts

import (
	"context"
	"errors"
)

// Sentinel errors for typed error handling in handlers.
var (
	ErrWebhookSignature = errors.New("fiat: invalid webhook signature")
	ErrProviderNotFound = errors.New("fiat: provider not found")
)

// FiatPaymentProvider is the core payment interface that all fiat providers must implement.
// Implementations live in internal/payment/fiat/{stripe,paypal}/.
type FiatPaymentProvider interface {
	// ProviderID returns the unique identifier for this provider (e.g. "stripe", "paypal").
	ProviderID() string

	// CreatePayment creates a payment session with the provider.
	//   Stripe → creates a PaymentIntent, returns clientSecret
	//   PayPal → creates an Order, returns orderID
	CreatePayment(ctx context.Context, params CreatePaymentParams) (*PaymentSession, error)

	// CapturePayment captures an authorized payment.
	//   CaptureAutomatic (Stripe): no-op, returns current status
	//   CaptureManual (PayPal): calls the Capture API
	CapturePayment(ctx context.Context, sessionID string) (*PaymentResult, error)

	// GetPayment retrieves the details of a payment.
	GetPayment(ctx context.Context, paymentID string) (*PaymentDetail, error)

	// ParseWebhook validates the webhook signature and parses the event payload
	// into a standardized WebhookEvent.
	ParseWebhook(ctx context.Context, payload []byte, headers map[string]string) (*WebhookEvent, error)
}

// FiatOnboardingProvider is an optional extension for SaaS OAuth-based seller onboarding.
// Standalone mode does not require this interface. Use type assertion to check availability:
//
//	if onboarder, ok := provider.(FiatOnboardingProvider); ok { ... }
type FiatOnboardingProvider interface {
	// GetOnboardingURL generates an OAuth URL for seller onboarding.
	// If params.AccountID is empty, the provider may auto-create an account.
	GetOnboardingURL(ctx context.Context, params OnboardingParams) (*OnboardingResult, error)

	// HandleOnboardingCallback processes the OAuth callback and returns the connected account.
	HandleOnboardingCallback(ctx context.Context, params CallbackParams) (*ProviderAccount, error)

	// GetAccountStatus queries the seller's account status with the provider.
	GetAccountStatus(ctx context.Context, accountID string) (*AccountStatus, error)
}

// FiatProviderRegistry manages registered FiatPaymentProvider instances.
// Business-level filtering (e.g. which providers a seller has enabled) belongs
// in FiatService, not here.
type FiatProviderRegistry interface {
	// Register adds a provider to the registry. Called at startup.
	Register(provider FiatPaymentProvider)

	// Unregister removes a provider from the registry.
	Unregister(id string)

	// ForProvider looks up a registered provider by ID.
	ForProvider(id string) (FiatPaymentProvider, error)

	// Registered returns all registered provider IDs (no business-state filtering).
	Registered() []string
}

// FiatService is the business-level fiat payment service exposed to handlers.
type FiatService interface {
	// EnabledProviders returns providers the current seller has configured, with status.
	EnabledProviders(ctx context.Context) ([]ProviderInfo, error)

	// CreatePayment creates a payment session. Automatically resolves the seller's
	// account ID from ReceivingAccount.
	CreatePayment(ctx context.Context, providerID string, params CreatePaymentParams) (*PaymentSession, error)

	// CapturePayment captures an authorized payment (required for PayPal, no-op for Stripe).
	CapturePayment(ctx context.Context, providerID string, sessionID string) (*PaymentResult, error)

	// GetPayment retrieves payment details.
	GetPayment(ctx context.Context, providerID string, paymentID string) (*PaymentDetail, error)

	// HandleWebhook processes a webhook event with idempotency guarantees.
	HandleWebhook(ctx context.Context, providerID string, payload []byte, headers map[string]string) error

	// --- Seller-side provider management (standalone mode) ---

	// GetProviderConfig returns the provider config with secrets masked.
	GetProviderConfig(providerID string) (*ProviderConfigView, error)

	// SaveProviderConfig stores or updates provider API keys.
	SaveProviderConfig(providerID string, cfg ProviderConfigInput) error

	// DeleteProviderConfig removes the config and deactivates the receiving account.
	DeleteProviderConfig(providerID string) error

	// VerifyProviderConfig tests the stored config by calling the provider's health endpoint.
	VerifyProviderConfig(providerID string) error

	// GetProviderStatus returns connection status for a specific provider.
	GetProviderStatus(ctx context.Context, providerID string) (*AccountStatus, error)

	// --- SaaS onboarding (platform-level, delegates to FiatOnboardingProvider) ---

	// GetOnboardingURL generates an OAuth/Account Link URL for seller onboarding.
	// Returns ErrNotImplemented if the provider does not support onboarding.
	GetOnboardingURL(ctx context.Context, providerID string, params OnboardingParams) (*OnboardingResult, error)

	// HandleOnboardingCallback processes the onboarding callback and returns account status.
	// Returns ErrNotImplemented if the provider does not support onboarding.
	HandleOnboardingCallback(ctx context.Context, providerID string, params CallbackParams) (*AccountStatus, error)
}

// FiatPlatformConfigurer allows the hosting layer to inject platform-level
// fiat providers into a tenant node's registry. Used in SaaS mode where
// the platform owns the Stripe Connect keys (ModeConnected), not the seller.
//
// Hosting obtains this via type assertion on FiatService:
//
//	if configurer, ok := fiatService.(FiatPlatformConfigurer); ok {
//	    configurer.RegisterPlatformProvider("stripe", secretKey, pubKey, webhookSecret)
//	}
type FiatPlatformConfigurer interface {
	RegisterPlatformProvider(providerID, secretKey, publishableKey, webhookSecret string)
}

// FiatPaymentProviderAccessor exposes the fiat payment subsystem.
// Handlers obtain this via type assertion on NodeService:
//
//	if fp, ok := nodeService.(FiatPaymentProviderAccessor); ok { ... }
type FiatPaymentProviderAccessor interface {
	Fiat() FiatService
}
