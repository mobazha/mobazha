package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"

	"github.com/danielgtaylor/huma/v2"
)

// Guest Checkout + PaymentMethods Huma registration functions.
// Build-neutral — extracted from huma_order_handlers.go (OP-1.3 Step 4a)
// so both the full-build order aggregation and the private_distribution aggregation
// can call them.

func (g *Gateway) registerGuestOrdersListAuth(api huma.API) {
	type q struct {
		State    string `query:"state"`
		Page     string `query:"page"`
		PageSize string `query:"pageSize"`
	}
	huma.Register(api, huma.Operation{
		OperationID: "guest-orders-list-auth",
		Method:      http.MethodGet,
		Path:        "/v1/guest/orders",
		Summary:     "Seller-visible guest-checkout orders",
		Tags:        []string{"orders", "guest"},
		Security:    nodeAuthSecurity,
	}, func(ctx context.Context, hq *q) (*nodeDataOutput, error) {
		v := url.Values{}
		if hq.State != "" {
			v.Set("state", hq.State)
		}
		if hq.Page != "" {
			v.Set("page", hq.Page)
		}
		if hq.PageSize != "" {
			v.Set("pageSize", hq.PageSize)
		}
		rawURL := "/v1/guest/orders"
		if qs := v.Encode(); qs != "" {
			rawURL += "?" + qs
		}
		req := nodeBridgeRequest(ctx, http.MethodGet, rawURL, nil)
		rr := httptest.NewRecorder()
		g.handleGETGuestOrders(rr, req)
		data, err := nodeBridgeSuccessData(rr)
		if err != nil {
			return nil, err
		}
		return &nodeDataOutput{Body: data}, nil
	})
}

func (g *Gateway) registerGuestOrderShip(api huma.API) {
	type in struct {
		Token string          `path:"token" doc:"Guest checkout token."`
		Body  json.RawMessage `json:",omitempty"`
	}
	huma.Register(api, huma.Operation{
		OperationID: "guest-orders-ship-token",
		Method:      http.MethodPut,
		Path:        "/v1/guest/orders/{token}/ship",
		Summary:     "Mark guest order shipped with tracking payload",
		Tags:        []string{"orders", "guest"},
		Security:    nodeAuthSecurity,
	}, func(ctx context.Context, hi *in) (*nodeNoContentOutput, error) {
		rawURL := "/v1/guest/orders/" + url.PathEscape(hi.Token) + "/ship"
		req := nodeBridgeRequestWithVars(ctx, http.MethodPut, rawURL, bytes.NewReader(hi.Body), map[string]string{"token": hi.Token})
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		g.handleShipGuestOrder(rr, req)
		if err := nodeBridgeNoContent(rr); err != nil {
			return nil, err
		}
		return nil, nil
	})
}

func (g *Gateway) registerGuestOrderComplete(api huma.API) {
	type in struct {
		Token string `path:"token" doc:"Guest checkout token."`
	}
	huma.Register(api, huma.Operation{
		OperationID: "guest-orders-complete-token",
		Method:      http.MethodPut,
		Path:        "/v1/guest/orders/{token}/complete",
		Summary:     "Manually finalize guest-funded order",
		Tags:        []string{"orders", "guest"},
		Security:    nodeAuthSecurity,
	}, func(ctx context.Context, hi *in) (*nodeNoContentOutput, error) {
		rawURL := "/v1/guest/orders/" + url.PathEscape(hi.Token) + "/complete"
		req := nodeBridgeRequestWithVars(ctx, http.MethodPut, rawURL, nil, map[string]string{"token": hi.Token})
		rr := httptest.NewRecorder()
		g.handleCompleteGuestOrder(rr, req)
		if err := nodeBridgeNoContent(rr); err != nil {
			return nil, err
		}
		return nil, nil
	})
}

