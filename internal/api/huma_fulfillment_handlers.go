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

// registerNodeHumaFulfillmentOperations registers bridged fulfillment / supply-chain OpenAPI ops (AH-1.4 Batch 3).
func (g *Gateway) registerNodeHumaFulfillmentOperations(api huma.API) {
	g.registerFulfillmentListProviders(api)
	g.registerFulfillmentConnect(api)
	g.registerFulfillmentDisconnect(api)
	g.registerFulfillmentProviderStatus(api)
	g.registerFulfillmentCatalog(api)
	g.registerFulfillmentCatalogProduct(api)
	g.registerFulfillmentImportProduct(api)
	g.registerFulfillmentSyncedProducts(api)
	g.registerFulfillmentStoreSyncProducts(api)
	g.registerFulfillmentStoreSyncProduct(api)
	g.registerFulfillmentSyncProduct(api)
	g.registerFulfillmentOrderStatus(api)
	g.registerFulfillmentShippingEstimates(api)
	g.registerFulfillmentWebhookPublic(api)
}

func fulfillmentQuerySuffix(q url.Values) string {
	if len(q) == 0 {
		return ""
	}
	return "?" + q.Encode()
}

func (g *Gateway) registerFulfillmentListProviders(api huma.API) {
	huma.Register(api, huma.Operation{
		OperationID: "fulfillment-list-providers",
		Method:      http.MethodGet,
		Path:        "/v1/fulfillment/providers",
		Summary:     "List fulfillment providers",
		Tags:        []string{"fulfillment"},
		Security:    nodeAuthSecurity,
	}, func(ctx context.Context, _ *struct{}) (*nodeDataOutput, error) {
		req := nodeBridgeRequest(ctx, http.MethodGet, "/v1/fulfillment/providers", nil)
		rr := httptest.NewRecorder()
		g.handleGETFulfillmentProviders(rr, req)
		data, err := nodeBridgeSuccessData(rr)
		if err != nil {
			return nil, err
		}
		return &nodeDataOutput{Body: data}, nil
	})
}

func (g *Gateway) registerFulfillmentConnect(api huma.API) {
	type in struct {
		ProviderID string          `path:"providerID" doc:"Fulfillment provider ID."`
		Body       json.RawMessage `json:",omitempty"`
	}
	huma.Register(api, huma.Operation{
		OperationID: "fulfillment-post-connect",
		Method:      http.MethodPost,
		Path:        "/v1/fulfillment/{providerID}/connect",
		Summary:     "Connect a fulfillment provider",
		Tags:        []string{"fulfillment"},
		Security:    nodeAuthSecurity,
	}, func(ctx context.Context, hi *in) (*nodeDataOutput, error) {
		rawURL := "/v1/fulfillment/" + url.PathEscape(hi.ProviderID) + "/connect"
		req := nodeBridgeRequestWithVars(ctx, http.MethodPost, rawURL, bytes.NewReader(hi.Body), map[string]string{"providerID": hi.ProviderID})
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		g.handlePOSTConnectFulfillmentProvider(rr, req)
		data, err := nodeBridgeSuccessData(rr)
		if err != nil {
			return nil, err
		}
		return &nodeDataOutput{Body: data}, nil
	})
}

func (g *Gateway) registerFulfillmentDisconnect(api huma.API) {
	type in struct {
		ProviderID string `path:"providerID" doc:"Fulfillment provider ID."`
	}
	huma.Register(api, huma.Operation{
		OperationID: "fulfillment-delete-disconnect",
		Method:      http.MethodDelete,
		Path:        "/v1/fulfillment/{providerID}/disconnect",
		Summary:     "Disconnect fulfillment provider",
		Tags:        []string{"fulfillment"},
		Security:    nodeAuthSecurity,
	}, func(ctx context.Context, hi *in) (*nodeNoContentOutput, error) {
		rawURL := "/v1/fulfillment/" + url.PathEscape(hi.ProviderID) + "/disconnect"
		req := nodeBridgeRequestWithVars(ctx, http.MethodDelete, rawURL, nil, map[string]string{"providerID": hi.ProviderID})
		rr := httptest.NewRecorder()
		g.handleDELETEDisconnectFulfillmentProvider(rr, req)
		if err := nodeBridgeNoContent(rr); err != nil {
			return nil, err
		}
		return nil, nil
	})
}

