package contracts

import "time"

// CaptureMode indicates how a payment is captured after authorization.
type CaptureMode string

const (
	// CaptureAutomatic means the provider auto-captures on successful authorization (Stripe default).
	CaptureAutomatic CaptureMode = "automatic"
	// CaptureManual means explicit CapturePayment call is required (PayPal default).
	CaptureManual CaptureMode = "manual"
)

// CreatePaymentParams holds the parameters for creating a fiat payment session.
type CreatePaymentParams struct {
	OrderID     string
	Amount      int64  // smallest currency unit (e.g. cents for USD)
	Currency    string // ISO 4217 code (USD, EUR, GBP)
	Description string
	Metadata    map[string]string
	ReturnURL   string // required by PayPal
	CancelURL   string // required by PayPal

	// SellerAccountID is filled internally by FiatPaymentAppService;
	// handlers must not set this directly.
	SellerAccountID string
}

// PaymentSession is returned by CreatePayment with provider-specific client data.
type PaymentSession struct {
	SessionID   string      `json:"sessionID"`
	CaptureMode CaptureMode `json:"captureMode"`
	ExpiresAt   time.Time   `json:"expiresAt"`
	Status      string      `json:"status"`
	ApproveURL  string      `json:"approveURL,omitempty"` // PayPal: URL for buyer to approve the order

	Stripe *StripeSessionData `json:"stripe,omitempty"`
	PayPal *PayPalSessionData `json:"paypal,omitempty"`
}

// StripeSessionData contains client-side data for Stripe Elements integration.
type StripeSessionData struct {
	ClientSecret       string `json:"clientSecret"`
	PublishableKey     string `json:"publishableKey"`
	ConnectedAccountID string `json:"connectedAccountId,omitempty"`
}

// PayPalSessionData contains client-side data for PayPal JS SDK integration.
type PayPalSessionData struct {
	OrderID  string `json:"orderID"`
	ClientID string `json:"clientID"`
}

// PaymentResult is returned by CapturePayment with the final payment status.
type PaymentResult struct {
	PaymentID     string            `json:"paymentID"`
	Status        string            `json:"status"` // "succeeded", "failed", "pending"
	Amount        int64             `json:"amount"`
	Currency      string            `json:"currency"`
	PaymentMethod PaymentMethodInfo `json:"paymentMethod"`
}

// PaymentMethodInfo describes the payment instrument used.
type PaymentMethodInfo struct {
	Type  string `json:"type"`  // "card", "paypal", "venmo", "apple_pay"
	Brand string `json:"brand"` // "visa", "mastercard", "paypal"
	Last4 string `json:"last4"` // "4242" or "" for non-card
}

// PaymentDetail is returned by GetPayment for querying a payment.
type PaymentDetail struct {
	PaymentID       string            `json:"paymentID"`
	Status          string            `json:"status"`
	Amount          int64             `json:"amount"`
	Currency        string            `json:"currency"`
	SellerAccountID string            `json:"sellerAccountID"`
	PaymentMethod   PaymentMethodInfo `json:"paymentMethod"`
	CreatedAt       time.Time         `json:"createdAt"`
	ReceiptURL      string            `json:"receiptURL,omitempty"`
}

// WebhookEvent is the standardized representation of a provider webhook event.
type WebhookEvent struct {
	EventID    string           // original event ID (for idempotency dedup)
	Type       WebhookEventType // standardized event type
	ProviderID string           // fiat provider: "stripe" | "paypal"
	PaymentID  string           // provider's payment ID
	OrderID    string           // extracted from metadata
	AccountID  string           // seller's account ID (for SaaS routing)
	Raw        interface{}      // original parsed event object

	// Enriched fields (best-effort, may be zero-valued if provider API is unreachable)
	Coin          string            // e.g. "fiat:USD" — used by PaymentSent to identify fiat payments
	Amount        int64             // payment amount in minimal units (cents)
	Currency      string            // ISO 4217 currency code
	PaymentMethod PaymentMethodInfo // card/wallet details
}

// WebhookEventType is a standardized webhook event category.
type WebhookEventType string

const (
	WebhookPaymentSucceeded WebhookEventType = "payment.succeeded"
	WebhookPaymentFailed    WebhookEventType = "payment.failed"
	WebhookDisputeOpened    WebhookEventType = "dispute.opened"
	WebhookDisputeResolved  WebhookEventType = "dispute.resolved"
	WebhookRefundCreated    WebhookEventType = "refund.created"
	WebhookAccountUpdated   WebhookEventType = "account.updated"
)

