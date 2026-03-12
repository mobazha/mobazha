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
	ClientID      string
	ClientSecret  string
	WebhookID     string // PayPal webhook ID for signature verification
	Mode          Mode
	Sandbox       bool
	PartnerID     string // PayPal partner merchant ID (SaaS only)
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

	result := &contracts.PaymentResult{
		PaymentID: resp.ID,
		Status:    mapPayPalStatus(resp.Status),
		PaymentMethod: contracts.PaymentMethodInfo{
			Type:  "paypal",
			Brand: "paypal",
		},
	}

	if len(resp.PurchaseUnits) > 0 {
		pu := resp.PurchaseUnits[0]
		result.Currency = pu.Amount.CurrencyCode
		if v, err := parseAmount(pu.Amount.Value); err == nil {
			result.Amount = v
		}
	}

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
		if v, err := parseAmount(pu.Amount.Value); err == nil {
			detail.Amount = v
		}
		if pu.Payee != nil {
			detail.SellerAccountID = pu.Payee.MerchantID
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

	case "CUSTOMER.DISPUTE.RESOLVED":
		we.Type = contracts.WebhookDisputeResolved

	case "PAYMENT.SALE.REFUNDED", "PAYMENT.CAPTURE.REFUNDED":
		we.Type = contracts.WebhookRefundCreated

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
		if v, err := parseAmount(resp.Amount.Value); err == nil {
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
			we.Coin = "fiat:" + we.Currency
		}
		if v, err := parseAmount(pu.Amount.Value); err == nil {
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

// parseAmount converts a decimal string back to cents.
func parseAmount(s string) (int64, error) {
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0, err
	}
	return int64(math.Round(f * 100)), nil
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
	_ contracts.FiatPaymentProvider   = (*Provider)(nil)
	_ contracts.FiatOnboardingProvider = (*Provider)(nil)
)
