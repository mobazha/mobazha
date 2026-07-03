package stripe

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	gostripe "github.com/stripe/stripe-go/v82"
	"github.com/stripe/stripe-go/v82/client"
	"github.com/stripe/stripe-go/v82/webhook"

	"github.com/mobazha/mobazha/pkg/contracts"
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

func (p *Provider) CreatePayment(_ context.Context, params contracts.CreatePaymentParams) (*contracts.FiatProviderSession, error) {
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

	sd := &contracts.StripeSessionData{
		ClientSecret:   pi.ClientSecret,
		PublishableKey: p.config.PublishableKey,
	}
	if p.config.Mode == ModeConnected && params.SellerAccountID != "" {
		sd.ConnectedAccountID = params.SellerAccountID
	}

	return &contracts.FiatProviderSession{
		SessionID:   pi.ID,
		CaptureMode: contracts.CaptureAutomatic,
		ExpiresAt:   time.Now().Add(30 * time.Minute),
		Status:      string(pi.Status),
		Stripe:      sd,
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

	event, err := webhook.ConstructEventWithOptions(payload, sig, p.config.WebhookSecret, webhook.ConstructEventOptions{
		IgnoreAPIVersionMismatch: true,
	})
	if err != nil {
		return nil, fmt.Errorf("%w: %v", contracts.ErrWebhookSignature, err)
	}

	we := &contracts.WebhookEvent{
		EventID:    event.ID,
		ProviderID: providerID,
		Raw:        &event,
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
		we.Amount = pi.Amount
		we.Currency = strings.ToUpper(string(pi.Currency))
		we.Coin = "fiat:" + providerID + ":" + we.Currency
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
		we.Amount = pi.Amount
		we.Currency = strings.ToUpper(string(pi.Currency))
		we.Coin = "fiat:" + providerID + ":" + we.Currency

	case "charge.dispute.created":
		we.Type = contracts.WebhookDisputeOpened
		if d, err := extractDispute(event.Data.Raw); err == nil {
			we.DisputeID = d.ID
			we.DisputeReason = string(d.Reason)
			if d.PaymentIntent != nil {
				we.PaymentID = d.PaymentIntent.ID
				we.OrderID = d.PaymentIntent.Metadata["order_id"]
			}
		}

	case "charge.dispute.closed":
		we.Type = contracts.WebhookDisputeResolved
		if d, err := extractDispute(event.Data.Raw); err == nil {
			we.DisputeID = d.ID
			we.DisputeReason = string(d.Reason)
			we.DisputeOutcome = mapDisputeStatus(d.Status)
			if d.PaymentIntent != nil {
				we.PaymentID = d.PaymentIntent.ID
				we.OrderID = d.PaymentIntent.Metadata["order_id"]
			}
		}

	case "charge.refunded":
		we.Type = contracts.WebhookRefundCreated
		if ch, err := extractCharge(event.Data.Raw); err == nil {
			if ch.PaymentIntent != nil {
				we.PaymentID = ch.PaymentIntent.ID
				we.OrderID = ch.PaymentIntent.Metadata["order_id"]
			}
			we.Amount = ch.AmountRefunded
			we.Currency = strings.ToUpper(string(ch.Currency))
			we.Coin = "fiat:" + providerID + ":" + we.Currency
			if ch.Refunds != nil && len(ch.Refunds.Data) > 0 {
				we.RefundID = ch.Refunds.Data[0].ID
			}
		}

	case "payment_intent.canceled":
		we.Type = contracts.WebhookPaymentCanceled
		pi, err := extractPaymentIntent(event.Data.Raw)
		if err != nil {
			return nil, err
		}
		we.PaymentID = pi.ID
		we.OrderID = pi.Metadata["order_id"]
		we.Amount = pi.Amount
		we.Currency = strings.ToUpper(string(pi.Currency))
		we.Coin = "fiat:" + providerID + ":" + we.Currency

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

func (p *Provider) RefundPayment(_ context.Context, params contracts.RefundParams) (*contracts.RefundResult, error) {
	api := newAPI(p.config.SecretKey, p.config.BackendURL)

	rp := &gostripe.RefundParams{
		PaymentIntent: gostripe.String(params.PaymentID),
	}

	if params.Amount != nil {
		rp.Amount = params.Amount
	}

	if params.Reason != "" {
		rp.Reason = gostripe.String(mapRefundReason(params.Reason))
	}

	if params.Metadata != nil {
		rp.Metadata = params.Metadata
	}

	if p.config.Mode == ModeConnected {
		if ra := p.connectedAccountFromMetadata(params.Metadata); ra != "" {
			rp.SetStripeAccount(ra)
		}
	}

	refund, err := api.Refunds.New(rp)
	if err != nil {
		return nil, fmt.Errorf("stripe refund: %w", err)
	}

	return &contracts.RefundResult{
		RefundID: refund.ID,
		Status:   string(refund.Status),
		Amount:   refund.Amount,
		Currency: string(refund.Currency),
	}, nil
}

func (p *Provider) CancelPayment(_ context.Context, paymentID string) error {
	api := newAPI(p.config.SecretKey, p.config.BackendURL)
	params := &gostripe.PaymentIntentCancelParams{}
	_, err := api.PaymentIntents.Cancel(paymentID, params)
	if err != nil {
		return fmt.Errorf("stripe cancel payment intent %s: %w", paymentID, err)
	}
	return nil
}

func (p *Provider) connectedAccountFromMetadata(meta map[string]string) string {
	if meta == nil {
		return ""
	}
	return meta["connectedAccountID"]
}

func mapRefundReason(reason string) string {
	switch reason {
	case "duplicate":
		return string(gostripe.RefundReasonDuplicate)
	case "fraudulent":
		return string(gostripe.RefundReasonFraudulent)
	default:
		return string(gostripe.RefundReasonRequestedByCustomer)
	}
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
		Email:          acct.Email,
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

func extractDispute(raw json.RawMessage) (*gostripe.Dispute, error) {
	var d gostripe.Dispute
	if err := json.Unmarshal(raw, &d); err != nil {
		return nil, fmt.Errorf("stripe: unmarshal dispute: %w", err)
	}
	return &d, nil
}

func extractCharge(raw json.RawMessage) (*gostripe.Charge, error) {
	var ch gostripe.Charge
	if err := json.Unmarshal(raw, &ch); err != nil {
		return nil, fmt.Errorf("stripe: unmarshal charge: %w", err)
	}
	return &ch, nil
}

func mapDisputeStatus(s gostripe.DisputeStatus) string {
	switch s {
	case gostripe.DisputeStatusLost:
		return "lost"
	case gostripe.DisputeStatusWon:
		return "won"
	default:
		return string(s)
	}
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

// requiredWebhookEvents lists all Stripe event types the platform needs to receive.
var requiredWebhookEvents = []string{
	"payment_intent.succeeded",
	"payment_intent.payment_failed",
	"payment_intent.canceled",
	"charge.refunded",
	"charge.dispute.created",
	"charge.dispute.closed",
	"account.updated",
}

// SetupWebhook creates or updates a Stripe webhook endpoint for the given URL.
// If a webhook with the same URL already exists, it updates the enabled events
// and returns the existing endpoint (note: Stripe does not return the signing secret
// for existing endpoints, so a new endpoint may need to be created).
func (p *Provider) SetupWebhook(ctx context.Context, webhookURL string) (*contracts.WebhookSetupResult, error) {
	api := newAPI(p.config.SecretKey, p.config.BackendURL)

	existing, err := p.findExistingWebhook(api, webhookURL)
	if err != nil {
		return nil, fmt.Errorf("stripe: list webhook endpoints: %w", err)
	}

	if existing != nil {
		updateParams := &gostripe.WebhookEndpointParams{
			EnabledEvents: gostripe.StringSlice(requiredWebhookEvents),
		}
		if _, err := api.WebhookEndpoints.Update(existing.ID, updateParams); err != nil {
			return nil, fmt.Errorf("stripe: update webhook endpoint: %w", err)
		}
		return &contracts.WebhookSetupResult{
			WebhookID:     existing.ID,
			WebhookSecret: existing.Secret,
		}, nil
	}

	params := &gostripe.WebhookEndpointParams{
		URL:           gostripe.String(webhookURL),
		EnabledEvents: gostripe.StringSlice(requiredWebhookEvents),
	}
	endpoint, err := api.WebhookEndpoints.New(params)
	if err != nil {
		return nil, fmt.Errorf("stripe: create webhook endpoint: %w", err)
	}

	return &contracts.WebhookSetupResult{
		WebhookID:     endpoint.ID,
		WebhookSecret: endpoint.Secret,
	}, nil
}

// CleanupWebhook removes the Stripe webhook endpoint matching the given URL.
func (p *Provider) CleanupWebhook(ctx context.Context, webhookURL string) error {
	api := newAPI(p.config.SecretKey, p.config.BackendURL)

	existing, err := p.findExistingWebhook(api, webhookURL)
	if err != nil {
		return nil
	}
	if existing == nil {
		return nil
	}

	if _, err := api.WebhookEndpoints.Del(existing.ID, nil); err != nil {
		return fmt.Errorf("stripe: delete webhook endpoint %s: %w", existing.ID, err)
	}
	return nil
}

func (p *Provider) findExistingWebhook(api *client.API, webhookURL string) (*gostripe.WebhookEndpoint, error) {
	params := &gostripe.WebhookEndpointListParams{}
	params.Filters.AddFilter("limit", "", "100")
	iter := api.WebhookEndpoints.List(params)
	for iter.Next() {
		ep := iter.WebhookEndpoint()
		if ep.URL == webhookURL {
			return ep, nil
		}
	}
	return nil, iter.Err()
}

// Compile-time interface compliance checks.
var (
	_ contracts.FiatPaymentProvider    = (*Provider)(nil)
	_ contracts.FiatOnboardingProvider = (*Provider)(nil)
	_ contracts.FiatWebhookConfigurator = (*Provider)(nil)
)
