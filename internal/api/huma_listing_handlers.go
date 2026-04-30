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

// registerNodeHumaListingOperations registers bridged listing OpenAPI ops (AH-1.4 Batch 2).
func (g *Gateway) registerNodeHumaListingOperations(api huma.API) {
	g.registerListingGetMineSlugOrCID(api)
	g.registerListingCreate(api)
	g.registerListingUpdate(api)
	g.registerListingDelete(api)
	g.registerListingImport(api)
	g.registerListingImportJSON(api)

	g.registerListingIndexByPeer(api)
	g.registerListingIndex(api)
	g.registerListingTemplate(api)
	g.registerListingGetByPeerSlug(api)
	g.registerListingGetByListingID(api)
}

// --- Auth ---

func (g *Gateway) registerListingGetMineSlugOrCID(api huma.API) {
	type listingMineInput struct {
		SlugOrCID string `path:"slugOrCID" doc:"Seller listing slug or CID."`
	}
	huma.Register(api, huma.Operation{
		OperationID: "listings-get-mine-slug-or-cid",
		Method:      http.MethodGet,
		Path:        "/v1/listings/mine/{slugOrCID}",
		Summary:     "Get authenticated seller listing by slug or CID",
		Tags:        []string{"listings"},
		Security:    nodeAuthSecurity,
	}, func(ctx context.Context, in *listingMineInput) (*nodeDataOutput, error) {
		rawURL := "/v1/listings/mine/" + url.PathEscape(in.SlugOrCID)
		req := nodeBridgeRequestWithVars(ctx, http.MethodGet, rawURL, nil, map[string]string{"slugOrCID": in.SlugOrCID})
		rr := httptest.NewRecorder()
		g.handleGETMyListing(rr, req)
		data, err := nodeBridgeSuccessData(rr)
		if err != nil {
			return nil, err
		}
		return &nodeDataOutput{Body: data}, nil
	})
}

func (g *Gateway) registerListingCreate(api huma.API) {
	type listingBodyInput struct {
		Body json.RawMessage
	}
	huma.Register(api, huma.Operation{
		OperationID: "listings-create",
		Method:      http.MethodPost,
		Path:        "/v1/listings",
		Summary:     "Create listing",
		Tags:        []string{"listings"},
		Security:    nodeAuthSecurity,
	}, func(ctx context.Context, in *listingBodyInput) (*nodeDataOutput, error) {
		req := nodeBridgeRequest(ctx, http.MethodPost, "/v1/listings", bytes.NewReader(in.Body))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		g.handlePOSTListing(rr, req)
		data, err := nodeBridgeSuccessData(rr)
		if err != nil {
			return nil, err
		}
		return &nodeDataOutput{Body: data}, nil
	})
}

func (g *Gateway) registerListingUpdate(api huma.API) {
	type listingBodyInput struct {
		Body json.RawMessage
	}
	huma.Register(api, huma.Operation{
		OperationID: "listings-update",
		Method:      http.MethodPut,
		Path:        "/v1/listings",
		Summary:     "Update listing",
		Tags:        []string{"listings"},
		Security:    nodeAuthSecurity,
	}, func(ctx context.Context, in *listingBodyInput) (*nodeDataOutput, error) {
		req := nodeBridgeRequest(ctx, http.MethodPut, "/v1/listings", bytes.NewReader(in.Body))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		g.handlePUTListing(rr, req)
		data, err := nodeBridgeSuccessData(rr)
		if err != nil {
			return nil, err
		}
		return &nodeDataOutput{Body: data}, nil
	})
}

func (g *Gateway) registerListingDelete(api huma.API) {
	type listingDeleteInput struct {
		Slug string `path:"slug" doc:"Listing slug to delete."`
	}
	huma.Register(api, huma.Operation{
		OperationID: "listings-delete",
		Method:      http.MethodDelete,
		Path:        "/v1/listings/{slug}",
		Summary:     "Delete listing by slug",
		Tags:        []string{"listings"},
		Security:    nodeAuthSecurity,
	}, func(ctx context.Context, in *listingDeleteInput) (*nodeDataOutput, error) {
		rawURL := "/v1/listings/" + url.PathEscape(in.Slug)
		req := nodeBridgeRequestWithVars(ctx, http.MethodDelete, rawURL, nil, map[string]string{"slug": in.Slug})
		rr := httptest.NewRecorder()
		g.handleDELETEListing(rr, req)
		data, err := nodeBridgeSuccessData(rr)
		if err != nil {
			return nil, err
		}
		return &nodeDataOutput{Body: data}, nil
	})
}

func (g *Gateway) registerListingImport(api huma.API) {
	type listingBodyInput struct {
		Body json.RawMessage
	}
	huma.Register(api, huma.Operation{
		OperationID: "listings-import",
		Method:      http.MethodPost,
		Path:        "/v1/listings/import",
		Summary:     "Batch import listings",
		Tags:        []string{"listings"},
		Security:    nodeAuthSecurity,
	}, func(ctx context.Context, in *listingBodyInput) (*nodeDataOutput, error) {
		req := nodeBridgeRequest(ctx, http.MethodPost, "/v1/listings/import", bytes.NewReader(in.Body))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		g.handlePOSTListingsImport(rr, req)
		data, err := nodeBridgeSuccessData(rr)
		if err != nil {
			return nil, err
		}
		return &nodeDataOutput{Body: data}, nil
	})
}