func (g *Gateway) registerFulfillmentProviderStatus(api huma.API) {
	type in struct {
		ProviderID string `path:"providerID" doc:"Fulfillment provider ID."`
	}
	huma.Register(api, huma.Operation{
		OperationID: "fulfillment-get-provider-status",
		Method:      http.MethodGet,
		Path:        "/v1/fulfillment/{providerID}/status",
		Summary:     "Fulfillment provider connection status",
		Tags:        []string{"fulfillment"},
		Security:    nodeAuthSecurity,
	}, func(ctx context.Context, hi *in) (*nodeDataOutput, error) {
		rawURL := "/v1/fulfillment/" + url.PathEscape(hi.ProviderID) + "/status"
		req := nodeBridgeRequestWithVars(ctx, http.MethodGet, rawURL, nil, map[string]string{"providerID": hi.ProviderID})
		rr := httptest.NewRecorder()
		g.handleGETFulfillmentProviderStatus(rr, req)
		data, err := nodeBridgeSuccessData(rr)
		if err != nil {
			return nil, err
		}
		return &nodeDataOutput{Body: data}, nil
	})
}

func (g *Gateway) registerFulfillmentCatalog(api huma.API) {
	type in struct {
		ProviderID string `path:"providerID" doc:"Fulfillment provider ID."`
		CategoryID string `query:"categoryId"`
		Search     string `query:"search"`
		Offset     string `query:"offset"`
		Limit      string `query:"limit"`
	}
	huma.Register(api, huma.Operation{
		OperationID: "fulfillment-get-catalog",
		Method:      http.MethodGet,
		Path:        "/v1/fulfillment/{providerID}/catalog",
		Summary:     "Browse provider catalog",
		Tags:        []string{"fulfillment"},
		Security:    nodeAuthSecurity,
	}, func(ctx context.Context, hi *in) (*nodeDataOutput, error) {
		q := url.Values{}
		if hi.CategoryID != "" {
			q.Set("categoryId", hi.CategoryID)
		}
		if hi.Search != "" {
			q.Set("search", hi.Search)
		}
		if hi.Offset != "" {
			q.Set("offset", hi.Offset)
		}
		if hi.Limit != "" {
			q.Set("limit", hi.Limit)
		}
		rawURL := "/v1/fulfillment/" + url.PathEscape(hi.ProviderID) + "/catalog" + fulfillmentQuerySuffix(q)
		req := nodeBridgeRequestWithVars(ctx, http.MethodGet, rawURL, nil, map[string]string{"providerID": hi.ProviderID})
		rr := httptest.NewRecorder()
		g.handleGETFulfillmentCatalog(rr, req)
		data, err := nodeBridgeSuccessData(rr)
		if err != nil {
			return nil, err
		}
		return &nodeDataOutput{Body: data}, nil
	})
}

func (g *Gateway) registerFulfillmentCatalogProduct(api huma.API) {
	type in struct {
		ProviderID string `path:"providerID" doc:"Fulfillment provider ID."`
		ProductID  string `path:"productID" doc:"Catalog product ID."`
	}
	huma.Register(api, huma.Operation{
		OperationID: "fulfillment-get-catalog-product",
		Method:      http.MethodGet,
		Path:        "/v1/fulfillment/{providerID}/catalog/{productID}",
		Summary:     "Get catalog product detail",
		Tags:        []string{"fulfillment"},
		Security:    nodeAuthSecurity,
	}, func(ctx context.Context, hi *in) (*nodeDataOutput, error) {
		rawURL := "/v1/fulfillment/" + url.PathEscape(hi.ProviderID) + "/catalog/" + url.PathEscape(hi.ProductID)
		req := nodeBridgeRequestWithVars(ctx, http.MethodGet, rawURL, nil, map[string]string{
			"providerID": hi.ProviderID,
			"productID":  hi.ProductID,
		})
		rr := httptest.NewRecorder()
		g.handleGETFulfillmentCatalogProduct(rr, req)
		data, err := nodeBridgeSuccessData(rr)
		if err != nil {
			return nil, err
		}
		return &nodeDataOutput{Body: data}, nil
	})
}

func (g *Gateway) registerFulfillmentImportProduct(api huma.API) {
	type in struct {
		ProviderID string          `path:"providerID" doc:"Fulfillment provider ID."`
		Body       json.RawMessage `json:",omitempty"`
	}
	huma.Register(api, huma.Operation{
		OperationID: "fulfillment-post-import-product",
		Method:      http.MethodPost,
		Path:        "/v1/fulfillment/{providerID}/import",
		Summary:     "Import a product from provider catalog",
		Tags:        []string{"fulfillment"},
		Security:    nodeAuthSecurity,
	}, func(ctx context.Context, hi *in) (*nodeDataOutput, error) {
		rawURL := "/v1/fulfillment/" + url.PathEscape(hi.ProviderID) + "/import"
		req := nodeBridgeRequestWithVars(ctx, http.MethodPost, rawURL, bytes.NewReader(hi.Body), map[string]string{"providerID": hi.ProviderID})
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		g.handlePOSTImportFulfillmentProduct(rr, req)
		data, err := nodeBridgeSuccessData(rr)
		if err != nil {
			return nil, err
		}
		return &nodeDataOutput{Body: data}, nil
	})
}

