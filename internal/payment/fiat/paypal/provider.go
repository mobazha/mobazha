package paypal

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/mobazha/mobazha3.0/pkg/contracts"
)

const providerID = "paypal"

// Mode determines how the PayPal provider operates.
type Mode string

const (
	// ModePartner uses platform credentials with payee merchant_id (SaaS / PPCP).
	ModePartner Mode = "partner"
	// ModeDirect uses the seller's own credentials (standalone).
	ModeDirect Mode = "direct"
)

// Config holds PayPal provider configuration.
type Config struct {
	ClientID     string
	ClientSecret string
	WebhookID    string // PayPal webhook ID for signature verification
	Mode         Mode
	Sandbox      bool
	PartnerID    string // PayPal partner merchant ID (SaaS only)
}

// Provider implements contracts.FiatPaymentProvider and contracts.FiatOnboardingProvider
// for PayPal Commerce Platform (PPCP) Partner and Direct modes.
type Provider struct {
	config   Config
	client   *apiClient
	sigCache *signatureCache
}

// NewProvider creates a new PayPal provider with the given configuration.
func NewProvider(cfg Config) *Provider {
	return &Provider{
		config:   cfg,
		client:   newAPIClient(cfg.ClientID, cfg.ClientSecret, cfg.Sandbox),
		sigCache: newSignatureCache(),
	}
}

// OverrideBaseURL replaces the API client's base URL and pre-fills an access
// token so OAuth is skipped. Use only in integration tests.
func (p *Provider) OverrideBaseURL(url string) {
	p.client.baseURL = url
	p.client.accessToken = "test-access-token"
	p.client.tokenExpiry = time.Now().Add(1 * time.Hour)
}

func (p *Provider) ProviderID() string { return providerID }

func (p *Provider) CreatePayment(ctx context.Context, params contracts.CreatePaymentParams) (*contracts.PaymentSession, error) {
	amountStr := formatAmount(params.Amount, params.Currency)

	pu := purchaseUnit{
		ReferenceID: params.OrderID,
		Amount: amount{
			CurrencyCode: params.Currency,
			Value:        amountStr,
		},
		CustomID: params.OrderID,
	}
	if params.Description != "" {
		pu.Description = params.Description
	}

	if p.config.Mode == ModePartner && params.SellerAccountID != "" {
		pu.Payee = &payee{MerchantID: params.SellerAccountID}
	}

	reqBody := orderRequest{
		Intent:        "CAPTURE",
		PurchaseUnits: []purchaseUnit{pu},
	}

	if params.ReturnURL != "" || params.CancelURL != "" {
		reqBody.ApplicationContext = &appContext{
			ReturnURL: params.ReturnURL,
			CancelURL: params.CancelURL,
		}
	}

	var resp orderResponse
	if err := p.client.doJSON(ctx, "POST", "/v2/checkout/orders", reqBody, &resp); err != nil {
		return nil, fmt.Errorf("paypal: create order: %w", err)
	}

	approveURL := ""
	for _, l := range resp.Links {
		if l.Rel == "approve" {
			approveURL = l.Href
			break
		}
	}

	return &contracts.PaymentSession{
		SessionID:   resp.ID,
		CaptureMode: contracts.CaptureManual,
		ExpiresAt:   time.Now().Add(3 * time.Hour),
		Status:      resp.Status,
		PayPal: &contracts.PayPalSessionData{
			OrderID:  resp.ID,
			ClientID: p.config.ClientID,
		},
		ApproveURL: approveURL,
	}, nil
}

func (p *Provider) CapturePayment(ctx context.Context, sessionID string) (*contracts.PaymentResult, error) {
	var resp orderResponse
	if err := p.client.doJSON(ctx, "POST", "/v2/checkout/orders/"+sessionID+"/capture", nil, &resp); err != nil {
		return nil, fmt.Errorf("paypal: capture order: %w", err)
	}

	// Use the Capture ID (not Order ID) as PaymentID so RefundPayment works correctly.
	// PayPal Refund API requires: POST /v2/payments/captures/{captureID}/refund
	paymentID := resp.ID
	result := &contracts.PaymentResult{
		Status: mapPayPalStatus(resp.Status),
		PaymentMethod: contracts.PaymentMethodInfo{
			Type:  "paypal",
			Brand: "paypal",
		},
	}

	if len(resp.PurchaseUnits) > 0 {
		pu := resp.PurchaseUnits[0]
		result.Currency = pu.Amount.CurrencyCode
		if v, err := parseAmount(pu.Amount.Value, pu.Amount.CurrencyCode); err == nil {
			result.Amount = v
		}
		if pu.Payments != nil && len(pu.Payments.Captures) > 0 {
			paymentID = pu.Payments.Captures[0].ID
		}
	}
	result.PaymentID = paymentID

	return result, nil
}

