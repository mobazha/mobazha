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

// registerNodeHumaBuyRequestPublicOperations registers the wanted board
// endpoints that anyone may browse: the open board and a single request with
// its offers.
func (g *Gateway) registerNodeHumaBuyRequestPublicOperations(api huma.API) {
	g.registerBuyRequestList(api)
	g.registerBuyRequestGet(api)
}

// registerNodeHumaBuyRequestAdminOperations registers the authenticated
// wanted board endpoints: publishing, offering, accepting and cancelling.
func (g *Gateway) registerNodeHumaBuyRequestAdminOperations(api huma.API) {
	g.registerBuyRequestListMine(api)
	g.registerBuyRequestCreate(api)
	g.registerBuyRequestCancel(api)
	g.registerBuyRequestCreateOffer(api)
	g.registerBuyRequestAcceptOffer(api)
}

func (g *Gateway) registerBuyRequestList(api huma.API) {
	type buyRequestListInput struct {
		CategorySlug string `query:"categorySlug" required:"false" doc:"Filter by category slug."`
		Currency     string `query:"currency" required:"false" doc:"Only requests that accept this currency."`
		DeliverTo    string `query:"deliverTo" required:"false" doc:"ISO 3166-1 alpha-2 destination the goods must reach."`
		Status       string `query:"status" required:"false" doc:"Filter by status; defaults to open."`
		Page         int    `query:"page" required:"false" doc:"1-based page number."`
		PageSize     int    `query:"pageSize" required:"false" doc:"Items per page (max 100)."`
	}
	huma.Register(api, huma.Operation{
		OperationID: "buy-requests-list",
		Method:      http.MethodGet,
		Path:        "/v1/buy-requests",
		Summary:     "Browse open buy requests",
		Description: "The public wanted board: what buyers are looking for right now, with an offer count per request.",
		Tags:        []string{"buy-requests"},
	}, func(ctx context.Context, in *buyRequestListInput) (*nodeDataOutput, error) {
		query := url.Values{}
		if in.CategorySlug != "" {
			query.Set("categorySlug", in.CategorySlug)
		}
		if in.Currency != "" {
			query.Set("currency", in.Currency)
		}
		if in.DeliverTo != "" {
			query.Set("deliverTo", in.DeliverTo)
		}
		if in.Status != "" {
			query.Set("status", in.Status)
		}
		if in.Page > 0 {
			query.Set("page", strconv.Itoa(in.Page))
		}
		if in.PageSize > 0 {
			query.Set("pageSize", strconv.Itoa(in.PageSize))
		}
		rawURL := "/v1/buy-requests"
		if encoded := query.Encode(); encoded != "" {
			rawURL += "?" + encoded
		}
		req := nodeBridgeRequest(ctx, http.MethodGet, rawURL, nil)
		rr := httptest.NewRecorder()
		g.handleListBuyRequests(rr, req)
		data, err := nodeBridgeSuccessData(rr)
		if err != nil {
			return nil, err
		}
		return &nodeDataOutput{Body: data}, nil
	})
}

func (g *Gateway) registerBuyRequestGet(api huma.API) {
	type buyRequestGetInput struct {
		Slug string `path:"slug" doc:"Buy request slug."`
	}
	huma.Register(api, huma.Operation{
		OperationID: "buy-requests-get",
		Method:      http.MethodGet,
		Path:        "/v1/buy-requests/{slug}",
		Summary:     "Get a buy request with its offers",
		Description: "Offers are public on purpose: a wanted board only works when sellers can see the going rate.",
		Tags:        []string{"buy-requests"},
	}, func(ctx context.Context, in *buyRequestGetInput) (*nodeDataOutput, error) {
		rawURL := "/v1/buy-requests/" + url.PathEscape(in.Slug)
		req := nodeBridgeRequestWithVars(ctx, http.MethodGet, rawURL, nil, map[string]string{"slug": in.Slug})
		rr := httptest.NewRecorder()
		g.handleGetBuyRequest(rr, req)
		data, err := nodeBridgeSuccessData(rr)
		if err != nil {
			return nil, err
		}
		return &nodeDataOutput{Body: data}, nil
	})
}

func (g *Gateway) registerBuyRequestListMine(api huma.API) {
	type buyRequestMineInput struct {
		Page     int `query:"page" required:"false" doc:"1-based page number."`
		PageSize int `query:"pageSize" required:"false" doc:"Items per page (max 100)."`
	}
	huma.Register(api, huma.Operation{
		OperationID: "buy-requests-list-mine",
		Method:      http.MethodGet,
		Path:        "/v1/buy-requests/mine",
		Summary:     "List buy requests published by this node",
		Tags:        []string{"buy-requests"},
		Security:    nodeAuthSecurity,
	}, func(ctx context.Context, in *buyRequestMineInput) (*nodeDataOutput, error) {
		query := url.Values{}
		if in.Page > 0 {
			query.Set("page", strconv.Itoa(in.Page))
		}
		if in.PageSize > 0 {
			query.Set("pageSize", strconv.Itoa(in.PageSize))
		}
		rawURL := "/v1/buy-requests/mine"
		if encoded := query.Encode(); encoded != "" {
			rawURL += "?" + encoded
		}
		req := nodeBridgeRequest(ctx, http.MethodGet, rawURL, nil)
		rr := httptest.NewRecorder()
		g.handleListMyBuyRequests(rr, req)
		data, err := nodeBridgeSuccessData(rr)
		if err != nil {
			return nil, err
		}
		return &nodeDataOutput{Body: data}, nil
	})
}

