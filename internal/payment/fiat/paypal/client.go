package paypal

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"
)

const (
	sandboxBaseURL    = "https://api-m.sandbox.paypal.com"
	productionBaseURL = "https://api-m.paypal.com"
)

// apiClient is a lightweight PayPal REST API client with automatic OAuth2 token management.
type apiClient struct {
	clientID     string
	clientSecret string
	baseURL      string
	httpClient   *http.Client

	mu          sync.Mutex
	accessToken string
	tokenExpiry time.Time
}

func newAPIClient(clientID, clientSecret string, sandbox bool) *apiClient {
	base := productionBaseURL
	if sandbox {
		base = sandboxBaseURL
	}
	return &apiClient{
		clientID:     clientID,
		clientSecret: clientSecret,
		baseURL:      base,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

func (c *apiClient) getToken(ctx context.Context) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.accessToken != "" && time.Now().Before(c.tokenExpiry) {
		return c.accessToken, nil
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		c.baseURL+"/v1/oauth2/token",
		bytes.NewBufferString("grant_type=client_credentials"))
	if err != nil {
		return "", err
	}
	req.SetBasicAuth(c.clientID, c.clientSecret)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("paypal: token request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return "", fmt.Errorf("paypal: token request failed (%d): %s", resp.StatusCode, body)
	}

	var tokenResp struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return "", fmt.Errorf("paypal: decode token response: %w", err)
	}

	c.accessToken = tokenResp.AccessToken
	// Expire token 60 seconds early to account for clock skew and network latency
	c.tokenExpiry = time.Now().Add(time.Duration(tokenResp.ExpiresIn-60) * time.Second)
	return c.accessToken, nil
}

func (c *apiClient) doJSON(ctx context.Context, method, path string, body, result interface{}) error {
	token, err := c.getToken(ctx)
	if err != nil {
		return err
	}

	var bodyReader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("paypal: marshal request: %w", err)
		}
		bodyReader = bytes.NewReader(data)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, bodyReader)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("paypal: %s %s: %w", method, path, err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 512*1024))
	if err != nil {
		return fmt.Errorf("paypal: read response: %w", err)
	}

	if resp.StatusCode >= 400 {
		return fmt.Errorf("paypal: %s %s returned %d: %s", method, path, resp.StatusCode, respBody)
	}

	if result != nil && len(respBody) > 0 {
		if err := json.Unmarshal(respBody, result); err != nil {
			return fmt.Errorf("paypal: decode response: %w", err)
		}
	}
	return nil
}

// --- PayPal v2 Orders API types ---

type orderRequest struct {
	Intent             string         `json:"intent"`
	PurchaseUnits      []purchaseUnit `json:"purchase_units"`
	ApplicationContext *appContext    `json:"application_context,omitempty"`
}

type purchaseUnit struct {
	ReferenceID string `json:"reference_id,omitempty"`
	Amount      amount `json:"amount"`
	Description string `json:"description,omitempty"`
	CustomID    string `json:"custom_id,omitempty"`
	Payee       *payee `json:"payee,omitempty"`
}

type amount struct {
	CurrencyCode string `json:"currency_code"`
	Value        string `json:"value"`
}

type payee struct {
	MerchantID string `json:"merchant_id,omitempty"`
	EmailAddr  string `json:"email_address,omitempty"`
}

type appContext struct {
	ReturnURL string `json:"return_url,omitempty"`
	CancelURL string `json:"cancel_url,omitempty"`
}

type orderResponse struct {
	ID            string       `json:"id"`
	Status        string       `json:"status"`
	PurchaseUnits []puResponse `json:"purchase_units"`
	Links         []link       `json:"links"`
	CreateTime    string       `json:"create_time"`
}

type puResponse struct {
	ReferenceID string `json:"reference_id"`
	Amount      amount `json:"amount"`
	CustomID    string `json:"custom_id"`
	Payee       *payee `json:"payee"`
	Payments    *struct {
		Captures []captureDetail `json:"captures"`
	} `json:"payments,omitempty"`
}

