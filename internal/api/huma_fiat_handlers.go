package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"

	"github.com/danielgtaylor/huma/v2"
)

// registerNodeHumaFiatPublicOperations registers public fiat ops accessible
// without authentication (buyer checkout, provider webhooks).
func (g *Gateway) registerNodeHumaFiatPublicOperations(api huma.API) {
	g.registerFiatPublicWebhook(api)
	g.registerFiatPublicListProviders(api)
	g.registerFiatPublicPostPayment(api)
	g.registerFiatPublicCapturePayment(api)
}

// registerNodeHumaFiatAdminOperations registers seller-facing fiat provider
// management and authenticated payment ops.
func (g *Gateway) registerNodeHumaFiatAdminOperations(api huma.API) {
	g.registerFiatSellerListProviders(api)
	g.registerFiatSellerPostPayment(api)
	g.registerFiatSellerGetPayment(api)
	g.registerFiatSellerCapturePayment(api)
	g.registerFiatSellerRefundPayment(api)
	g.registerFiatSellerListProviderActions(api)
	g.registerFiatSellerRetryProviderAction(api)
	g.registerFiatSellerProviderStatus(api)
	g.registerFiatSellerGetProviderConfig(api)
	g.registerFiatSellerPutProviderConfig(api)
	g.registerFiatSellerDeleteProviderConfig(api)
	g.registerFiatSellerSetupWebhook(api)
	g.registerFiatSellerVerifyProvider(api)
}

func (g *Gateway) registerFiatSellerListProviderActions(api huma.API) {
	type in struct {
		Provider string `query:"provider" doc:"Optional fiat provider filter."`
		Action   string `query:"action" doc:"Optional action-kind filter."`
		State    string `query:"state" doc:"Optional durable state filter."`
		Limit    int    `query:"limit" minimum:"0" maximum:"100" doc:"Maximum rows; zero uses the server default."`
	}
	huma.Register(api, huma.Operation{
		OperationID: "fiat-list-provider-actions",
		Method:      http.MethodGet,
		Path:        "/v1/fiat/provider-actions",
		Summary:     "List durable fiat provider actions",
		Tags:        []string{"fiat"},
		Security:    nodeAuthSecurity,
	}, func(ctx context.Context, hi *in) (*nodeDataOutput, error) {
		values := url.Values{}
		if hi.Provider != "" {
			values.Set("provider", hi.Provider)
		}
		if hi.Action != "" {
			values.Set("action", hi.Action)
		}
		if hi.State != "" {
			values.Set("state", hi.State)
		}
		if hi.Limit > 0 {
			values.Set("limit", strconv.Itoa(hi.Limit))
		}
		rawURL := "/v1/fiat/provider-actions"
		if encoded := values.Encode(); encoded != "" {
			rawURL += "?" + encoded
		}
		req := nodeBridgeRequest(ctx, http.MethodGet, rawURL, nil)
		rr := httptest.NewRecorder()
		g.handleGETFiatProviderActions(rr, req)
		data, err := nodeBridgeSuccessData(rr)
		if err != nil {
			return nil, err
		}
		return &nodeDataOutput{Body: data}, nil
	})
}

func (g *Gateway) registerFiatSellerRetryProviderAction(api huma.API) {
	type in struct {
		ActionID string `path:"actionID" doc:"Durable provider action ID."`
	}
	huma.Register(api, huma.Operation{
		OperationID: "fiat-retry-provider-action",
		Method:      http.MethodPost,
		Path:        "/v1/fiat/provider-actions/{actionID}/retry",
		Summary:     "Retry a durable fiat provider action",
		Tags:        []string{"fiat"},
		Security:    nodeAuthSecurity,
	}, func(ctx context.Context, hi *in) (*nodeDataOutput, error) {
		rawURL := "/v1/fiat/provider-actions/" + url.PathEscape(hi.ActionID) + "/retry"
		req := nodeBridgeRequestWithVars(ctx, http.MethodPost, rawURL, nil, map[string]string{"actionID": hi.ActionID})
		rr := httptest.NewRecorder()
		g.handlePOSTFiatProviderActionRetry(rr, req)
		data, err := nodeBridgeSuccessData(rr)
		if err != nil {
			return nil, err
		}
		return &nodeDataOutput{Body: data}, nil
	})
}