func (g *Gateway) registerGuestOrderPostPublic(api huma.API) {
	type in struct {
		Body json.RawMessage `json:",omitempty"`
	}
	huma.Register(api, huma.Operation{
		OperationID:   "guest-orders-post-public",
		Method:        http.MethodPost,
		Path:          "/v1/guest/orders",
		Summary:       "Public guest checkout initiation",
		Tags:          []string{"orders", "guest"},
		DefaultStatus: http.StatusCreated,
	}, func(ctx context.Context, hi *in) (*nodeDataOutput, error) {
		if g.guestOrderLimiter != nil {
			ip := clientIPFromContext(ctx)
			if !g.guestOrderLimiter.allow(ip) {
				return nil, huma.NewError(http.StatusTooManyRequests,
					"Rate limit exceeded. Please try again later.")
			}
		}
		req := nodeBridgeRequest(ctx, http.MethodPost, "/v1/guest/orders", bytes.NewReader(hi.Body))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		g.handlePOSTGuestOrder(rr, req)
		data, err := nodeBridgeRawSuccess(rr)
		if err != nil {
			return nil, err
		}
		return &nodeDataOutput{Body: data}, nil
	})
}

func (g *Gateway) registerGuestOrderQuotePublic(api huma.API) {
	type in struct {
		Body json.RawMessage `json:",omitempty"`
	}
	huma.Register(api, huma.Operation{
		OperationID: "guest-orders-quote-public",
		Method:      http.MethodPost,
		Path:        "/v1/guest/orders/quote",
		Summary:     "Public guest checkout supply preflight",
		Tags:        []string{"orders", "guest"},
	}, func(ctx context.Context, hi *in) (*nodeDataOutput, error) {
		if g.guestOrderLimiter != nil {
			ip := clientIPFromContext(ctx)
			if !g.guestOrderLimiter.allow(ip) {
				return nil, huma.NewError(http.StatusTooManyRequests,
					"Rate limit exceeded. Please try again later.")
			}
		}
		req := nodeBridgeRequest(ctx, http.MethodPost, "/v1/guest/orders/quote", bytes.NewReader(hi.Body))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		g.handlePOSTGuestOrderQuote(rr, req)
		data, err := nodeBridgeSuccessData(rr)
		if err != nil {
			return nil, err
		}
		return &nodeDataOutput{Body: data}, nil
	})
}

func (g *Gateway) registerGuestOrderGetPublic(api huma.API) {
	type in struct {
		Token string `path:"token" doc:"Guest checkout token."`
	}
	huma.Register(api, huma.Operation{
		OperationID: "guest-orders-get-public",
		Method:      http.MethodGet,
		Path:        "/v1/guest/orders/{token}",
		Summary:     "Public guest order tracker",
		Tags:        []string{"orders", "guest"},
	}, func(ctx context.Context, hi *in) (*nodeDataOutput, error) {
		rawURL := "/v1/guest/orders/" + url.PathEscape(hi.Token)
		req := nodeBridgeRequestWithVars(ctx, http.MethodGet, rawURL, nil, map[string]string{"token": hi.Token})
		rr := httptest.NewRecorder()
		g.handleGETGuestOrder(rr, req)
		data, err := nodeBridgeSuccessData(rr)
		if err != nil {
			return nil, err
		}
		return &nodeDataOutput{Body: data}, nil
	})
}

// registerGuestOrderAdminDetail exposes GET /v1/guest/orders/{token}/detail
// for authenticated seller access (includes shipping address ciphertext).
func (g *Gateway) registerGuestOrderAdminDetail(api huma.API) {
	type in struct {
		Token string `path:"token" doc:"Guest checkout token."`
	}
	huma.Register(api, huma.Operation{
		OperationID: "guest-orders-admin-detail",
		Method:      http.MethodGet,
		Path:        "/v1/guest/orders/{token}/detail",
		Summary:     "Admin: full guest order detail including encrypted shipping address",
		Tags:        []string{"orders", "guest"},
		Security:    nodeAuthSecurity,
	}, func(ctx context.Context, hi *in) (*nodeDataOutput, error) {
		rawURL := "/v1/guest/orders/" + url.PathEscape(hi.Token) + "/detail"
		req := nodeBridgeRequestWithVars(ctx, http.MethodGet, rawURL, nil, map[string]string{"token": hi.Token})
		rr := httptest.NewRecorder()
		g.handleGETAdminGuestOrderDetail(rr, req)
		data, err := nodeBridgeSuccessData(rr)
		if err != nil {
			return nil, err
		}
		return &nodeDataOutput{Body: data}, nil
	})
}

