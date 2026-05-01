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

// registerNodeHumaShippingOperations registers bridged shipping profile / location OpenAPI ops (AH-1.4 Batch 4).
func (g *Gateway) registerNodeHumaShippingOperations(api huma.API) {
	type jsonBody struct {
		Body json.RawMessage `json:",omitempty"`
	}

	type profileIDPath struct {
		ProfileID string `path:"profileID" doc:"Shipping profile ID."`
	}

	huma.Register(api, huma.Operation{
		OperationID: "shipping-profiles-post",
		Method:      http.MethodPost,
		Path:        "/v1/shipping/profiles",
		Summary:     "Create shipping profile",
		Tags:        []string{"shipping"},
		Security:    nodeAuthSecurity,
	}, func(ctx context.Context, in *jsonBody) (*nodeDataOutput, error) {
		req := nodeBridgeRequest(ctx, http.MethodPost, "/v1/shipping/profiles", bytes.NewReader(in.Body))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		g.handleCreateShippingProfile(rr, req)
		data, err := nodeBridgeSuccessData(rr)
		if err != nil {
			return nil, err
		}
		return &nodeDataOutput{Body: data}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "shipping-profiles-get",
		Method:      http.MethodGet,
		Path:        "/v1/shipping/profiles",
		Summary:     "List shipping profiles",
		Tags:        []string{"shipping"},
		Security:    nodeAuthSecurity,
	}, func(ctx context.Context, _ *struct{}) (*nodeDataOutput, error) {
		req := nodeBridgeRequest(ctx, http.MethodGet, "/v1/shipping/profiles", nil)
		rr := httptest.NewRecorder()
		g.handleListShippingProfiles(rr, req)
		data, err := nodeBridgeSuccessData(rr)
		if err != nil {
			return nil, err
		}
		return &nodeDataOutput{Body: data}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "shipping-profiles-default-post",
		Method:      http.MethodPost,
		Path:        "/v1/shipping/profiles/{profileID}/set-default",
		Summary:     "Set default shipping profile",
		Tags:        []string{"shipping"},
		Security:    nodeAuthSecurity,
	}, func(ctx context.Context, in *profileIDPath) (*nodeNoContentOutput, error) {
		rawURL := "/v1/shipping/profiles/" + url.PathEscape(in.ProfileID) + "/set-default"
		req := nodeBridgeRequestWithVars(ctx, http.MethodPost, rawURL, bytes.NewReader([]byte("{}")), map[string]string{"profileID": in.ProfileID})
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		g.handleSetDefaultShippingProfile(rr, req)
		if err := nodeBridgeNoContent(rr); err != nil {
			return nil, err
		}
		return &nodeNoContentOutput{}, nil
	})

	type listingsQ struct {
		ProfileID string `path:"profileID"`
		Page      string `query:"page"`
		PageSize  string `query:"pageSize"`
	}
	huma.Register(api, huma.Operation{
		OperationID: "shipping-profiles-listings-get",
		Method:      http.MethodGet,
		Path:        "/v1/shipping/profiles/{profileID}/listings",
		Summary:     "Paginated listings using a shipping profile",
		Tags:        []string{"shipping"},
		Security:    nodeAuthSecurity,
	}, func(ctx context.Context, q *listingsQ) (*nodeDataOutput, error) {
		uv := url.Values{}
		if q.Page != "" {
			uv.Set("page", q.Page)
		}
		if q.PageSize != "" {
			uv.Set("pageSize", q.PageSize)
		}
		rawURL := "/v1/shipping/profiles/" + url.PathEscape(q.ProfileID) + "/listings"
		if enc := uv.Encode(); enc != "" {
			rawURL += "?" + enc
		}
		req := nodeBridgeRequestWithVars(ctx, http.MethodGet, rawURL, nil, map[string]string{"profileID": q.ProfileID})
		rr := httptest.NewRecorder()
		g.handleListProfileListings(rr, req)
		data, err := nodeBridgeSuccessData(rr)
		if err != nil {
			return nil, err
		}
		return &nodeDataOutput{Body: data}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "shipping-profiles-id-get",
		Method:      http.MethodGet,
		Path:        "/v1/shipping/profiles/{profileID}",
		Summary:     "Get shipping profile",
		Tags:        []string{"shipping"},
		Security:    nodeAuthSecurity,
	}, func(ctx context.Context, in *profileIDPath) (*nodeDataOutput, error) {
		rawURL := "/v1/shipping/profiles/" + url.PathEscape(in.ProfileID)
		req := nodeBridgeRequestWithVars(ctx, http.MethodGet, rawURL, nil, map[string]string{"profileID": in.ProfileID})
		rr := httptest.NewRecorder()
		g.handleGetShippingProfile(rr, req)
		data, err := nodeBridgeSuccessData(rr)
		if err != nil {
			return nil, err
		}
		return &nodeDataOutput{Body: data}, nil
	})

	type profilePut struct {
		ProfileID string          `path:"profileID"`
		Body      json.RawMessage `json:",omitempty"`
	}
	huma.Register(api, huma.Operation{
		OperationID: "shipping-profiles-id-put",
		Method:      http.MethodPut,
		Path:        "/v1/shipping/profiles/{profileID}",
		Summary:     "Replace shipping profile",
		Tags:        []string{"shipping"},
		Security:    nodeAuthSecurity,
	}, func(ctx context.Context, in *profilePut) (*nodeDataOutput, error) {
		rawURL := "/v1/shipping/profiles/" + url.PathEscape(in.ProfileID)
		req := nodeBridgeRequestWithVars(ctx, http.MethodPut, rawURL, bytes.NewReader(in.Body), map[string]string{"profileID": in.ProfileID})
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		g.handleUpdateShippingProfile(rr, req)
		data, err := nodeBridgeSuccessData(rr)
		if err != nil {
			return nil, err
		}
		return &nodeDataOutput{Body: data}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "shipping-profiles-id-patch",
		Method:      http.MethodPatch,
		Path:        "/v1/shipping/profiles/{profileID}",
		Summary:     "Patch shipping profile",
		Tags:        []string{"shipping"},
		Security:    nodeAuthSecurity,
	}, func(ctx context.Context, in *profilePut) (*nodeDataOutput, error) {
		rawURL := "/v1/shipping/profiles/" + url.PathEscape(in.ProfileID)
		req := nodeBridgeRequestWithVars(ctx, http.MethodPatch, rawURL, bytes.NewReader(in.Body), map[string]string{"profileID": in.ProfileID})
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		g.handlePatchShippingProfile(rr, req)
		data, err := nodeBridgeSuccessData(rr)
		if err != nil {
			return nil, err
		}
		return &nodeDataOutput{Body: data}, nil
	})

	type profileDeleteQ struct {
		ProfileID string `path:"profileID"`
		MigrateTo string `query:"migrateTo"`
	}
	huma.Register(api, huma.Operation{
		OperationID: "shipping-profiles-id-delete",
		Method:      http.MethodDelete,
		Path:        "/v1/shipping/profiles/{profileID}",
		Summary:     "Delete shipping profile",
		Tags:        []string{"shipping"},
		Security:    nodeAuthSecurity,
	}, func(ctx context.Context, in *profileDeleteQ) (*nodeNoContentOutput, error) {
		uv := url.Values{}
		if in.MigrateTo != "" {
			uv.Set("migrateTo", in.MigrateTo)
		}
		rawURL := "/v1/shipping/profiles/" + url.PathEscape(in.ProfileID)
		if enc := uv.Encode(); enc != "" {
			rawURL += "?" + enc
		}
		req := nodeBridgeRequestWithVars(ctx, http.MethodDelete, rawURL, nil, map[string]string{"profileID": in.ProfileID})
		rr := httptest.NewRecorder()
		g.handleDeleteShippingProfile(rr, req)
		if err := nodeBridgeNoContent(rr); err != nil {
			return nil, err
		}
		return &nodeNoContentOutput{}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "shipping-locations-post",
		Method:      http.MethodPost,
		Path:        "/v1/shipping/locations",
		Summary:     "Create shipping location",
		Tags:        []string{"shipping"},
		Security:    nodeAuthSecurity,
	}, func(ctx context.Context, in *jsonBody) (*nodeDataOutput, error) {
		req := nodeBridgeRequest(ctx, http.MethodPost, "/v1/shipping/locations", bytes.NewReader(in.Body))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		g.handleCreateShippingLocation(rr, req)
		data, err := nodeBridgeSuccessData(rr)
		if err != nil {
			return nil, err
		}
		return &nodeDataOutput{Body: data}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "shipping-locations-get",
		Method:      http.MethodGet,
		Path:        "/v1/shipping/locations",
		Summary:     "List shipping locations",
		Tags:        []string{"shipping"},
		Security:    nodeAuthSecurity,
	}, func(ctx context.Context, _ *struct{}) (*nodeDataOutput, error) {
		req := nodeBridgeRequest(ctx, http.MethodGet, "/v1/shipping/locations", nil)
		rr := httptest.NewRecorder()
		g.handleListShippingLocations(rr, req)
		data, err := nodeBridgeSuccessData(rr)
		if err != nil {
			return nil, err
		}
		return &nodeDataOutput{Body: data}, nil
	})

	type locationIDPath struct {
		LocationID string `path:"locationID" doc:"Shipping location ID."`
	}

	huma.Register(api, huma.Operation{
		OperationID: "shipping-locations-id-get",
		Method:      http.MethodGet,
		Path:        "/v1/shipping/locations/{locationID}",
		Summary:     "Get shipping location",
		Tags:        []string{"shipping"},
		Security:    nodeAuthSecurity,
	}, func(ctx context.Context, in *locationIDPath) (*nodeDataOutput, error) {
		rawURL := "/v1/shipping/locations/" + url.PathEscape(in.LocationID)
		req := nodeBridgeRequestWithVars(ctx, http.MethodGet, rawURL, nil, map[string]string{"locationID": in.LocationID})
		rr := httptest.NewRecorder()
		g.handleGetShippingLocation(rr, req)
		data, err := nodeBridgeSuccessData(rr)
		if err != nil {
			return nil, err
		}
		return &nodeDataOutput{Body: data}, nil
	})

	type locationPut struct {
		LocationID string          `path:"locationID"`
		Body       json.RawMessage `json:",omitempty"`
	}
	huma.Register(api, huma.Operation{
		OperationID: "shipping-locations-id-put",
		Method:      http.MethodPut,
		Path:        "/v1/shipping/locations/{locationID}",
		Summary:     "Update shipping location",
		Tags:        []string{"shipping"},
		Security:    nodeAuthSecurity,
	}, func(ctx context.Context, in *locationPut) (*nodeDataOutput, error) {
		rawURL := "/v1/shipping/locations/" + url.PathEscape(in.LocationID)
		req := nodeBridgeRequestWithVars(ctx, http.MethodPut, rawURL, bytes.NewReader(in.Body), map[string]string{"locationID": in.LocationID})
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		g.handleUpdateShippingLocation(rr, req)
		data, err := nodeBridgeSuccessData(rr)
		if err != nil {
			return nil, err
		}
		return &nodeDataOutput{Body: data}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "shipping-locations-id-delete",
		Method:      http.MethodDelete,
		Path:        "/v1/shipping/locations/{locationID}",
		Summary:     "Delete shipping location",
		Tags:        []string{"shipping"},
		Security:    nodeAuthSecurity,
	}, func(ctx context.Context, in *locationIDPath) (*nodeNoContentOutput, error) {
		rawURL := "/v1/shipping/locations/" + url.PathEscape(in.LocationID)
		req := nodeBridgeRequestWithVars(ctx, http.MethodDelete, rawURL, nil, map[string]string{"locationID": in.LocationID})
		rr := httptest.NewRecorder()
		g.handleDeleteShippingLocation(rr, req)
		if err := nodeBridgeNoContent(rr); err != nil {
			return nil, err
		}
		return &nodeNoContentOutput{}, nil
	})

	type staleQ struct {
		Page     string `query:"page"`
		PageSize string `query:"pageSize"`
	}
	huma.Register(api, huma.Operation{
		OperationID: "shipping-stale-listings-get",
		Method:      http.MethodGet,
		Path:        "/v1/shipping/stale-listings",
		Summary:     "List stale shipping snapshots",
		Tags:        []string{"shipping"},
		Security:    nodeAuthSecurity,
	}, func(ctx context.Context, q *staleQ) (*nodeDataOutput, error) {
		uv := url.Values{}
		if q.Page != "" {
			uv.Set("page", q.Page)
		}
		if q.PageSize != "" {
			uv.Set("pageSize", q.PageSize)
		}
		rawURL := "/v1/shipping/stale-listings"
		if enc := uv.Encode(); enc != "" {
			rawURL += "?" + enc
		}
		req := nodeBridgeRequest(ctx, http.MethodGet, rawURL, nil)
		rr := httptest.NewRecorder()
		g.handleListStaleListings(rr, req)
		data, err := nodeBridgeSuccessData(rr)
		if err != nil {
			return nil, err
		}
		return &nodeDataOutput{Body: data}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "shipping-refresh-snapshots-post",
		Method:      http.MethodPost,
		Path:        "/v1/shipping/refresh-snapshots",
		Summary:     "Refresh stale listing shipping snapshots",
		Tags:        []string{"shipping"},
		Security:    nodeAuthSecurity,
	}, func(ctx context.Context, _ *struct{}) (*nodeDataOutput, error) {
		req := nodeBridgeRequest(ctx, http.MethodPost, "/v1/shipping/refresh-snapshots", bytes.NewReader([]byte("{}")))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		g.handleRefreshSnapshots(rr, req)
		data, err := nodeBridgeSuccessData(rr)
		if err != nil {
			return nil, err
		}
		return &nodeDataOutput{Body: data}, nil
	})
}
