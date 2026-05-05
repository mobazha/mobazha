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