func (g *Gateway) registerFiatSellerListProviders(api huma.API) {
	huma.Register(api, huma.Operation{
		OperationID: "fiat-list-enabled-providers",
		Method:      http.MethodGet,
		Path:        "/v1/fiat/providers",
		Summary:     "List enabled fiat providers for seller dashboard",
		Tags:        []string{"fiat"},
		Security:    nodeAuthSecurity,
	}, func(ctx context.Context, _ *struct{}) (*nodeDataOutput, error) {
		req := nodeBridgeRequest(ctx, http.MethodGet, "/v1/fiat/providers", nil)
		rr := httptest.NewRecorder()
		g.handleGETFiatProviders(rr, req)
		data, err := nodeBridgeSuccessData(rr)
		if err != nil {
			return nil, err
		}
		return &nodeDataOutput{Body: data}, nil
	})
}

func (g *Gateway) registerFiatSellerPostPayment(api huma.API) {
	type in struct {
		ProviderID string          `path:"providerID" doc:"Fiat provider ID."`
		Body       json.RawMessage `json:",omitempty"`
	}
	huma.Register(api, huma.Operation{
		OperationID: "fiat-create-payment-session",
		Method:      http.MethodPost,
		Path:        "/v1/fiat/{providerID}/payments",
		Summary:     "Create a fiat checkout session",
		Tags:        []string{"fiat"},
		Security:    nodeAuthSecurity,
	}, func(ctx context.Context, hi *in) (*nodeDataOutput, error) {
		rawURL := "/v1/fiat/" + url.PathEscape(hi.ProviderID) + "/payments"
		req := nodeBridgeRequestWithVars(ctx, http.MethodPost, rawURL, bytes.NewReader(hi.Body), map[string]string{"providerID": hi.ProviderID})
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		g.handlePOSTFiatPayment(rr, req)
		data, err := nodeBridgeSuccessData(rr)
		if err != nil {
			return nil, err
		}
		return &nodeDataOutput{Body: data}, nil
	})
}

func (g *Gateway) registerFiatSellerGetPayment(api huma.API) {
	type in struct {
		ProviderID string `path:"providerID" doc:"Fiat provider ID."`
		PaymentID  string `path:"paymentID" doc:"Provider payment/session ID."`
	}
	huma.Register(api, huma.Operation{
		OperationID: "fiat-get-payment",
		Method:      http.MethodGet,
		Path:        "/v1/fiat/{providerID}/payments/{paymentID}",
		Summary:     "Get fiat payment details",
		Tags:        []string{"fiat"},
		Security:    nodeAuthSecurity,
	}, func(ctx context.Context, hi *in) (*nodeDataOutput, error) {
		rawURL := "/v1/fiat/" + url.PathEscape(hi.ProviderID) + "/payments/" + url.PathEscape(hi.PaymentID)
		req := nodeBridgeRequestWithVars(ctx, http.MethodGet, rawURL, nil, map[string]string{
			"providerID": hi.ProviderID,
			"paymentID":  hi.PaymentID,
		})
		rr := httptest.NewRecorder()
		g.handleGETFiatPayment(rr, req)
		data, err := nodeBridgeSuccessData(rr)
		if err != nil {
			return nil, err
		}
		return &nodeDataOutput{Body: data}, nil
	})
}

func (g *Gateway) registerFiatSellerCapturePayment(api huma.API) {
	type in struct {
		ProviderID string `path:"providerID" doc:"Fiat provider ID."`
		SessionID  string `path:"sessionID" doc:"Authorized session ID to capture."`
	}
	huma.Register(api, huma.Operation{
		OperationID: "fiat-capture-payment",
		Method:      http.MethodPost,
		Path:        "/v1/fiat/{providerID}/payments/{sessionID}/capture",
		Summary:     "Capture a fiat authorization",
		Tags:        []string{"fiat"},
		Security:    nodeAuthSecurity,
	}, func(ctx context.Context, hi *in) (*nodeDataOutput, error) {
		rawURL := "/v1/fiat/" + url.PathEscape(hi.ProviderID) + "/payments/" + url.PathEscape(hi.SessionID) + "/capture"
		req := nodeBridgeRequestWithVars(ctx, http.MethodPost, rawURL, nil, map[string]string{
			"providerID": hi.ProviderID,
			"sessionID":  hi.SessionID,
		})
		rr := httptest.NewRecorder()
		g.handlePOSTFiatCapture(rr, req)
		data, err := nodeBridgeSuccessData(rr)
		if err != nil {
			return nil, err
		}
		return &nodeDataOutput{Body: data}, nil
	})
}