func (p *Provider) GetPayment(ctx context.Context, paymentID string) (*contracts.PaymentDetail, error) {
	var resp orderResponse
	if err := p.client.doJSON(ctx, "GET", "/v2/checkout/orders/"+paymentID, nil, &resp); err != nil {
		return nil, fmt.Errorf("paypal: get order: %w", err)
	}

	detail := &contracts.PaymentDetail{
		PaymentID: resp.ID,
		Status:    mapPayPalStatus(resp.Status),
		PaymentMethod: contracts.PaymentMethodInfo{
			Type:  "paypal",
			Brand: "paypal",
		},
	}

	if t, err := time.Parse(time.RFC3339, resp.CreateTime); err == nil {
		detail.CreatedAt = t
	}

	if len(resp.PurchaseUnits) > 0 {
		pu := resp.PurchaseUnits[0]
		detail.Currency = pu.Amount.CurrencyCode
		if v, err := parseAmount(pu.Amount.Value, pu.Amount.CurrencyCode); err == nil {
			detail.Amount = v
		}
		if pu.Payee != nil {
			detail.SellerAccountID = pu.Payee.MerchantID
		}
		if pu.Payments != nil && len(pu.Payments.Captures) > 0 {
			capture := pu.Payments.Captures[0]
			if capture.ID != "" {
				detail.PaymentID = capture.ID
			}
			if capture.Status != "" {
				detail.Status = mapPayPalStatus(capture.Status)
			}
			if capture.Amount.Value != "" {
				detail.Currency = capture.Amount.CurrencyCode
				if v, err := parseAmount(capture.Amount.Value, capture.Amount.CurrencyCode); err == nil {
					detail.Amount = v
				}
			}
		}
	}

	return detail, nil
}

func (p *Provider) ParseWebhook(ctx context.Context, payload []byte, headers map[string]string) (*contracts.WebhookEvent, error) {
	if err := p.verifyWebhookViaAPI(ctx, payload, headers); err != nil {
		return nil, fmt.Errorf("%w: %v", contracts.ErrWebhookSignature, err)
	}

	var event webhookEvent
	if err := json.Unmarshal(payload, &event); err != nil {
		return nil, fmt.Errorf("paypal: unmarshal webhook: %w", err)
	}

	we := &contracts.WebhookEvent{
		EventID:    event.ID,
		ProviderID: providerID,
		Raw:        &event,
	}

	switch event.EventType {
	case "CHECKOUT.ORDER.COMPLETED", "PAYMENT.CAPTURE.COMPLETED":
		we.Type = contracts.WebhookPaymentSucceeded
		p.extractResourceDetails(event.Resource, we)

	case "PAYMENT.CAPTURE.DENIED", "CHECKOUT.ORDER.DECLINED":
		we.Type = contracts.WebhookPaymentFailed
		p.extractResourceDetails(event.Resource, we)

	case "CUSTOMER.DISPUTE.CREATED":
		we.Type = contracts.WebhookDisputeOpened
		p.extractDisputeDetails(event.Resource, we)

	case "CUSTOMER.DISPUTE.RESOLVED":
		we.Type = contracts.WebhookDisputeResolved
		p.extractDisputeDetails(event.Resource, we)

	case "PAYMENT.SALE.REFUNDED", "PAYMENT.CAPTURE.REFUNDED":
		we.Type = contracts.WebhookRefundCreated
		p.extractRefundDetails(event.Resource, we)

	case "MERCHANT.ONBOARDING.COMPLETED":
		we.Type = contracts.WebhookAccountUpdated

	default:
		we.Type = contracts.WebhookEventType(event.EventType)
	}

	return we, nil
}

func (p *Provider) RefundPayment(ctx context.Context, params contracts.RefundParams) (*contracts.RefundResult, error) {
	// PayPal Capture Refund API: POST /v2/payments/captures/{captureID}/refund
	path := fmt.Sprintf("/v2/payments/captures/%s/refund", params.PaymentID)

	body := make(map[string]interface{})
	if params.Amount != nil {
		body["amount"] = map[string]string{
			"value":         formatAmount(*params.Amount, params.Currency),
			"currency_code": strings.ToUpper(params.Currency),
		}
	}
	if params.Reason != "" {
		body["note_to_payer"] = params.Reason
	}

	var resp refundResponse
	if err := p.client.doJSON(ctx, "POST", path, body, &resp); err != nil {
		errMsg := err.Error()
		if strings.Contains(errMsg, "422") && strings.Contains(errMsg, "CAPTURE_FULLY_REFUNDED") {
			return nil, contracts.ErrAlreadyRefunded
		}
		return nil, fmt.Errorf("paypal refund: %w", err)
	}

	result := &contracts.RefundResult{
		RefundID: resp.ID,
		Status:   mapRefundStatus(resp.Status),
	}
	if resp.Amount.Value != "" {
		if v, err := parseAmount(resp.Amount.Value, resp.Amount.CurrencyCode); err == nil {
			result.Amount = v
		}
		result.Currency = strings.ToUpper(resp.Amount.CurrencyCode)
	}

	return result, nil
}

