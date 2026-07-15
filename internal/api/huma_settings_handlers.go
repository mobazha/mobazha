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

// registerNodeHumaSettingsPublicOperations registers public settings ops
// (storefront branding by peerID — buyer browsing).
func (g *Gateway) registerNodeHumaSettingsPublicOperations(api huma.API) {
	type storefrontPeerPath struct {
		PeerID string `path:"peerID" doc:"Store peer ID."`
	}
	huma.Register(api, huma.Operation{
		OperationID: "settings-storefront-public-get",
		Method:      http.MethodGet,
		Path:        "/v1/settings/storefront/{peerID}",
		Summary:     "Get storefront branding (public)",
		Tags:        []string{"settings"},
	}, func(ctx context.Context, in *storefrontPeerPath) (*nodeDataOutput, error) {
		rawURL := "/v1/settings/storefront/" + url.PathEscape(in.PeerID)
		req := nodeBridgeRequestWithVars(ctx, http.MethodGet, rawURL, nil, map[string]string{"peerID": in.PeerID})
		rr := httptest.NewRecorder()
		g.handleGETStorefrontConfigPublic(rr, req)
		data, err := nodeBridgeSuccessData(rr)
		if err != nil {
			return nil, err
		}
		return &nodeDataOutput{Body: data}, nil
	})
}