func (g *Gateway) registerFiatSellerRefundPayment(api huma.API) {
	type in struct {
		ProviderID     string          `path:"providerID" doc:"Fiat provider ID."`
		PaymentID      string          `path:"paymentID" doc:"Payment ID to refund."`
		IdempotencyKey string          `header:"Idempotency-Key" required:"true" doc:"Stable key for this refund intent."`
		Body           json.RawMessage `json:",omitempty"`
	}
	huma.Register(api, huma.Operation{
		OperationID: "fiat-refund-payment",
		Method:      http.MethodPost,
		Path:        "/v1/fiat/{providerID}/payments/{paymentID}/refund",
		Summary:     "Refund a fiat payment",
		Tags:        []string{"fiat"},
		Security:    nodeAuthSecurity,
	}, func(ctx context.Context, hi *in) (*nodeDataOutput, error) {
		rawURL := "/v1/fiat/" + url.PathEscape(hi.ProviderID) + "/payments/" + url.PathEscape(hi.PaymentID) + "/refund"
		req := nodeBridgeRequestWithVars(ctx, http.MethodPost, rawURL, bytes.NewReader(hi.Body), map[string]string{
			"providerID": hi.ProviderID,
			"paymentID":  hi.PaymentID,
		})
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Idempotency-Key", hi.IdempotencyKey)
		rr := httptest.NewRecorder()
		g.handlePOSTFiatRefund(rr, req)
		data, err := nodeBridgeSuccessData(rr)
		if err != nil {
			return nil, err
		}
		return &nodeDataOutput{Body: data}, nil
	})
}

func (g *Gateway) registerFiatSellerProviderStatus(api huma.API) {
	type in struct {
		ProviderID string `path:"providerID" doc:"Fiat provider ID."`
	}
	huma.Register(api, huma.Operation{
		OperationID: "fiat-get-provider-connection-status",
		Method:      http.MethodGet,
		Path:        "/v1/fiat/{providerID}/status",
		Summary:     "Seller fiat provider onboarding status",
		Tags:        []string{"fiat"},
		Security:    nodeAuthSecurity,
	}, func(ctx context.Context, hi *in) (*nodeDataOutput, error) {
		rawURL := "/v1/fiat/" + url.PathEscape(hi.ProviderID) + "/status"
		req := nodeBridgeRequestWithVars(ctx, http.MethodGet, rawURL, nil, map[string]string{"providerID": hi.ProviderID})
		rr := httptest.NewRecorder()
		g.handleGETFiatProviderStatus(rr, req)
		data, err := nodeBridgeSuccessData(rr)
		if err != nil {
			return nil, err
		}
		return &nodeDataOutput{Body: data}, nil
	})
}

func (g *Gateway) registerFiatSellerGetProviderConfig(api huma.API) {
	type in struct {
		ProviderID string `path:"providerID" doc:"Fiat provider ID."`
	}
	huma.Register(api, huma.Operation{
		OperationID: "fiat-get-provider-config-view",
		Method:      http.MethodGet,
		Path:        "/v1/fiat/{providerID}/config",
		Summary:     "Load seller-facing provider configuration",
		Tags:        []string{"fiat"},
		Security:    nodeAuthSecurity,
	}, func(ctx context.Context, hi *in) (*nodeDataOutput, error) {
		rawURL := "/v1/fiat/" + url.PathEscape(hi.ProviderID) + "/config"
		req := nodeBridgeRequestWithVars(ctx, http.MethodGet, rawURL, nil, map[string]string{"providerID": hi.ProviderID})
		rr := httptest.NewRecorder()
		g.handleGETFiatProviderConfig(rr, req)
		data, err := nodeBridgeSuccessData(rr)
		if err != nil {
			return nil, err
		}
		return &nodeDataOutput{Body: data}, nil
	})
}

func (g *Gateway) registerFiatSellerPutProviderConfig(api huma.API) {
	type in struct {
		ProviderID string          `path:"providerID" doc:"Fiat provider ID."`
		Body       json.RawMessage `json:",omitempty"`
	}
	huma.Register(api, huma.Operation{
		OperationID: "fiat-save-provider-config",
		Method:      http.MethodPut,
		Path:        "/v1/fiat/{providerID}/config",
		Summary:     "Save fiat provider keys and settings",
		Tags:        []string{"fiat"},
		Security:    nodeAuthSecurity,
	}, func(ctx context.Context, hi *in) (*nodeDataOutput, error) {
		rawURL := "/v1/fiat/" + url.PathEscape(hi.ProviderID) + "/config"
		req := nodeBridgeRequestWithVars(ctx, http.MethodPut, rawURL, bytes.NewReader(hi.Body), map[string]string{"providerID": hi.ProviderID})
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		g.handlePUTFiatProviderConfig(rr, req)
		data, err := nodeBridgeSuccessData(rr)
		if err != nil {
			return nil, err
		}
		return &nodeDataOutput{Body: data}, nil
	})
}

