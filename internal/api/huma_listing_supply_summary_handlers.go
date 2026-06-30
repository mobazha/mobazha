//go:build !private_distribution

package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"

	"github.com/danielgtaylor/huma/v2"
)

func (g *Gateway) registerListingSupplySummaryCapability(api huma.API) {
	type listingSupplySummaryInput struct {
		Body json.RawMessage
	}
	huma.Register(api, huma.Operation{
		OperationID: "listings-post-supply-summary",
		Method:      http.MethodPost,
		Path:        "/v1/listings/supply-summary",
		Summary:     "Get seller supply summaries",
		Tags:        []string{"listings"},
		Security:    nodeAuthSecurity,
	}, func(ctx context.Context, in *listingSupplySummaryInput) (*nodeDataOutput, error) {
		req := nodeBridgeRequest(ctx, http.MethodPost, "/v1/listings/supply-summary", bytes.NewReader(in.Body))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		g.handlePOSTListingSupplySummary(rr, req)
		data, err := nodeBridgeSuccessData(rr)
		if err != nil {
			return nil, err
		}
		return &nodeDataOutput{Body: data}, nil
	})
}
