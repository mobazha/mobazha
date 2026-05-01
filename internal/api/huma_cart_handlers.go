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

// registerNodeHumaCartOperations registers bridged shopping-cart OpenAPI ops (AH-1.4 Batch 3).
func (g *Gateway) registerNodeHumaCartOperations(api huma.API) {
	g.registerCartItemsCount(api)
	g.registerCartList(api)
	g.registerCartClear(api)
	g.registerCartAddItem(api)
	g.registerCartUpdateItem(api)
	g.registerCartRemoveItem(api)
}

func (g *Gateway) registerCartItemsCount(api huma.API) {
	huma.Register(api, huma.Operation{
		OperationID: "carts-get-items-count",
		Method:      http.MethodGet,
		Path:        "/v1/carts/count",
		Summary:     "Cart item count across vendors",
		Tags:        []string{"carts"},
		Security:    nodeAuthSecurity,
	}, func(ctx context.Context, _ *struct{}) (*nodeDataOutput, error) {
		req := nodeBridgeRequest(ctx, http.MethodGet, "/v1/carts/count", nil)
		rr := httptest.NewRecorder()
		g.handleGETCartsItemsCount(rr, req)
		data, err := nodeBridgeSuccessData(rr)
		if err != nil {
			return nil, err
		}
		return &nodeDataOutput{Body: data}, nil
	})
}

func (g *Gateway) registerCartList(api huma.API) {
	huma.Register(api, huma.Operation{
		OperationID: "carts-get",
		Method:      http.MethodGet,
		Path:        "/v1/carts",
		Summary:     "List shopping carts",
		Tags:        []string{"carts"},
		Security:    nodeAuthSecurity,
	}, func(ctx context.Context, _ *struct{}) (*nodeDataOutput, error) {
		req := nodeBridgeRequest(ctx, http.MethodGet, "/v1/carts", nil)
		rr := httptest.NewRecorder()
		g.handleGETCarts(rr, req)
		data, err := nodeBridgeSuccessData(rr)
		if err != nil {
			return nil, err
		}
		return &nodeDataOutput{Body: data}, nil
	})
}

func (g *Gateway) registerCartClear(api huma.API) {
	huma.Register(api, huma.Operation{
		OperationID: "carts-delete-all",
		Method:      http.MethodDelete,
		Path:        "/v1/carts",
		Summary:     "Clear all carts",
		Tags:        []string{"carts"},
		Security:    nodeAuthSecurity,
	}, func(ctx context.Context, _ *struct{}) (*nodeDataOutput, error) {
		req := nodeBridgeRequest(ctx, http.MethodDelete, "/v1/carts", nil)
		rr := httptest.NewRecorder()
		g.handleClearCarts(rr, req)
		data, err := nodeBridgeSuccessData(rr)
		if err != nil {
			return nil, err
		}
		return &nodeDataOutput{Body: data}, nil
	})
}

func (g *Gateway) registerCartAddItem(api huma.API) {
	type in struct {
		PeerID string          `path:"peerID" doc:"Vendor peer ID."`
		Body   json.RawMessage `json:",omitempty"`
	}
	huma.Register(api, huma.Operation{
		OperationID: "carts-post-peer-items",
		Method:      http.MethodPost,
		Path:        "/v1/carts/{peerID}/items",
		Summary:     "Add item to cart for vendor",
		Tags:        []string{"carts"},
		Security:    nodeAuthSecurity,
	}, func(ctx context.Context, hi *in) (*nodeDataOutput, error) {
		rawURL := "/v1/carts/" + url.PathEscape(hi.PeerID) + "/items"
		req := nodeBridgeRequestWithVars(ctx, http.MethodPost, rawURL, bytes.NewReader(hi.Body), map[string]string{"peerID": hi.PeerID})
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		g.handleAddToCart(rr, req)
		data, err := nodeBridgeSuccessData(rr)
		if err != nil {
			return nil, err
		}
		return &nodeDataOutput{Body: data}, nil
	})
}

func (g *Gateway) registerCartUpdateItem(api huma.API) {
	type in struct {
		PeerID string          `path:"peerID" doc:"Vendor peer ID."`
		Body   json.RawMessage `json:",omitempty"`
	}
	huma.Register(api, huma.Operation{
		OperationID: "carts-put-peer-items",
		Method:      http.MethodPut,
		Path:        "/v1/carts/{peerID}/items",
		Summary:     "Update cart item for vendor",
		Tags:        []string{"carts"},
		Security:    nodeAuthSecurity,
	}, func(ctx context.Context, hi *in) (*nodeDataOutput, error) {
		rawURL := "/v1/carts/" + url.PathEscape(hi.PeerID) + "/items"
		req := nodeBridgeRequestWithVars(ctx, http.MethodPut, rawURL, bytes.NewReader(hi.Body), map[string]string{"peerID": hi.PeerID})
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		g.handleAddToCart(rr, req)
		data, err := nodeBridgeSuccessData(rr)
		if err != nil {
			return nil, err
		}
		return &nodeDataOutput{Body: data}, nil
	})
}

func (g *Gateway) registerCartRemoveItem(api huma.API) {
	type in struct {
		PeerID string          `path:"peerID" doc:"Vendor peer ID."`
		Body   json.RawMessage `json:",omitempty"`
	}
	huma.Register(api, huma.Operation{
		OperationID: "carts-delete-peer-items",
		Method:      http.MethodDelete,
		Path:        "/v1/carts/{peerID}/items",
		Summary:     "Remove item from cart",
		Tags:        []string{"carts"},
		Security:    nodeAuthSecurity,
	}, func(ctx context.Context, hi *in) (*nodeDataOutput, error) {
		rawURL := "/v1/carts/" + url.PathEscape(hi.PeerID) + "/items"
		req := nodeBridgeRequestWithVars(ctx, http.MethodDelete, rawURL, bytes.NewReader(hi.Body), map[string]string{"peerID": hi.PeerID})
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		g.handleRemoveCartItem(rr, req)
		data, err := nodeBridgeSuccessData(rr)
		if err != nil {
			return nil, err
		}
		return &nodeDataOutput{Body: data}, nil
	})
}