func (g *Gateway) registerFiatSellerDeleteProviderConfig(api huma.API) {
	type in struct {
		ProviderID string `path:"providerID" doc:"Fiat provider ID."`
	}
	huma.Register(api, huma.Operation{
		OperationID: "fiat-disconnect-provider",
		Method:      http.MethodDelete,
		Path:        "/v1/fiat/{providerID}/config",
		Summary:     "Disconnect a fiat provider",
		Tags:        []string{"fiat"},
		Security:    nodeAuthSecurity,
	}, func(ctx context.Context, hi *in) (*nodeNoContentOutput, error) {
		rawURL := "/v1/fiat/" + url.PathEscape(hi.ProviderID) + "/config"
		req := nodeBridgeRequestWithVars(ctx, http.MethodDelete, rawURL, nil, map[string]string{"providerID": hi.ProviderID})
		rr := httptest.NewRecorder()
		g.handleDELETEFiatProviderConfig(rr, req)
		if err := nodeBridgeNoContent(rr); err != nil {
			return nil, err
		}
		return nil, nil
	})
}

func (g *Gateway) registerFiatSellerSetupWebhook(api huma.API) {
	type in struct {
		ProviderID string `path:"providerID" doc:"Fiat provider ID."`
	}
	huma.Register(api, huma.Operation{
		OperationID: "fiat-register-provider-webhook",
		Method:      http.MethodPost,
		Path:        "/v1/fiat/{providerID}/setup-webhook",
		Summary:     "Automatically register provider webhook URLs",
		Tags:        []string{"fiat"},
		Security:    nodeAuthSecurity,
	}, func(ctx context.Context, hi *in) (*nodeDataOutput, error) {
		rawURL := "/v1/fiat/" + url.PathEscape(hi.ProviderID) + "/setup-webhook"
		req := nodeBridgeRequestWithVars(ctx, http.MethodPost, rawURL, nil, map[string]string{"providerID": hi.ProviderID})
		rr := httptest.NewRecorder()
		g.handlePOSTFiatSetupWebhook(rr, req)
		data, err := nodeBridgeSuccessData(rr)
		if err != nil {
			return nil, err
		}
		return &nodeDataOutput{Body: data}, nil
	})
}

func (g *Gateway) registerFiatSellerVerifyProvider(api huma.API) {
	type in struct {
		ProviderID string `path:"providerID" doc:"Fiat provider ID."`
	}
	huma.Register(api, huma.Operation{
		OperationID: "fiat-verify-provider-credentials",
		Method:      http.MethodPost,
		Path:        "/v1/fiat/{providerID}/verify",
		Summary:     "Ping provider credentials",
		Tags:        []string{"fiat"},
		Security:    nodeAuthSecurity,
	}, func(ctx context.Context, hi *in) (*nodeDataOutput, error) {
		rawURL := "/v1/fiat/" + url.PathEscape(hi.ProviderID) + "/verify"
		req := nodeBridgeRequestWithVars(ctx, http.MethodPost, rawURL, nil, map[string]string{"providerID": hi.ProviderID})
		rr := httptest.NewRecorder()
		g.handlePOSTFiatProviderVerify(rr, req)
		data, err := nodeBridgeSuccessData(rr)
		if err != nil {
			return nil, err
		}
		return &nodeDataOutput{Body: data}, nil
	})
}

