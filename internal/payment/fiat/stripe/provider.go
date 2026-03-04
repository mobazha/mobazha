package stripe

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	gostripe "github.com/stripe/stripe-go/v82"
	"github.com/stripe/stripe-go/v82/webhook"

	"github.com/mobazha/mobazha3.0/pkg/contracts"
)

const providerID = "stripe"

// Provider implements contracts.FiatPaymentProvider and contracts.FiatOnboardingProvider
// for Stripe Connect Standard (SaaS) and Direct (standalone) modes.
type Provider struct {
	config Config
}

// NewProvider creates a new Stripe provider with the given configuration.
func NewProvider(cfg Config) *Provider {
	return &Provider{config: cfg}
}

func (p *Provider) ProviderID() string { return providerID }

func (p *Provider) CreatePayment(_ context.Context, params contracts.CreatePaymentParams) (*contracts.PaymentSession, error) {
	api := newAPI(p.config.SecretKey, p.config.BackendURL)

	piParams := &gostripe.PaymentIntentParams{
		Amount:   gostripe.Int64(params.Amount),
		Currency: gostripe.String(params.Currency),
		AutomaticPaymentMethods: &gostripe.PaymentIntentAutomaticPaymentMethodsParams{
			Enabled: gostripe.Bool(true),
		},
	}

	if params.Metadata != nil {
		piParams.Metadata = params.Metadata
	} else {
		piParams.Metadata = make(map[string]string)
	}
	piParams.Metadata["order_id"] = params.OrderID

	if params.Description != "" {
		piParams.Description = gostripe.String(params.Description)
	}

	if p.config.Mode == ModeConnected && params.SellerAccountID != "" {
		piParams.Params.StripeAccount = gostripe.String(params.SellerAccountID)
	}

	pi, err := api.PaymentIntents.New(piParams)
	if err != nil {
		return nil, fmt.Errorf("stripe: create payment intent: %w", err)
	}

	publishableKey := p.config.PublishableKey

	return &contracts.PaymentSession{
		SessionID:   pi.ID,
		CaptureMode: contracts.CaptureAutomatic,
		ExpiresAt:   time.Now().Add(30 * time.Minute),
		Status:      string(pi.Status),
		Stripe: &contracts.StripeSessionData{
			ClientSecret:   pi.ClientSecret,
			PublishableKey: publishableKey,
		},
	}, nil
}

func (p *Provider) CapturePayment(_ context.Context, sessionID string) (*contracts.PaymentResult, error) {
	api := newAPI(p.config.SecretKey, p.config.BackendURL)

	pi, err := api.PaymentIntents.Get(sessionID, nil)
	if err != nil {
		return nil, fmt.Errorf("stripe: get payment intent: %w", err)
	}

	return &contracts.PaymentResult{
		PaymentID:     pi.ID,
		Status:        mapStripeStatus(pi.Status),
		Amount:        pi.Amount,
		Currency:      string(pi.Currency),
		PaymentMethod: extractPaymentMethod(pi),
	}, nil
}

func (p *Provider) GetPayment(_ context.Context, paymentID string) (*contracts.PaymentDetail, error) {
	api := newAPI(p.config.SecretKey, p.config.BackendURL)

	params := &gostripe.PaymentIntentParams{}
	params.AddExpand("payment_method")
	pi, err := api.PaymentIntents.Get(paymentID, params)
	if err != nil {
		return nil, fmt.Errorf("stripe: get payment: %w", err)
	}

	detail := &contracts.PaymentDetail{
		PaymentID:     pi.ID,
		Status:        mapStripeStatus(pi.Status),
		Amount:        pi.Amount,
		Currency:      string(pi.Currency),
		PaymentMethod: extractPaymentMethod(pi),
		CreatedAt:     time.Unix(pi.Created, 0),
	}

	if pi.LatestCharge != nil {
		detail.ReceiptURL = pi.LatestCharge.ReceiptURL
	}

	if pi.TransferData != nil && pi.TransferData.Destination != nil {
		detail.SellerAccountID = pi.TransferData.Destination.ID
	}

	return detail, nil
}