// OnboardingParams holds the parameters for generating an onboarding URL.
type OnboardingParams struct {
	AccountID  string // Pre-created provider account ID (e.g. Stripe acct_xxx for Account Link flow)
	SellerID   string // Platform-level seller ID (peer ID), used for state tracking
	ReturnURL  string
	RefreshURL string
	Country    string // ISO 3166-1 alpha-2 (optional)
	Email      string // optional
}

// OnboardingResult is returned by GetOnboardingURL with the onboarding URL and
// the provider account ID (which may have been auto-created during the call).
type OnboardingResult struct {
	URL       string `json:"url"`
	AccountID string `json:"accountID,omitempty"`
}

// CallbackParams holds the data received from an onboarding callback.
type CallbackParams struct {
	AccountID    string // Provider account ID (for Account Link flow: acct_xxx)
	Code         string // OAuth authorization code (for OAuth flow: ac_xxx)
	TrackingID   string // partner referral tracking ID (PayPal)
	MerchantIDPP string // merchant_id_in_paypal (PayPal)
	State        string // OAuth state parameter
}

// ProviderAccount is returned by HandleOnboardingCallback with the connected account info.
type ProviderAccount struct {
	ProviderID string `json:"providerID"`
	AccountID  string `json:"accountID"`
	Email      string `json:"email"`
	Status     string `json:"status"` // "active", "pending", "restricted"
}

// AccountStatus represents a seller's account status with a fiat provider.
type AccountStatus struct {
	AccountID      string   `json:"accountID"`
	Email          string   `json:"email,omitempty"`
	IsActive       bool     `json:"isActive"`
	Status         string   `json:"status"`
	ChargesEnabled bool     `json:"chargesEnabled"`
	PayoutsEnabled bool     `json:"payoutsEnabled"`
	Requirements   []string `json:"requirements,omitempty"`
}

// ProviderInfo describes a fiat provider's status for a specific seller.
type ProviderInfo struct {
	ProviderID string `json:"providerID"`
	Status     string `json:"status"`    // "active", "not_connected", "pending", "restricted"
	AccountID  string `json:"accountID"` // non-empty when connected
}

// ProviderConfigView is the API response for provider config (secrets masked).
type ProviderConfigView struct {
	ProviderID    string `json:"providerID"`
	AccountID     string `json:"accountID,omitempty"`
	PublicKey     string `json:"publicKey,omitempty"`
	SecretKey     string `json:"secretKey"` // masked: "sk_l****ive"
	WebhookSecret string `json:"webhookSecret,omitempty"` // masked: "****"
	IsActive      bool   `json:"isActive"`
}

// ProviderConfigInput is the API request body for saving provider config.
type ProviderConfigInput struct {
	AccountID     string `json:"accountID"`
	PublicKey     string `json:"publicKey"`
	SecretKey     string `json:"secretKey"`
	WebhookSecret string `json:"webhookSecret"`
}

// RefundParams holds the parameters for refunding a fiat payment.
type RefundParams struct {
	// PaymentID is the original payment identifier.
	//   Stripe: PaymentIntent ID (pi_xxx)
	//   PayPal: Capture ID
	PaymentID string

	// Amount is the refund amount in smallest currency unit (e.g. cents).
	// nil = full refund; non-nil = partial refund.
	Amount *int64

	// Currency is the ISO 4217 code (e.g. "USD", "EUR").
	Currency string

	// Reason describes why the refund is issued.
	//   Stripe: "requested_by_customer" | "duplicate" | "fraudulent"
	//   PayPal: free-text note_to_payer
	Reason string

	// Metadata holds additional key-value pairs (e.g. orderID).
	Metadata map[string]string
}

// RefundResult holds the outcome of a refund operation.
type RefundResult struct {
	// RefundID is the provider-assigned refund identifier.
	//   Stripe: re_xxx
	//   PayPal: refund ID
	RefundID string

	// Status indicates the refund state:
	//   "succeeded" — refund completed
	//   "pending"   — refund in progress (PayPal may be async)
	//   "failed"    — refund failed
	Status string

	// Amount is the actual refunded amount in smallest currency unit.
	Amount int64

	// Currency is the ISO 4217 currency code.
	Currency string
}