func mapRefundStatus(status string) string {
	switch status {
	case "COMPLETED":
		return "succeeded"
	case "PENDING":
		return "pending"
	case "CANCELLED":
		return "failed"
	default:
		return "pending"
	}
}

func (p *Provider) CancelPayment(_ context.Context, _ string) error {
	return nil
}

// --- FiatOnboardingProvider (SaaS / PPCP Partner) ---

func (p *Provider) GetOnboardingURL(ctx context.Context, params contracts.OnboardingParams) (*contracts.OnboardingResult, error) {
	reqBody := partnerReferralRequest{
		TrackingID: params.SellerID,
		Operations: []referralOperation{{
			Operation: "API_INTEGRATION",
			APIIntegrationPreference: apiIntegrationPref{
				RestAPIIntegration: restAPIIntegration{
					IntegrationMethod: "PAYPAL",
					IntegrationType:   "THIRD_PARTY",
				},
			},
		}},
		Products: []string{"EXPRESS_CHECKOUT"},
		LegalConsents: []legalConsent{{
			Type:    "SHARE_DATA_CONSENT",
			Granted: true,
		}},
	}

	if params.ReturnURL != "" {
		reqBody.PartnerConfigOverride = &partnerConfig{
			ReturnURL: params.ReturnURL,
		}
	}

	var resp partnerReferralResponse
	if err := p.client.doJSON(ctx, "POST", "/v2/customer/partner-referrals", reqBody, &resp); err != nil {
		return nil, fmt.Errorf("paypal: create partner referral: %w", err)
	}

	for _, l := range resp.Links {
		if l.Rel == "action_url" {
			return &contracts.OnboardingResult{URL: l.Href}, nil
		}
	}
	return nil, fmt.Errorf("paypal: no action_url in partner referral response")
}

func (p *Provider) HandleOnboardingCallback(ctx context.Context, params contracts.CallbackParams) (*contracts.ProviderAccount, error) {
	merchantID := params.MerchantIDPP
	if merchantID == "" {
		return nil, fmt.Errorf("paypal: merchant_id_in_paypal is required")
	}

	var integration merchantIntegration
	path := fmt.Sprintf("/v1/customer/partners/%s/merchant-integrations/%s", p.config.PartnerID, merchantID)
	if err := p.client.doJSON(ctx, "GET", path, nil, &integration); err != nil {
		return nil, fmt.Errorf("paypal: get merchant integration: %w", err)
	}

	status := "pending"
	if integration.PaymentsReceivable && integration.PrimaryEmailConfirmed {
		status = "active"
	}

	return &contracts.ProviderAccount{
		ProviderID: providerID,
		AccountID:  merchantID,
		Status:     status,
	}, nil
}

func (p *Provider) GetAccountStatus(ctx context.Context, accountID string) (*contracts.AccountStatus, error) {
	if p.config.PartnerID == "" {
		return &contracts.AccountStatus{
			AccountID: accountID,
			Status:    "active",
			IsActive:  true,
		}, nil
	}

	var integration merchantIntegration
	path := fmt.Sprintf("/v1/customer/partners/%s/merchant-integrations/%s", p.config.PartnerID, accountID)
	if err := p.client.doJSON(ctx, "GET", path, nil, &integration); err != nil {
		return nil, fmt.Errorf("paypal: get merchant status: %w", err)
	}

	active := integration.PaymentsReceivable && integration.PrimaryEmailConfirmed
	status := "pending"
	if active {
		status = "active"
	}

	return &contracts.AccountStatus{
		AccountID:      accountID,
		IsActive:       active,
		Status:         status,
		ChargesEnabled: integration.PaymentsReceivable,
		PayoutsEnabled: integration.PaymentsReceivable,
	}, nil
}

// --- Helpers ---