// registerPGPKeyGet exposes GET /v1/settings/pgp-key (public — buyers call this
// to retrieve the vendor's public key before encrypting their shipping address).
func (g *Gateway) registerPGPKeyGet(api huma.API) {
	huma.Register(api, huma.Operation{
		OperationID: "settings-pgp-key-get",
		Method:      http.MethodGet,
		Path:        "/v1/settings/pgp-key",
		Summary:     "Get seller PGP public key (public)",
		Tags:        []string{"settings"},
	}, func(ctx context.Context, _ *struct{}) (*nodeDataOutput, error) {
		req := nodeBridgeRequest(ctx, http.MethodGet, "/v1/settings/pgp-key", nil)
		rr := httptest.NewRecorder()
		g.handleGETPGPPublicKey(rr, req)
		data, err := nodeBridgeSuccessData(rr)
		if err != nil {
			return nil, err
		}
		return &nodeDataOutput{Body: data}, nil
	})
}

// registerPGPKeyPut exposes PUT /v1/settings/pgp-key (authenticated seller).
func (g *Gateway) registerPGPKeyPut(api huma.API) {
	type in struct {
		Body json.RawMessage `json:",omitempty"`
	}
	huma.Register(api, huma.Operation{
		OperationID: "settings-pgp-key-put",
		Method:      http.MethodPut,
		Path:        "/v1/settings/pgp-key",
		Summary:     "Set seller PGP public key",
		Tags:        []string{"settings"},
		Security:    adminOnlyAuthSecurity,
	}, func(ctx context.Context, hi *in) (*nodeDataOutput, error) {
		req := nodeBridgeRequest(ctx, http.MethodPut, "/v1/settings/pgp-key", bytes.NewReader(hi.Body))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		g.handlePUTPGPPublicKey(rr, req)
		data, err := nodeBridgeSuccessData(rr)
		if err != nil {
			return nil, err
		}
		return &nodeDataOutput{Body: data}, nil
	})
}

// registerPGPKeyDelete exposes DELETE /v1/settings/pgp-key (authenticated seller).
func (g *Gateway) registerPGPKeyDelete(api huma.API) {
	huma.Register(api, huma.Operation{
		OperationID: "settings-pgp-key-delete",
		Method:      http.MethodDelete,
		Path:        "/v1/settings/pgp-key",
		Summary:     "Remove seller PGP public key",
		Tags:        []string{"settings"},
		Security:    adminOnlyAuthSecurity,
	}, func(ctx context.Context, _ *struct{}) (*nodeNoContentOutput, error) {
		req := nodeBridgeRequest(ctx, http.MethodDelete, "/v1/settings/pgp-key", nil)
		rr := httptest.NewRecorder()
		g.handleDELETEPGPPublicKey(rr, req)
		if err := nodeBridgeNoContent(rr); err != nil {
			return nil, err
		}
		return nil, nil
	})
}

func (g *Gateway) registerPaymentMethodsGet(api huma.API) {
	type in struct {
		PeerID string `path:"peerID" doc:"Storefront seller peer scope."`
	}
	huma.Register(api, huma.Operation{
		OperationID: "payment-methods-get-by-peer-id",
		Method:      http.MethodGet,
		Path:        "/v1/payment-methods/{peerID}",
		Summary:     "Public storefront payment rails summary",
		Tags:        []string{"payments"},
	}, func(ctx context.Context, hi *in) (*nodeDataOutput, error) {
		rawURL := "/v1/payment-methods/" + url.PathEscape(hi.PeerID)
		req := nodeBridgeRequestWithVars(ctx, http.MethodGet, rawURL, nil, map[string]string{"peerID": hi.PeerID})
		rr := httptest.NewRecorder()
		g.handleGETPaymentMethods(rr, req)
		data, err := nodeBridgeSuccessData(rr)
		if err != nil {
			return nil, err
		}
		return &nodeDataOutput{Body: data}, nil
	})
}