func (g *Gateway) registerBuyRequestCreate(api huma.API) {
	type buyRequestBodyInput struct {
		Body json.RawMessage
	}
	huma.Register(api, huma.Operation{
		OperationID: "buy-requests-create",
		Method:      http.MethodPost,
		Path:        "/v1/buy-requests",
		Summary:     "Publish a buy request",
		Description: "A buyer states what they need and what they will pay at most. " +
			"Pass expiresInDays for the common \"keep this up for N days\" case, " +
			"or expiresAt with an RFC3339 timestamp for an exact deadline.",
		Tags:     []string{"buy-requests"},
		Security: nodeAuthSecurity,
	}, func(ctx context.Context, in *buyRequestBodyInput) (*nodeDataOutput, error) {
		req := nodeBridgeRequest(ctx, http.MethodPost, "/v1/buy-requests", bytes.NewReader(in.Body))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		g.handleCreateBuyRequest(rr, req)
		data, err := nodeBridgeSuccessData(rr)
		if err != nil {
			return nil, err
		}
		return &nodeDataOutput{Body: data}, nil
	})
}

func (g *Gateway) registerBuyRequestCancel(api huma.API) {
	type buyRequestCancelInput struct {
		Slug string `path:"slug" doc:"Buy request slug."`
	}
	huma.Register(api, huma.Operation{
		OperationID: "buy-requests-cancel",
		Method:      http.MethodDelete,
		Path:        "/v1/buy-requests/{slug}",
		Summary:     "Close a buy request",
		Description: "Only the buyer may close a request, and only while it is still open.",
		Tags:        []string{"buy-requests"},
		Security:    nodeAuthSecurity,
	}, func(ctx context.Context, in *buyRequestCancelInput) (*nodeDataOutput, error) {
		rawURL := "/v1/buy-requests/" + url.PathEscape(in.Slug)
		req := nodeBridgeRequestWithVars(ctx, http.MethodDelete, rawURL, nil, map[string]string{"slug": in.Slug})
		rr := httptest.NewRecorder()
		g.handleCancelBuyRequest(rr, req)
		data, err := nodeBridgeSuccessData(rr)
		if err != nil {
			return nil, err
		}
		return &nodeDataOutput{Body: data}, nil
	})
}

func (g *Gateway) registerBuyRequestCreateOffer(api huma.API) {
	type buyRequestOfferBodyInput struct {
		Slug string `path:"slug" doc:"Buy request slug."`
		Body json.RawMessage
	}
	huma.Register(api, huma.Operation{
		OperationID: "buy-requests-create-offer",
		Method:      http.MethodPost,
		Path:        "/v1/buy-requests/{slug}/offers",
		Summary:     "Offer on a buy request",
		Description: "A seller answers with a price, a quantity and a lead time. " +
			"Offers are binding, so a seller may only offer once per request, and " +
			"the currency must be one the buyer accepts.",
		Tags:     []string{"buy-requests"},
		Security: nodeAuthSecurity,
	}, func(ctx context.Context, in *buyRequestOfferBodyInput) (*nodeDataOutput, error) {
		rawURL := "/v1/buy-requests/" + url.PathEscape(in.Slug) + "/offers"
		req := nodeBridgeRequestWithVars(ctx, http.MethodPost, rawURL, bytes.NewReader(in.Body), map[string]string{"slug": in.Slug})
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		g.handleCreateBuyRequestOffer(rr, req)
		data, err := nodeBridgeSuccessData(rr)
		if err != nil {
			return nil, err
		}
		return &nodeDataOutput{Body: data}, nil
	})
}

func (g *Gateway) registerBuyRequestAcceptOffer(api huma.API) {
	type buyRequestAcceptInput struct {
		Slug string `path:"slug" doc:"Buy request slug."`
		Body json.RawMessage
	}
	huma.Register(api, huma.Operation{
		OperationID: "buy-requests-accept-offer",
		Method:      http.MethodPost,
		Path:        "/v1/buy-requests/{slug}/accept",
		Summary:     "Accept an offer",
		Description: "Locks the request onto the accepted offer, rejects the others and " +
			"returns an escrow order draft that the normal checkout flow consumes. " +
			"Payment never happens outside escrow.",
		Tags:     []string{"buy-requests"},
		Security: nodeAuthSecurity,
	}, func(ctx context.Context, in *buyRequestAcceptInput) (*nodeDataOutput, error) {
		rawURL := "/v1/buy-requests/" + url.PathEscape(in.Slug) + "/accept"
		req := nodeBridgeRequestWithVars(ctx, http.MethodPost, rawURL, bytes.NewReader(in.Body), map[string]string{"slug": in.Slug})
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		g.handleAcceptBuyRequestOffer(rr, req)
		data, err := nodeBridgeSuccessData(rr)
		if err != nil {
			return nil, err
		}
		return &nodeDataOutput{Body: data}, nil
	})
}
