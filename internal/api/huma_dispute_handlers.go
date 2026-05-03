//go:build !private_distribution

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

// registerNodeHumaDisputeOperations registers bridged dispute OpenAPI ops (AH-1.4 Batch 3).
func (g *Gateway) registerNodeHumaDisputeOperations(api huma.API) {
	g.registerDisputesInstructionsRelease(api)
	g.registerDisputesOpen(api)
	g.registerDisputesClose(api)
	g.registerDisputesAfterSale(api)
	g.registerDisputesRelease(api)
	g.registerDisputesReleaseAfterTimeout(api)
}

func (g *Gateway) registerDisputesInstructionsRelease(api huma.API) {
	type disputeOrderBody struct {
		OrderID string          `path:"orderID" doc:"Order ID."`
		Body    json.RawMessage `json:",omitempty"`
	}
	huma.Register(api, huma.Operation{
		OperationID: "disputes-post-instructions-release",
		Method:      http.MethodPost,
		Path:        "/v1/disputes/{orderID}/instructions/release",
		Summary:     "Get release-funds instructions for a dispute",
		Tags:        []string{"disputes"},
		Security:    nodeAuthSecurity,
	}, func(ctx context.Context, hi *disputeOrderBody) (*nodeDataOutput, error) {
		rawURL := "/v1/disputes/" + url.PathEscape(hi.OrderID) + "/instructions/release"
		req := nodeBridgeRequestWithVars(ctx, http.MethodPost, rawURL, bytes.NewReader(hi.Body), map[string]string{"orderID": hi.OrderID})
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		g.handleGETReleaseFundsInstructions(rr, req)
		data, err := nodeBridgeSuccessData(rr)
		if err != nil {
			return nil, err
		}
		return &nodeDataOutput{Body: data}, nil
	})
}

func (g *Gateway) registerDisputesOpen(api huma.API) {
	type disputeOrderBody struct {
		OrderID string          `path:"orderID" doc:"Order ID."`
		Body    json.RawMessage `json:",omitempty"`
	}
	huma.Register(api, huma.Operation{
		OperationID: "disputes-post-open",
		Method:      http.MethodPost,
		Path:        "/v1/disputes/{orderID}/open",
		Summary:     "Open a dispute",
		Tags:        []string{"disputes"},
		Security:    nodeAuthSecurity,
	}, func(ctx context.Context, hi *disputeOrderBody) (*nodeDataOutput, error) {
		rawURL := "/v1/disputes/" + url.PathEscape(hi.OrderID) + "/open"
		req := nodeBridgeRequestWithVars(ctx, http.MethodPost, rawURL, bytes.NewReader(hi.Body), map[string]string{"orderID": hi.OrderID})
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		g.handlePOSTOpenDispute(rr, req)
		data, err := nodeBridgeSuccessData(rr)
		if err != nil {
			return nil, err
		}
		return &nodeDataOutput{Body: data}, nil
	})
}

func (g *Gateway) registerDisputesClose(api huma.API) {
	type disputeOrderBody struct {
		OrderID string          `path:"orderID" doc:"Order ID."`
		Body    json.RawMessage `json:",omitempty"`
	}
	huma.Register(api, huma.Operation{
		OperationID: "disputes-post-close",
		Method:      http.MethodPost,
		Path:        "/v1/disputes/{orderID}/close",
		Summary:     "Close a dispute",
		Tags:        []string{"disputes"},
		Security:    nodeAuthSecurity,
	}, func(ctx context.Context, hi *disputeOrderBody) (*nodeDataOutput, error) {
		rawURL := "/v1/disputes/" + url.PathEscape(hi.OrderID) + "/close"
		req := nodeBridgeRequestWithVars(ctx, http.MethodPost, rawURL, bytes.NewReader(hi.Body), map[string]string{"orderID": hi.OrderID})
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		g.handlePOSTCloseDispute(rr, req)
		data, err := nodeBridgeSuccessData(rr)
		if err != nil {
			return nil, err
		}
		return &nodeDataOutput{Body: data}, nil
	})
}

func (g *Gateway) registerDisputesAfterSale(api huma.API) {
	type disputeOrderBody struct {
		OrderID string          `path:"orderID" doc:"Order ID."`
		Body    json.RawMessage `json:",omitempty"`
	}
	huma.Register(api, huma.Operation{
		OperationID: "disputes-post-after-sale",
		Method:      http.MethodPost,
		Path:        "/v1/disputes/{orderID}/after-sale",
		Summary:     "Open an after-sale dispute",
		Tags:        []string{"disputes"},
		Security:    nodeAuthSecurity,
	}, func(ctx context.Context, hi *disputeOrderBody) (*nodeDataOutput, error) {
		rawURL := "/v1/disputes/" + url.PathEscape(hi.OrderID) + "/after-sale"
		req := nodeBridgeRequestWithVars(ctx, http.MethodPost, rawURL, bytes.NewReader(hi.Body), map[string]string{"orderID": hi.OrderID})
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		g.handlePOSTOpenAfterSaleDispute(rr, req)
		data, err := nodeBridgeSuccessData(rr)
		if err != nil {
			return nil, err
		}
		return &nodeDataOutput{Body: data}, nil
	})
}

func (g *Gateway) registerDisputesRelease(api huma.API) {
	type disputeOrderBody struct {
		OrderID string          `path:"orderID" doc:"Order ID."`
		Body    json.RawMessage `json:",omitempty"`
	}
	huma.Register(api, huma.Operation{
		OperationID: "disputes-post-release",
		Method:      http.MethodPost,
		Path:        "/v1/disputes/{orderID}/release",
		Summary:     "Release escrow funds",
		Tags:        []string{"disputes"},
		Security:    nodeAuthSecurity,
	}, func(ctx context.Context, hi *disputeOrderBody) (*nodeDataOutput, error) {
		rawURL := "/v1/disputes/" + url.PathEscape(hi.OrderID) + "/release"
		req := nodeBridgeRequestWithVars(ctx, http.MethodPost, rawURL, bytes.NewReader(hi.Body), map[string]string{"orderID": hi.OrderID})
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		g.handlePOSTReleaseFunds(rr, req)
		data, err := nodeBridgeSuccessData(rr)
		if err != nil {
			return nil, err
		}
		return &nodeDataOutput{Body: data}, nil
	})
}

func (g *Gateway) registerDisputesReleaseAfterTimeout(api huma.API) {
	type disputeOrderBody struct {
		OrderID string          `path:"orderID" doc:"Order ID."`
		Body    json.RawMessage `json:",omitempty"`
	}
	huma.Register(api, huma.Operation{
		OperationID: "disputes-post-release-after-timeout",
		Method:      http.MethodPost,
		Path:        "/v1/disputes/{orderID}/release-after-timeout",
		Summary:     "Release escrow after timeout",
		Tags:        []string{"disputes"},
		Security:    nodeAuthSecurity,
	}, func(ctx context.Context, hi *disputeOrderBody) (*nodeDataOutput, error) {
		rawURL := "/v1/disputes/" + url.PathEscape(hi.OrderID) + "/release-after-timeout"
		req := nodeBridgeRequestWithVars(ctx, http.MethodPost, rawURL, bytes.NewReader(hi.Body), map[string]string{"orderID": hi.OrderID})
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		g.handlePOSTReleaseEscrow(rr, req)
		data, err := nodeBridgeSuccessData(rr)
		if err != nil {
			return nil, err
		}
		return &nodeDataOutput{Body: data}, nil
	})
}