func (p *Provider) ParseWebhook(_ context.Context, payload []byte, headers map[string]string) (*contracts.WebhookEvent, error) {
	sig := headers["Stripe-Signature"]
	if sig == "" {
		sig = headers["stripe-signature"]
	}
	if sig == "" {
		return nil, fmt.Errorf("%w: missing Stripe-Signature header", contracts.ErrWebhookSignature)
	}

	event, err := webhook.ConstructEvent(payload, sig, p.config.WebhookSecret)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", contracts.ErrWebhookSignature, err)
	}

	we := &contracts.WebhookEvent{
		EventID: event.ID,
		Raw:     &event,
	}

	if event.Account != "" {
		we.AccountID = event.Account
	}

	switch event.Type {
	case "payment_intent.succeeded":
		we.Type = contracts.WebhookPaymentSucceeded
		pi, err := extractPaymentIntent(event.Data.Raw)
		if err != nil {
			return nil, err
		}
		we.PaymentID = pi.ID
		we.OrderID = pi.Metadata["order_id"]
		we.Coin = "fiat:" + strings.ToUpper(string(pi.Currency))
		we.Amount = pi.Amount
		we.Currency = strings.ToUpper(string(pi.Currency))
		we.PaymentMethod = extractPaymentMethod(pi)
		if we.AccountID == "" && pi.TransferData != nil && pi.TransferData.Destination != nil {
			we.AccountID = pi.TransferData.Destination.ID
		}

	case "payment_intent.payment_failed":
		we.Type = contracts.WebhookPaymentFailed
		pi, err := extractPaymentIntent(event.Data.Raw)
		if err != nil {
			return nil, err
		}
		we.PaymentID = pi.ID
		we.OrderID = pi.Metadata["order_id"]
		we.Coin = "fiat:" + strings.ToUpper(string(pi.Currency))
		we.Amount = pi.Amount
		we.Currency = strings.ToUpper(string(pi.Currency))

	case "charge.dispute.created":
		we.Type = contracts.WebhookDisputeOpened

	case "charge.dispute.closed":
		we.Type = contracts.WebhookDisputeResolved

	case "charge.refunded":
		we.Type = contracts.WebhookRefundCreated

	case "account.updated":
		we.Type = contracts.WebhookAccountUpdated
		var acct struct {
			ID string `json:"id"`
		}
		if err := json.Unmarshal(event.Data.Raw, &acct); err == nil {
			we.AccountID = acct.ID
		}

	default:
		we.Type = WebhookEventType(event.Type)
	}

	return we, nil
}

// --- FiatOnboardingProvider (SaaS only) ---

func (p *Provider) GetOnboardingURL(_ context.Context, params contracts.OnboardingParams) (*contracts.OnboardingResult, error) {
	api := newAPI(p.config.SecretKey, p.config.BackendURL)

	accountID := params.AccountID
	if accountID == "" {
		acctParams := &gostripe.AccountParams{
			Type: gostripe.String(string(gostripe.AccountTypeStandard)),
		}
		if params.Email != "" {
			acctParams.Email = gostripe.String(params.Email)
		}
		if params.Country != "" {
			acctParams.Country = gostripe.String(params.Country)
		}
		if params.SellerID != "" {
			acctParams.Metadata = map[string]string{"seller_id": params.SellerID}
		}
		acct, err := api.Accounts.New(acctParams)
		if err != nil {
			return nil, fmt.Errorf("stripe: create connected account: %w", err)
		}
		accountID = acct.ID
	}

	linkParams := &gostripe.AccountLinkParams{
		Account:    gostripe.String(accountID),
		Type:       gostripe.String("account_onboarding"),
		Collect:    gostripe.String("currently_due"),
		RefreshURL: gostripe.String(params.RefreshURL),
		ReturnURL:  gostripe.String(params.ReturnURL),
	}

	link, err := api.AccountLinks.New(linkParams)
	if err != nil {
		return nil, fmt.Errorf("stripe: create account link: %w", err)
	}
	return &contracts.OnboardingResult{
		URL:       link.URL,
		AccountID: accountID,
	}, nil
}