func (g *Gateway) registerListingImportJSON(api huma.API) {
	type listingBodyInput struct {
		Body json.RawMessage
	}
	huma.Register(api, huma.Operation{
		OperationID: "listings-import-json",
		Method:      http.MethodPost,
		Path:        "/v1/listings/import/json",
		Summary:     "Batch import listings from JSON payload",
		Tags:        []string{"listings"},
		Security:    nodeAuthSecurity,
	}, func(ctx context.Context, in *listingBodyInput) (*nodeDataOutput, error) {
		req := nodeBridgeRequest(ctx, http.MethodPost, "/v1/listings/import/json", bytes.NewReader(in.Body))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		g.handlePOSTListingsImportJSON(rr, req)
		data, err := nodeBridgeSuccessData(rr)
		if err != nil {
			return nil, err
		}
		return &nodeDataOutput{Body: data}, nil
	})
}

// --- Public ---

func (g *Gateway) registerListingIndexByPeer(api huma.API) {
	type listingIndexPeerInput struct {
		PeerID string `path:"peerID" doc:"Seller peer ID for listing index."`
	}
	huma.Register(api, huma.Operation{
		OperationID: "listings-index-by-peer-id",
		Method:      http.MethodGet,
		Path:        "/v1/listings/index/{peerID}",
		Summary:     "Get listing index for a peer",
		Tags:        []string{"listings"},
	}, func(ctx context.Context, in *listingIndexPeerInput) (*nodeDataOutput, error) {
		rawURL := "/v1/listings/index/" + url.PathEscape(in.PeerID)
		req := nodeBridgeRequestWithVars(ctx, http.MethodGet, rawURL, nil, map[string]string{"peerID": in.PeerID})
		rr := httptest.NewRecorder()
		g.handleGETListingIndex(rr, req)
		data, err := nodeBridgeSuccessData(rr)
		if err != nil {
			return nil, err
		}
		return &nodeDataOutput{Body: data}, nil
	})
}

func (g *Gateway) registerListingIndex(api huma.API) {
	huma.Register(api, huma.Operation{
		OperationID: "listings-index",
		Method:      http.MethodGet,
		Path:        "/v1/listings/index",
		Summary:     "Get listing index (self)",
		Tags:        []string{"listings"},
	}, func(ctx context.Context, _ *struct{}) (*nodeDataOutput, error) {
		req := nodeBridgeRequest(ctx, http.MethodGet, "/v1/listings/index", nil)
		rr := httptest.NewRecorder()
		g.handleGETListingIndex(rr, req)
		data, err := nodeBridgeSuccessData(rr)
		if err != nil {
			return nil, err
		}
		return &nodeDataOutput{Body: data}, nil
	})
}

func (g *Gateway) registerListingTemplate(api huma.API) {
	huma.Register(api, huma.Operation{
		OperationID: "listings-template",
		Method:      http.MethodGet,
		Path:        "/v1/listings/template",
		Summary:     "Download batch import Excel template",
		Tags:        []string{"listings"},
	}, func(ctx context.Context, _ *struct{}) (*nodeLegacyBinaryBody, error) {
		req := nodeBridgeRequest(ctx, http.MethodGet, "/v1/listings/template", nil)
		rr := httptest.NewRecorder()
		g.handleGETListingsTemplate(rr, req)
		raw, err := nodeBridgeRecorderBinary(rr)
		if err != nil {
			return nil, err
		}
		return &nodeLegacyBinaryBody{Body: raw}, nil
	})
}

func (g *Gateway) registerListingGetByPeerSlug(api huma.API) {
	type listingPeerSlugInput struct {
		PeerID string `path:"peerID" doc:"Seller peer ID."`
		Slug   string `path:"slug" doc:"Listing slug."`
	}
	huma.Register(api, huma.Operation{
		OperationID: "listings-get-by-peer-slug",
		Method:      http.MethodGet,
		Path:        "/v1/listings/{peerID}/{slug}",
		Summary:     "Get public listing by peer ID and slug",
		Tags:        []string{"listings"},
	}, func(ctx context.Context, in *listingPeerSlugInput) (*nodeDataOutput, error) {
		rawURL := "/v1/listings/" + url.PathEscape(in.PeerID) + "/" + url.PathEscape(in.Slug)
		req := nodeBridgeRequestWithVars(ctx, http.MethodGet, rawURL, nil, map[string]string{"peerID": in.PeerID, "slug": in.Slug})
		rr := httptest.NewRecorder()
		g.handleGETListing(rr, req)
		data, err := nodeBridgeSuccessData(rr)
		if err != nil {
			return nil, err
		}
		return &nodeDataOutput{Body: data}, nil
	})
}

func (g *Gateway) registerListingGetByListingID(api huma.API) {
	type listingCIDInput struct {
		ListingID string `path:"listingID" doc:"Listing CID or seller slug shortcut."`
	}
	huma.Register(api, huma.Operation{
		OperationID: "listings-get-by-listing-id",
		Method:      http.MethodGet,
		Path:        "/v1/listings/{listingID}",
		Summary:     "Get public listing by listing ID (CID)",
		Tags:        []string{"listings"},
	}, func(ctx context.Context, in *listingCIDInput) (*nodeDataOutput, error) {
		rawURL := "/v1/listings/" + url.PathEscape(in.ListingID)
		req := nodeBridgeRequestWithVars(ctx, http.MethodGet, rawURL, nil, map[string]string{"listingID": in.ListingID})
		rr := httptest.NewRecorder()
		g.handleGETListing(rr, req)
		data, err := nodeBridgeSuccessData(rr)
		if err != nil {
			return nil, err
		}
		return &nodeDataOutput{Body: data}, nil
	})
}