func (g *Gateway) registerFulfillmentSyncedProducts(api huma.API) {
	type in struct {
		ProviderID string `path:"providerID" doc:"Fulfillment provider ID."`
	}
	huma.Register(api, huma.Operation{
		OperationID: "fulfillment-get-synced-products",
		Method:      http.MethodGet,
		Path:        "/v1/fulfillment/{providerID}/synced-products",
		Summary:     "List synced fulfillment products",
		Tags:        []string{"fulfillment"},
		Security:    nodeAuthSecurity,
	}, func(ctx context.Context, hi *in) (*nodeDataOutput, error) {
		rawURL := "/v1/fulfillment/" + url.PathEscape(hi.ProviderID) + "/synced-products"
		req := nodeBridgeRequestWithVars(ctx, http.MethodGet, rawURL, nil, map[string]string{"providerID": hi.ProviderID})
		rr := httptest.NewRecorder()
		g.handleGETSyncedProducts(rr, req)
		data, err := nodeBridgeSuccessData(rr)
		if err != nil {
			return nil, err
		}
		return &nodeDataOutput{Body: data}, nil
	})
}

func (g *Gateway) registerFulfillmentStoreSyncProducts(api huma.API) {
	type in struct {
		ProviderID string `path:"providerID" doc:"Fulfillment provider ID."`
		Offset     string `query:"offset"`
		Limit      string `query:"limit"`
	}
	huma.Register(api, huma.Operation{
		OperationID: "fulfillment-get-store-sync-products",
		Method:      http.MethodGet,
		Path:        "/v1/fulfillment/{providerID}/store-products",
		Summary:     "Browse store sync products",
		Tags:        []string{"fulfillment"},
		Security:    nodeAuthSecurity,
	}, func(ctx context.Context, hi *in) (*nodeDataOutput, error) {
		q := url.Values{}
		if hi.Offset != "" {
			q.Set("offset", hi.Offset)
		}
		if hi.Limit != "" {
			q.Set("limit", hi.Limit)
		}
		rawURL := "/v1/fulfillment/" + url.PathEscape(hi.ProviderID) + "/store-products" + fulfillmentQuerySuffix(q)
		req := nodeBridgeRequestWithVars(ctx, http.MethodGet, rawURL, nil, map[string]string{"providerID": hi.ProviderID})
		rr := httptest.NewRecorder()
		g.handleGETStoreSyncProducts(rr, req)
		data, err := nodeBridgeSuccessData(rr)
		if err != nil {
			return nil, err
		}
		return &nodeDataOutput{Body: data}, nil
	})
}

func (g *Gateway) registerFulfillmentStoreSyncProduct(api huma.API) {
	type in struct {
		ProviderID    string `path:"providerID" doc:"Fulfillment provider ID."`
		SyncProductID string `path:"syncProductID" doc:"Store sync product ID."`
	}
	huma.Register(api, huma.Operation{
		OperationID: "fulfillment-get-store-sync-product",
		Method:      http.MethodGet,
		Path:        "/v1/fulfillment/{providerID}/store-products/{syncProductID}",
		Summary:     "Get a store sync product",
		Tags:        []string{"fulfillment"},
		Security:    nodeAuthSecurity,
	}, func(ctx context.Context, hi *in) (*nodeDataOutput, error) {
		rawURL := "/v1/fulfillment/" + url.PathEscape(hi.ProviderID) + "/store-products/" + url.PathEscape(hi.SyncProductID)
		req := nodeBridgeRequestWithVars(ctx, http.MethodGet, rawURL, nil, map[string]string{
			"providerID":    hi.ProviderID,
			"syncProductID": hi.SyncProductID,
		})
		rr := httptest.NewRecorder()
		g.handleGETStoreSyncProduct(rr, req)
		data, err := nodeBridgeSuccessData(rr)
		if err != nil {
			return nil, err
		}
		return &nodeDataOutput{Body: data}, nil
	})
}