func (p *Provider) HandleOnboardingCallback(_ context.Context, params contracts.CallbackParams) (*contracts.ProviderAccount, error) {
	accountID := params.AccountID
	if accountID == "" {
		return nil, fmt.Errorf("stripe: AccountID is required (the pre-created acct_xxx)")
	}

	api := newAPI(p.config.SecretKey, p.config.BackendURL)

	acct, err := api.Accounts.GetByID(accountID, nil)
	if err != nil {
		return nil, fmt.Errorf("stripe: get account %s: %w", accountID, err)
	}

	status := "pending"
	if acct.ChargesEnabled && acct.PayoutsEnabled {
		status = "active"
	} else if acct.Requirements != nil && len(acct.Requirements.Errors) > 0 {
		status = "restricted"
	}

	return &contracts.ProviderAccount{
		ProviderID: providerID,
		AccountID:  acct.ID,
		Email:      acct.Email,
		Status:     status,
	}, nil
}

func (p *Provider) GetAccountStatus(_ context.Context, accountID string) (*contracts.AccountStatus, error) {
	api := newAPI(p.config.SecretKey, p.config.BackendURL)

	acct, err := api.Accounts.GetByID(accountID, nil)
	if err != nil {
		return nil, fmt.Errorf("stripe: get account status: %w", err)
	}

	status := &contracts.AccountStatus{
		AccountID:      acct.ID,
		ChargesEnabled: acct.ChargesEnabled,
		PayoutsEnabled: acct.PayoutsEnabled,
	}

	if acct.ChargesEnabled && acct.PayoutsEnabled {
		status.IsActive = true
		status.Status = "active"
	} else if acct.Requirements != nil && len(acct.Requirements.Errors) > 0 {
		status.Status = "restricted"
		for _, e := range acct.Requirements.Errors {
			status.Requirements = append(status.Requirements, e.Reason)
		}
	} else {
		status.Status = "pending"
		if acct.Requirements != nil {
			status.Requirements = acct.Requirements.CurrentlyDue
		}
	}

	return status, nil
}

// --- Helpers ---

// WebhookEventType allows passing through unhandled Stripe event types.
type WebhookEventType = contracts.WebhookEventType

func extractPaymentIntent(raw json.RawMessage) (*gostripe.PaymentIntent, error) {
	var pi gostripe.PaymentIntent
	if err := json.Unmarshal(raw, &pi); err != nil {
		return nil, fmt.Errorf("stripe: unmarshal payment intent: %w", err)
	}
	return &pi, nil
}

func extractPaymentMethod(pi *gostripe.PaymentIntent) contracts.PaymentMethodInfo {
	info := contracts.PaymentMethodInfo{Type: "card"}
	if pi.PaymentMethod != nil && pi.PaymentMethod.Card != nil {
		info.Brand = string(pi.PaymentMethod.Card.Brand)
		info.Last4 = pi.PaymentMethod.Card.Last4
	}
	return info
}

func mapStripeStatus(s gostripe.PaymentIntentStatus) string {
	switch s {
	case gostripe.PaymentIntentStatusSucceeded:
		return "succeeded"
	case gostripe.PaymentIntentStatusCanceled:
		return "failed"
	case gostripe.PaymentIntentStatusRequiresPaymentMethod,
		gostripe.PaymentIntentStatusRequiresConfirmation,
		gostripe.PaymentIntentStatusRequiresAction,
		gostripe.PaymentIntentStatusProcessing:
		return "pending"
	default:
		return string(s)
	}
}

// Compile-time interface compliance checks.
var (
	_ contracts.FiatPaymentProvider    = (*Provider)(nil)
	_ contracts.FiatOnboardingProvider = (*Provider)(nil)
)