func (g *Gateway) registerFiatPublicWebhook(api huma.API) {
	type in struct {
		ProviderID string          `path:"providerID" doc:"Fiat provider ID."`
		Body       json.RawMessage `json:",omitempty"`
	}
	huma.Register(api, huma.Operation{
		OperationID: "fiat-public-ingest-provider-webhook",
		Method:      http.MethodPost,
		Path:        "/v1/fiat/{providerID}/webhooks",
		Summary:     "Inbound provider webhook relay",
		Tags:        []string{"fiat"},
	}, func(ctx context.Context, hi *in) (*nodeNoContentOutput, error) {
		rawURL := "/v1/fiat/" + url.PathEscape(hi.ProviderID) + "/webhooks"
		req := nodeBridgeRequestWithVars(ctx, http.MethodPost, rawURL, bytes.NewReader(hi.Body), map[string]string{"providerID": hi.ProviderID})
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		g.handlePOSTFiatWebhook(rr, req)
		if err := nodeBridgeNoContent(rr); err != nil {
			return nil, err
		}
		return nil, nil
	})
}

func (g *Gateway) registerFiatPublicListProviders(api huma.API) {
	type in struct {
		PeerID string `path:"peerID" doc:"Seller peer scope."`
	}
	huma.Register(api, huma.Operation{
		OperationID: "fiat-public-list-providers-by-peer",
		Method:      http.MethodGet,
		Path:        "/v1/fiat/{peerID}/providers",
		Summary:     "Browse enabled fiat providers for a storefront peer",
		Tags:        []string{"fiat"},
	}, func(ctx context.Context, hi *in) (*nodeDataOutput, error) {
		rawURL := "/v1/fiat/" + url.PathEscape(hi.PeerID) + "/providers"
		req := nodeBridgeRequestWithVars(ctx, http.MethodGet, rawURL, nil, map[string]string{"peerID": hi.PeerID})
		rr := httptest.NewRecorder()
		g.handleGETFiatProviders(rr, req)
		data, err := nodeBridgeSuccessData(rr)
		if err != nil {
			return nil, err
		}
		return &nodeDataOutput{Body: data}, nil
	})
}

func (g *Gateway) registerFiatPublicPostPayment(api huma.API) {
	type in struct {
		PeerID     string          `path:"peerID" doc:"Seller peer ID."`
		ProviderID string          `path:"providerID" doc:"Fiat provider ID."`
		Body       json.RawMessage `json:",omitempty"`
	}
	huma.Register(api, huma.Operation{
		OperationID: "fiat-public-create-checkout-session",
		Method:      http.MethodPost,
		Path:        "/v1/fiat/{peerID}/{providerID}/payments",
		Summary:     "Create checkout session scoped to storefront peer",
		Tags:        []string{"fiat"},
	}, func(ctx context.Context, hi *in) (*nodeDataOutput, error) {
		rawURL := "/v1/fiat/" + url.PathEscape(hi.PeerID) + "/" + url.PathEscape(hi.ProviderID) + "/payments"
		req := nodeBridgeRequestWithVars(ctx, http.MethodPost, rawURL, bytes.NewReader(hi.Body), map[string]string{
			"peerID":     hi.PeerID,
			"providerID": hi.ProviderID,
		})
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		g.handlePOSTFiatPayment(rr, req)
		data, err := nodeBridgeSuccessData(rr)
		if err != nil {
			return nil, err
		}
		return &nodeDataOutput{Body: data}, nil
	})
}

func (g *Gateway) registerFiatPublicCapturePayment(api huma.API) {
	type in struct {
		PeerID     string `path:"peerID" doc:"Seller peer ID."`
		ProviderID string `path:"providerID" doc:"Fiat provider ID."`
		SessionID  string `path:"sessionID" doc:"Session ID to capture."`
	}
	huma.Register(api, huma.Operation{
		OperationID: "fiat-public-capture-checkout-session",
		Method:      http.MethodPost,
		Path:        "/v1/fiat/{peerID}/{providerID}/payments/{sessionID}/capture",
		Summary:     "Capture an authorized fiat session via storefront routing",
		Tags:        []string{"fiat"},
	}, func(ctx context.Context, hi *in) (*nodeDataOutput, error) {
		rawURL := "/v1/fiat/" + url.PathEscape(hi.PeerID) + "/" + url.PathEscape(hi.ProviderID) +
			"/payments/" + url.PathEscape(hi.SessionID) + "/capture"
		req := nodeBridgeRequestWithVars(ctx, http.MethodPost, rawURL, nil, map[string]string{
			"peerID":     hi.PeerID,
			"providerID": hi.ProviderID,
			"sessionID":  hi.SessionID,
		})
		rr := httptest.NewRecorder()
		g.handlePOSTFiatCapture(rr, req)
		data, err := nodeBridgeSuccessData(rr)
		if err != nil {
			return nil, err
		}
		return &nodeDataOutput{Body: data}, nil
	})
}