func (g *Gateway) registerFulfillmentSyncProduct(api huma.API) {
	type in struct {
		Slug string `path:"slug" doc:"Listing slug to sync."`
	}
	huma.Register(api, huma.Operation{
		OperationID: "fulfillment-post-sync-product-by-slug",
		Method:      http.MethodPost,
		Path:        "/v1/fulfillment/products/{slug}/sync",
		Summary:     "Push sync updates for a product by slug",
		Tags:        []string{"fulfillment"},
		Security:    nodeAuthSecurity,
	}, func(ctx context.Context, hi *in) (*nodeDataOutput, error) {
		rawURL := "/v1/fulfillment/products/" + url.PathEscape(hi.Slug) + "/sync"
		req := nodeBridgeRequestWithVars(ctx, http.MethodPost, rawURL, nil, map[string]string{"slug": hi.Slug})
		rr := httptest.NewRecorder()
		g.handlePOSTSyncProduct(rr, req)
		data, err := nodeBridgeSuccessData(rr)
		if err != nil {
			return nil, err
		}
		return &nodeDataOutput{Body: data}, nil
	})
}

func (g *Gateway) registerFulfillmentOrderStatus(api huma.API) {
	type in struct {
		OrderID string `path:"orderID" doc:"Order ID."`
	}
	huma.Register(api, huma.Operation{
		OperationID: "fulfillment-get-order-status",
		Method:      http.MethodGet,
		Path:        "/v1/fulfillment/orders/{orderID}/status",
		Summary:     "Get fulfillment execution status",
		Tags:        []string{"fulfillment"},
		Security:    nodeAuthSecurity,
	}, func(ctx context.Context, hi *in) (*nodeDataOutput, error) {
		rawURL := "/v1/fulfillment/orders/" + url.PathEscape(hi.OrderID) + "/status"
		req := nodeBridgeRequestWithVars(ctx, http.MethodGet, rawURL, nil, map[string]string{"orderID": hi.OrderID})
		rr := httptest.NewRecorder()
		g.handleGETFulfillmentOrderStatus(rr, req)
		data, err := nodeBridgeSuccessData(rr)
		if err != nil {
			return nil, err
		}
		return &nodeDataOutput{Body: data}, nil
	})
}

func (g *Gateway) registerFulfillmentShippingEstimates(api huma.API) {
	type in struct {
		ProviderID string          `path:"providerID" doc:"Fulfillment provider ID."`
		Body       json.RawMessage `json:",omitempty"`
	}
	huma.Register(api, huma.Operation{
		OperationID: "fulfillment-post-shipping-estimates",
		Method:      http.MethodPost,
		Path:        "/v1/fulfillment/{providerID}/shipping-estimates",
		Summary:     "Estimate shipping via provider",
		Tags:        []string{"fulfillment"},
		Security:    nodeAuthSecurity,
	}, func(ctx context.Context, hi *in) (*nodeDataOutput, error) {
		rawURL := "/v1/fulfillment/" + url.PathEscape(hi.ProviderID) + "/shipping-estimates"
		req := nodeBridgeRequestWithVars(ctx, http.MethodPost, rawURL, bytes.NewReader(hi.Body), map[string]string{"providerID": hi.ProviderID})
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		g.handlePOSTEstimateShipping(rr, req)
		data, err := nodeBridgeSuccessData(rr)
		if err != nil {
			return nil, err
		}
		return &nodeDataOutput{Body: data}, nil
	})
}

func (g *Gateway) registerFulfillmentWebhookPublic(api huma.API) {
	type in struct {
		ProviderID    string          `path:"providerID" doc:"Fulfillment provider ID."`
		WebhookSecret string          `path:"webhookSecret" doc:"Webhook secret path segment."`
		Body          json.RawMessage `json:",omitempty"`
	}
	huma.Register(api, huma.Operation{
		OperationID: "fulfillment-public-post-provider-webhook",
		Method:      http.MethodPost,
		Path:        "/v1/fulfillment/{providerID}/webhooks/{webhookSecret}",
		Summary:     "Provider fulfillment webhook receiver",
		Tags:        []string{"fulfillment"},
	}, func(ctx context.Context, hi *in) (*nodeNoContentOutput, error) {
		rawURL := "/v1/fulfillment/" + url.PathEscape(hi.ProviderID) + "/webhooks/" + url.PathEscape(hi.WebhookSecret)
		req := nodeBridgeRequestWithVars(ctx, http.MethodPost, rawURL, bytes.NewReader(hi.Body), map[string]string{
			"providerID":    hi.ProviderID,
			"webhookSecret": hi.WebhookSecret,
		})
		rr := httptest.NewRecorder()
		g.handlePOSTFulfillmentWebhook(rr, req)
		if err := nodeBridgeNoContent(rr); err != nil {
			return nil, err
		}
		return nil, nil
	})
}