type captureDetail struct {
	ID     string `json:"id"`
	Status string `json:"status"`
	Amount amount `json:"amount"`
}

type link struct {
	Href   string `json:"href"`
	Rel    string `json:"rel"`
	Method string `json:"method"`
}

// --- Webhook event types ---

type webhookEvent struct {
	ID           string          `json:"id"`
	EventType    string          `json:"event_type"`
	Resource     json.RawMessage `json:"resource"`
	ResourceType string          `json:"resource_type"`
	CreateTime   string          `json:"create_time"`
}

type webhookResource struct {
	ID            string       `json:"id"`
	Status        string       `json:"status"`
	CustomID      string       `json:"custom_id"`
	PurchaseUnits []puResponse `json:"purchase_units"`
}

type disputeResource struct {
	DisputeID      string `json:"dispute_id"`
	Reason         string `json:"reason"`
	Status         string `json:"status"`
	DisputeOutcome *struct {
		OutcomeCode string `json:"outcome_code"`
	} `json:"dispute_outcome,omitempty"`
	DisputedTransactions []struct {
		BuyerTransactionID  string `json:"buyer_transaction_id"`
		SellerTransactionID string `json:"seller_transaction_id"`
		CustomField         string `json:"custom"`
	} `json:"disputed_transactions,omitempty"`
	DisputeAmount amount `json:"dispute_amount"`
}

type refundResource struct {
	ID                     string `json:"id"`
	Status                 string `json:"status"`
	Amount                 amount `json:"amount"`
	SellerPayableBreakdown *struct {
		TotalRefundedAmount amount `json:"total_refunded_amount"`
	} `json:"seller_payable_breakdown,omitempty"`
	Links []link `json:"links"`
}

// --- Partner Referral (PPCP onboarding) ---

type partnerReferralRequest struct {
	TrackingID            string              `json:"tracking_id"`
	Operations            []referralOperation `json:"operations"`
	Products              []string            `json:"products"`
	LegalConsents         []legalConsent      `json:"legal_consents"`
	PartnerConfigOverride *partnerConfig      `json:"partner_config_override,omitempty"`
}

type referralOperation struct {
	Operation                string             `json:"operation"`
	APIIntegrationPreference apiIntegrationPref `json:"api_integration_preference"`
}

type apiIntegrationPref struct {
	RestAPIIntegration restAPIIntegration `json:"rest_api_integration"`
}

type restAPIIntegration struct {
	IntegrationMethod string                 `json:"integration_method"`
	IntegrationType   string                 `json:"integration_type"`
	ThirdPartyDetails *restThirdPartyDetails `json:"third_party_details,omitempty"`
}

type restThirdPartyDetails struct {
	Features     []string `json:"features,omitempty"`
	SignupMode   string   `json:"signup_mode,omitempty"`
	Organization string   `json:"organization,omitempty"`
}

type legalConsent struct {
	Type    string `json:"type"`
	Granted bool   `json:"granted"`
}

type partnerConfig struct {
	ReturnURL string `json:"return_url,omitempty"`
}

type partnerReferralResponse struct {
	Links []link `json:"links"`
}

type partnerReferralDetailsResponse struct {
	SubmitterPayerID string `json:"submitter_payer_id"`
	ReferralData     struct {
		CustomerData struct {
			ReferralUserPayerID struct {
				Value string `json:"value"`
			} `json:"referral_user_payer_id"`
		} `json:"customer_data"`
	} `json:"referral_data"`
}

type merchantIntegration struct {
	MerchantID            string `json:"merchant_id"`
	TrackingID            string `json:"tracking_id"`
	PaymentsReceivable    bool   `json:"payments_receivable"`
	PrimaryEmailConfirmed bool   `json:"primary_email_confirmed"`
}

type merchantTrackingResponse struct {
	MerchantID string `json:"merchant_id"`
	TrackingID string `json:"tracking_id"`
}

// --- Refund API types ---

type refundResponse struct {
	ID     string `json:"id"`
	Status string `json:"status"`
	Amount amount `json:"amount"`
}