func (p *Provider) extractDisputeDetails(raw json.RawMessage, we *contracts.WebhookEvent) {
	var res disputeResource
	if err := json.Unmarshal(raw, &res); err != nil {
		return
	}

	we.DisputeID = res.DisputeID
	we.DisputeReason = res.Reason

	if res.DisputeOutcome != nil {
		switch res.DisputeOutcome.OutcomeCode {
		case "RESOLVED_BUYER_FAVOUR":
			we.DisputeOutcome = "lost"
		case "RESOLVED_SELLER_FAVOUR":
			we.DisputeOutcome = "won"
		default:
			we.DisputeOutcome = res.DisputeOutcome.OutcomeCode
		}
	}

	if len(res.DisputedTransactions) > 0 {
		tx := res.DisputedTransactions[0]
		we.PaymentID = tx.SellerTransactionID
		if we.PaymentID == "" {
			we.PaymentID = tx.BuyerTransactionID
		}
		we.OrderID = tx.CustomField
	}

	if res.DisputeAmount.CurrencyCode != "" {
		we.Currency = strings.ToUpper(res.DisputeAmount.CurrencyCode)
		we.Coin = "fiat:" + providerID + ":" + we.Currency
	}
	if v, err := parseAmount(res.DisputeAmount.Value, res.DisputeAmount.CurrencyCode); err == nil {
		we.Amount = v
	}
}

func (p *Provider) extractRefundDetails(raw json.RawMessage, we *contracts.WebhookEvent) {
	var res refundResource
	if err := json.Unmarshal(raw, &res); err != nil {
		return
	}

	we.RefundID = res.ID

	// Extract the capture ID from the "up" link (parent capture)
	for _, l := range res.Links {
		if l.Rel == "up" && strings.Contains(l.Href, "/captures/") {
			parts := strings.Split(l.Href, "/captures/")
			if len(parts) == 2 {
				we.PaymentID = parts[1]
			}
		}
	}

	if res.Amount.CurrencyCode != "" {
		we.Currency = strings.ToUpper(res.Amount.CurrencyCode)
		we.Coin = "fiat:" + providerID + ":" + we.Currency
	}
	if v, err := parseAmount(res.Amount.Value, res.Amount.CurrencyCode); err == nil {
		we.Amount = v
	}
}

func (p *Provider) extractResourceDetails(raw json.RawMessage, we *contracts.WebhookEvent) {
	var res webhookResource
	if err := json.Unmarshal(raw, &res); err != nil {
		return
	}

	we.PaymentID = res.ID
	we.OrderID = res.CustomID

	if len(res.PurchaseUnits) > 0 {
		pu := res.PurchaseUnits[0]
		if we.OrderID == "" {
			we.OrderID = pu.CustomID
		}
		if pu.Payee != nil {
			we.AccountID = pu.Payee.MerchantID
		}
		if pu.Amount.CurrencyCode != "" {
			we.Currency = strings.ToUpper(pu.Amount.CurrencyCode)
			we.Coin = "fiat:" + providerID + ":" + we.Currency
		}
		if v, err := parseAmount(pu.Amount.Value, pu.Amount.CurrencyCode); err == nil {
			we.Amount = v
		}
	}

	we.PaymentMethod = contracts.PaymentMethodInfo{
		Type:  "paypal",
		Brand: "paypal",
	}
}

func getHeader(headers map[string]string, key string) string {
	if v, ok := headers[key]; ok {
		return v
	}
	// Try canonical HTTP header format
	for k, v := range headers {
		if len(k) == len(key) {
			match := true
			for i := range k {
				a, b := k[i], key[i]
				if a >= 'A' && a <= 'Z' {
					a += 'a' - 'A'
				}
				if b >= 'A' && b <= 'Z' {
					b += 'a' - 'A'
				}
				if a != b {
					match = false
					break
				}
			}
			if match {
				return v
			}
		}
	}
	return ""
}

// formatAmount converts cents to a decimal string (e.g. 2999 → "29.99").
// PayPal requires amounts as decimal strings, not integer cents.
func formatAmount(cents int64, currency string) string {
	// Zero-decimal currencies (JPY, KRW, etc.)
	switch currency {
	case "JPY", "KRW", "VND", "HUF", "TWD":
		return strconv.FormatInt(cents, 10)
	default:
		return fmt.Sprintf("%.2f", float64(cents)/100.0)
	}
}

// parseAmount converts a PayPal amount string to the smallest currency unit (cents).
// Zero-decimal currencies (JPY, KRW, etc.) are returned as-is.
func parseAmount(s string, currency string) (int64, error) {
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0, err
	}
	switch strings.ToUpper(currency) {
	case "JPY", "KRW", "VND", "HUF", "TWD":
		return int64(math.Round(f)), nil
	default:
		return int64(math.Round(f * 100)), nil
	}
}

func mapPayPalStatus(s string) string {
	switch s {
	case "COMPLETED":
		return "succeeded"
	case "DECLINED", "VOIDED":
		return "failed"
	case "CREATED", "SAVED", "APPROVED", "PAYER_ACTION_REQUIRED":
		return "pending"
	default:
		return s
	}
}

// Compile-time interface compliance checks.
var (
	_ contracts.FiatPaymentProvider    = (*Provider)(nil)
	_ contracts.FiatOnboardingProvider = (*Provider)(nil)
)