// registerNodeHumaSettingsAdminOperations registers authenticated settings ops:
// preferences, wishlists, storefront (seller), guest checkout, features.
func (g *Gateway) registerNodeHumaSettingsAdminOperations(api huma.API) {
	type jsonBody struct {
		Body json.RawMessage `json:",omitempty"`
	}

	huma.Register(api, huma.Operation{
		OperationID: "preferences-put",
		Method:      http.MethodPut,
		Path:        "/v1/preferences",
		Summary:     "Update user preferences",
		Tags:        []string{"settings"},
		Security:    nodeAuthSecurity,
	}, func(ctx context.Context, in *jsonBody) (*nodeDataOutput, error) {
		req := nodeBridgeRequest(ctx, http.MethodPut, "/v1/preferences", bytes.NewReader(in.Body))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		g.handlePutUserPreferences(rr, req)
		data, err := nodeBridgeSuccessData(rr)
		if err != nil {
			return nil, err
		}
		return &nodeDataOutput{Body: data}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "preferences-get",
		Method:      http.MethodGet,
		Path:        "/v1/preferences",
		Summary:     "Get user preferences",
		Tags:        []string{"settings"},
		Security:    nodeAuthSecurity,
	}, func(ctx context.Context, _ *struct{}) (*nodeDataOutput, error) {
		req := nodeBridgeRequest(ctx, http.MethodGet, "/v1/preferences", nil)
		rr := httptest.NewRecorder()
		g.handleGetUserPreferences(rr, req)
		data, err := nodeBridgeSuccessData(rr)
		if err != nil {
			return nil, err
		}
		return &nodeDataOutput{Body: data}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "preferences-currency-post",
		Method:      http.MethodPost,
		Path:        "/v1/preferences/currency",
		Summary:     "Bulk update listing currency preferences",
		Tags:        []string{"settings"},
		Security:    nodeAuthSecurity,
	}, func(ctx context.Context, in *jsonBody) (*nodeDataOutput, error) {
		req := nodeBridgeRequest(ctx, http.MethodPost, "/v1/preferences/currency", bytes.NewReader(in.Body))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		g.handlePOSTBulkUpdateCurrency(rr, req)
		data, err := nodeBridgeSuccessData(rr)
		if err != nil {
			return nil, err
		}
		return &nodeDataOutput{Body: data}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "wishlists-get",
		Method:      http.MethodGet,
		Path:        "/v1/wishlists",
		Summary:     "List wishlist items",
		Tags:        []string{"wishlists"},
		Security:    nodeAuthSecurity,
	}, func(ctx context.Context, _ *struct{}) (*nodeDataOutput, error) {
		req := nodeBridgeRequest(ctx, http.MethodGet, "/v1/wishlists", nil)
		rr := httptest.NewRecorder()
		g.handleGETWishlists(rr, req)
		data, err := nodeBridgeSuccessData(rr)
		if err != nil {
			return nil, err
		}
		return &nodeDataOutput{Body: data}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "wishlists-post",
		Method:      http.MethodPost,
		Path:        "/v1/wishlists",
		Summary:     "Add item to wishlist",
		Tags:        []string{"wishlists"},
		Security:    nodeAuthSecurity,
	}, func(ctx context.Context, in *jsonBody) (*nodeDataOutput, error) {
		req := nodeBridgeRequest(ctx, http.MethodPost, "/v1/wishlists", bytes.NewReader(in.Body))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		g.handlePOSTWishlist(rr, req)
		data, err := nodeBridgeSuccessData(rr)
		if err != nil {
			return nil, err
		}
		return &nodeDataOutput{Body: data}, nil
	})

	type wishlistPeerSlug struct {
		PeerID string `path:"peerID"`
		Slug   string `path:"slug"`
	}
	huma.Register(api, huma.Operation{
		OperationID: "wishlists-peer-slug-delete",
		Method:      http.MethodDelete,
		Path:        "/v1/wishlists/{peerID}/{slug}",
		Summary:     "Remove wishlist item",
		Tags:        []string{"wishlists"},
		Security:    nodeAuthSecurity,
	}, func(ctx context.Context, in *wishlistPeerSlug) (*nodeNoContentOutput, error) {
		rawURL := "/v1/wishlists/" + url.PathEscape(in.PeerID) + "/" + url.PathEscape(in.Slug)
		req := nodeBridgeRequestWithVars(ctx, http.MethodDelete, rawURL, nil, map[string]string{
			"peerID": in.PeerID,
			"slug":   in.Slug,
		})
		rr := httptest.NewRecorder()
		g.handleDELETEWishlist(rr, req)
		if err := nodeBridgeNoContent(rr); err != nil {
			return nil, err
		}
		return &nodeNoContentOutput{}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "settings-storefront-get",
		Method:      http.MethodGet,
		Path:        "/v1/settings/storefront",
		Summary:     "Get storefront branding (seller)",
		Tags:        []string{"settings"},
		Security:    nodeAuthSecurity,
	}, func(ctx context.Context, in *struct {
		Variant string `query:"variant" enum:"draft," doc:"Read the unpublished draft slot instead of the live config"`
	}) (*nodeDataOutput, error) {
		rawURL := "/v1/settings/storefront"
		if in.Variant == "draft" {
			rawURL += "?variant=draft"
		}
		req := nodeBridgeRequest(ctx, http.MethodGet, rawURL, nil)
		rr := httptest.NewRecorder()
		g.handleGETStorefrontConfig(rr, req)
		data, err := nodeBridgeSuccessData(rr)
		if err != nil {
			return nil, err
		}
		return &nodeDataOutput{Body: data}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "settings-storefront-draft-delete",
		Method:      http.MethodDelete,
		Path:        "/v1/settings/storefront/draft",
		Summary:     "Discard storefront branding draft",
		Tags:        []string{"settings"},
		Security:    nodeAuthSecurity,
	}, func(ctx context.Context, _ *struct{}) (*nodeDataOutput, error) {
		req := nodeBridgeRequest(ctx, http.MethodDelete, "/v1/settings/storefront/draft", nil)
		rr := httptest.NewRecorder()
		g.handleDELETEStorefrontDraft(rr, req)
		data, err := nodeBridgeSuccessData(rr)
		if err != nil {
			return nil, err
		}
		return &nodeDataOutput{Body: data}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "settings-storefront-put",
		Method:      http.MethodPut,
		Path:        "/v1/settings/storefront",
		Summary:     "Update storefront branding",
		Tags:        []string{"settings"},
		Security:    nodeAuthSecurity,
	}, func(ctx context.Context, in *jsonBody) (*nodeDataOutput, error) {
		req := nodeBridgeRequest(ctx, http.MethodPut, "/v1/settings/storefront", bytes.NewReader(in.Body))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		g.handlePUTStorefrontConfig(rr, req)
		data, err := nodeBridgeSuccessData(rr)
		if err != nil {
			return nil, err
		}
		return &nodeDataOutput{Body: data}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "settings-guest-checkout-get",
		Method:      http.MethodGet,
		Path:        "/v1/settings/guest-checkout",
		Summary:     "Get guest checkout settings",
		Tags:        []string{"settings"},
	}, func(ctx context.Context, _ *struct{}) (*nodeDataOutput, error) {
		req := nodeBridgeRequest(ctx, http.MethodGet, "/v1/settings/guest-checkout", nil)
		rr := httptest.NewRecorder()
		g.handleGETGuestCheckoutSettings(rr, req)
		data, err := nodeBridgeSuccessData(rr)
		if err != nil {
			return nil, err
		}
		return &nodeDataOutput{Body: data}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "settings-guest-checkout-put",
		Method:      http.MethodPut,
		Path:        "/v1/settings/guest-checkout",
		Summary:     "Update guest checkout settings",
		Tags:        []string{"settings"},
		Security:    nodeAuthSecurity,
	}, func(ctx context.Context, in *jsonBody) (*nodeDataOutput, error) {
		req := nodeBridgeRequest(ctx, http.MethodPut, "/v1/settings/guest-checkout", bytes.NewReader(in.Body))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		g.handlePUTGuestCheckoutSettings(rr, req)
		data, err := nodeBridgeSuccessData(rr)
		if err != nil {
			return nil, err
		}
		return &nodeDataOutput{Body: data}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "settings-payment-policy-get",
		Method:      http.MethodGet,
		Path:        "/v1/settings/payment-policy",
		Summary:     "Get store payment policy",
		Tags:        []string{"settings"},
		Security:    nodeAuthSecurity,
	}, func(ctx context.Context, _ *struct{}) (*nodeDataOutput, error) {
		req := nodeBridgeRequest(ctx, http.MethodGet, "/v1/settings/payment-policy", nil)
		rr := httptest.NewRecorder()
		g.handleGETStorePaymentPolicy(rr, req)
		data, err := nodeBridgeSuccessData(rr)
		if err != nil {
			return nil, err
		}
		return &nodeDataOutput{Body: data}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "settings-payment-policy-put",
		Method:      http.MethodPut,
		Path:        "/v1/settings/payment-policy",
		Summary:     "Update store payment policy",
		Tags:        []string{"settings"},
		Security:    nodeAuthSecurity,
	}, func(ctx context.Context, in *jsonBody) (*nodeDataOutput, error) {
		req := nodeBridgeRequest(ctx, http.MethodPut, "/v1/settings/payment-policy", bytes.NewReader(in.Body))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		g.handlePUTStorePaymentPolicy(rr, req)
		data, err := nodeBridgeSuccessData(rr)
		if err != nil {
			return nil, err
		}
		return &nodeDataOutput{Body: data}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "settings-guest-checkout-readiness-get",
		Method:      http.MethodGet,
		Path:        "/v1/settings/guest-checkout/readiness",
		Summary:     "Get guest checkout UTXO readiness",
		Tags:        []string{"settings"},
		Security:    nodeAuthSecurity,
	}, func(ctx context.Context, _ *struct{}) (*nodeDataOutput, error) {
		req := nodeBridgeRequest(ctx, http.MethodGet, "/v1/settings/guest-checkout/readiness", nil)
		rr := httptest.NewRecorder()
		g.handleGETGuestCheckoutReadiness(rr, req)
		data, err := nodeBridgeSuccessData(rr)
		if err != nil {
			return nil, err
		}
		return &nodeDataOutput{Body: data}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "features-get",
		Method:      http.MethodGet,
		Path:        "/v1/features",
		Summary:     "Get feature flags",
		Tags:        []string{"settings"},
		Security:    nodeAuthSecurity,
	}, func(ctx context.Context, _ *struct{}) (*nodeDataOutput, error) {
		req := nodeBridgeRequest(ctx, http.MethodGet, "/v1/features", nil)
		rr := httptest.NewRecorder()
		g.handleGETFeatures(rr, req)
		data, err := nodeBridgeSuccessData(rr)
		if err != nil {
			return nil, err
		}
		return &nodeDataOutput{Body: data}, nil
	})

	type featureKeyPut struct {
		Key  string          `path:"key" doc:"Feature flag key."`
		Body json.RawMessage `json:",omitempty"`
	}
	huma.Register(api, huma.Operation{
		OperationID: "settings-feature-put",
		Method:      http.MethodPut,
		Path:        "/v1/settings/features/{key}",
		Summary:     "Update a feature flag",
		Tags:        []string{"settings"},
		Security:    nodeAuthSecurity,
	}, func(ctx context.Context, in *featureKeyPut) (*nodeDataOutput, error) {
		rawURL := "/v1/settings/features/" + url.PathEscape(in.Key)
		req := nodeBridgeRequestWithVars(ctx, http.MethodPut, rawURL, bytes.NewReader(in.Body), map[string]string{"key": in.Key})
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		g.handlePUTFeatureSetting(rr, req)
		data, err := nodeBridgeSuccessData(rr)
		if err != nil {
			return nil, err
		}
		return &nodeDataOutput{Body: data}, nil
	})
}
