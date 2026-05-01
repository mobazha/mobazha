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

// registerNodeHumaCollectionOperations registers bridged collection management + public storefront ops (AH-1.4 Batch 4).
func (g *Gateway) registerNodeHumaCollectionOperations(api huma.API) {
	type jsonBody struct {
		Body json.RawMessage `json:",omitempty"`
	}

	type collectionIDPath struct {
		CollectionID string `path:"collectionID" doc:"Collection ID."`
	}

	huma.Register(api, huma.Operation{
		OperationID: "collections-post",
		Method:      http.MethodPost,
		Path:        "/v1/collections",
		Summary:     "Create collection",
		Tags:        []string{"collections"},
		Security:    nodeAuthSecurity,
	}, func(ctx context.Context, in *jsonBody) (*nodeDataOutput, error) {
		req := nodeBridgeRequest(ctx, http.MethodPost, "/v1/collections", bytes.NewReader(in.Body))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		g.handleCreateCollection(rr, req)
		data, err := nodeBridgeSuccessData(rr)
		if err != nil {
			return nil, err
		}
		return &nodeDataOutput{Body: data}, nil
	})

	type collectionsListQ struct {
		Page          string `query:"page"`
		PageSize      string `query:"pageSize"`
		PublishedOnly string `query:"publishedOnly"`
	}
	huma.Register(api, huma.Operation{
		OperationID: "collections-get",
		Method:      http.MethodGet,
		Path:        "/v1/collections",
		Summary:     "List collections",
		Tags:        []string{"collections"},
		Security:    nodeAuthSecurity,
	}, func(ctx context.Context, q *collectionsListQ) (*nodeDataOutput, error) {
		v := url.Values{}
		if q.Page != "" {
			v.Set("page", q.Page)
		}
		if q.PageSize != "" {
			v.Set("pageSize", q.PageSize)
		}
		if q.PublishedOnly != "" {
			v.Set("publishedOnly", q.PublishedOnly)
		}
		rawURL := "/v1/collections"
		if enc := v.Encode(); enc != "" {
			rawURL += "?" + enc
		}
		req := nodeBridgeRequest(ctx, http.MethodGet, rawURL, nil)
		rr := httptest.NewRecorder()
		g.handleListCollections(rr, req)
		data, err := nodeBridgeSuccessData(rr)
		if err != nil {
			return nil, err
		}
		return &nodeDataOutput{Body: data}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "collections-id-get",
		Method:      http.MethodGet,
		Path:        "/v1/collections/{collectionID}",
		Summary:     "Get collection",
		Tags:        []string{"collections"},
		Security:    nodeAuthSecurity,
	}, func(ctx context.Context, in *collectionIDPath) (*nodeDataOutput, error) {
		rawURL := "/v1/collections/" + url.PathEscape(in.CollectionID)
		req := nodeBridgeRequestWithVars(ctx, http.MethodGet, rawURL, nil, map[string]string{"collectionID": in.CollectionID})
		rr := httptest.NewRecorder()
		g.handleGetCollection(rr, req)
		data, err := nodeBridgeSuccessData(rr)
		if err != nil {
			return nil, err
		}
		return &nodeDataOutput{Body: data}, nil
	})

	type collPut struct {
		CollectionID string          `path:"collectionID"`
		Body         json.RawMessage `json:",omitempty"`
	}
	huma.Register(api, huma.Operation{
		OperationID: "collections-id-put",
		Method:      http.MethodPut,
		Path:        "/v1/collections/{collectionID}",
		Summary:     "Update collection",
		Tags:        []string{"collections"},
		Security:    nodeAuthSecurity,
	}, func(ctx context.Context, in *collPut) (*nodeDataOutput, error) {
		rawURL := "/v1/collections/" + url.PathEscape(in.CollectionID)
		req := nodeBridgeRequestWithVars(ctx, http.MethodPut, rawURL, bytes.NewReader(in.Body), map[string]string{"collectionID": in.CollectionID})
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		g.handleUpdateCollection(rr, req)
		data, err := nodeBridgeSuccessData(rr)
		if err != nil {
			return nil, err
		}
		return &nodeDataOutput{Body: data}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "collections-id-delete",
		Method:      http.MethodDelete,
		Path:        "/v1/collections/{collectionID}",
		Summary:     "Delete collection",
		Tags:        []string{"collections"},
		Security:    nodeAuthSecurity,
	}, func(ctx context.Context, in *collectionIDPath) (*nodeNoContentOutput, error) {
		rawURL := "/v1/collections/" + url.PathEscape(in.CollectionID)
		req := nodeBridgeRequestWithVars(ctx, http.MethodDelete, rawURL, nil, map[string]string{"collectionID": in.CollectionID})
		rr := httptest.NewRecorder()
		g.handleDeleteCollection(rr, req)
		if err := nodeBridgeNoContent(rr); err != nil {
			return nil, err
		}
		return &nodeNoContentOutput{}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "collections-id-products-post",
		Method:      http.MethodPost,
		Path:        "/v1/collections/{collectionID}/products",
		Summary:     "Add products to collection",
		Tags:        []string{"collections"},
		Security:    nodeAuthSecurity,
	}, func(ctx context.Context, in *collPut) (*nodeDataOutput, error) {
		rawURL := "/v1/collections/" + url.PathEscape(in.CollectionID) + "/products"
		req := nodeBridgeRequestWithVars(ctx, http.MethodPost, rawURL, bytes.NewReader(in.Body), map[string]string{"collectionID": in.CollectionID})
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		g.handleAddCollectionProducts(rr, req)
		data, err := nodeBridgeSuccessData(rr)
		if err != nil {
			return nil, err
		}
		return &nodeDataOutput{Body: data}, nil
	})

	type prodSlugPath struct {
		CollectionID string `path:"collectionID"`
		Slug         string `path:"slug" doc:"Listing slug."`
	}
	huma.Register(api, huma.Operation{
		OperationID: "collections-id-products-slug-delete",
		Method:      http.MethodDelete,
		Path:        "/v1/collections/{collectionID}/products/{slug}",
		Summary:     "Remove product from collection",
		Tags:        []string{"collections"},
		Security:    nodeAuthSecurity,
	}, func(ctx context.Context, in *prodSlugPath) (*nodeNoContentOutput, error) {
		rawURL := "/v1/collections/" + url.PathEscape(in.CollectionID) + "/products/" + url.PathEscape(in.Slug)
		req := nodeBridgeRequestWithVars(ctx, http.MethodDelete, rawURL, nil, map[string]string{
			"collectionID": in.CollectionID,
			"slug":         in.Slug,
		})
		rr := httptest.NewRecorder()
		g.handleRemoveCollectionProduct(rr, req)
		if err := nodeBridgeNoContent(rr); err != nil {
			return nil, err
		}
		return &nodeNoContentOutput{}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "collections-id-products-reorder-put",
		Method:      http.MethodPut,
		Path:        "/v1/collections/{collectionID}/products/reorder",
		Summary:     "Reorder products in collection",
		Tags:        []string{"collections"},
		Security:    nodeAuthSecurity,
	}, func(ctx context.Context, in *collPut) (*nodeNoContentOutput, error) {
		rawURL := "/v1/collections/" + url.PathEscape(in.CollectionID) + "/products/reorder"
		req := nodeBridgeRequestWithVars(ctx, http.MethodPut, rawURL, bytes.NewReader(in.Body), map[string]string{"collectionID": in.CollectionID})
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		g.handleReorderCollectionProducts(rr, req)
		if err := nodeBridgeNoContent(rr); err != nil {
			return nil, err
		}
		return &nodeNoContentOutput{}, nil
	})

	type peerPublishedQ struct {
		PeerID   string `path:"peerID" doc:"Store peer ID."`
		Page     string `query:"page"`
		PageSize string `query:"pageSize"`
	}
	huma.Register(api, huma.Operation{
		OperationID: "collections-peer-published-get",
		Method:      http.MethodGet,
		Path:        "/v1/collections/{peerID}/published",
		Summary:     "List published collections (public storefront)",
		Tags:        []string{"collections"},
	}, func(ctx context.Context, q *peerPublishedQ) (*nodeDataOutput, error) {
		v := url.Values{}
		if q.Page != "" {
			v.Set("page", q.Page)
		}
		if q.PageSize != "" {
			v.Set("pageSize", q.PageSize)
		}
		rawURL := "/v1/collections/" + url.PathEscape(q.PeerID) + "/published"
		if enc := v.Encode(); enc != "" {
			rawURL += "?" + enc
		}
		req := nodeBridgeRequestWithVars(ctx, http.MethodGet, rawURL, nil, map[string]string{"peerID": q.PeerID})
		rr := httptest.NewRecorder()
		g.handleListCollectionsPublic(rr, req)
		data, err := nodeBridgeSuccessData(rr)
		if err != nil {
			return nil, err
		}
		return &nodeDataOutput{Body: data}, nil
	})

	type peerPublishedID struct {
		PeerID       string `path:"peerID"`
		CollectionID string `path:"collectionID"`
	}
	huma.Register(api, huma.Operation{
		OperationID: "collections-peer-published-id-get",
		Method:      http.MethodGet,
		Path:        "/v1/collections/{peerID}/published/{collectionID}",
		Summary:     "Get published collection (public storefront)",
		Tags:        []string{"collections"},
	}, func(ctx context.Context, in *peerPublishedID) (*nodeDataOutput, error) {
		rawURL := "/v1/collections/" + url.PathEscape(in.PeerID) + "/published/" + url.PathEscape(in.CollectionID)
		req := nodeBridgeRequestWithVars(ctx, http.MethodGet, rawURL, nil, map[string]string{
			"peerID":       in.PeerID,
			"collectionID": in.CollectionID,
		})
		rr := httptest.NewRecorder()
		g.handleGetCollectionPublic(rr, req)
		data, err := nodeBridgeSuccessData(rr)
		if err != nil {
			return nil, err
		}
		return &nodeDataOutput{Body: data}, nil
	})
}
