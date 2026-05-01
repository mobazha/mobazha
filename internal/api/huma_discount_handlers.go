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

// registerNodeHumaDiscountOperations registers bridged seller discount management OpenAPI ops (AH-1.4 Batch 4).
func (g *Gateway) registerNodeHumaDiscountOperations(api huma.API) {
	type jsonBody struct {
		Body json.RawMessage `json:",omitempty"`
	}

	type discountIDPath struct {
		DiscountID string `path:"discountID" doc:"Discount ID."`
	}

	huma.Register(api, huma.Operation{
		OperationID:   "discounts-post",
		Method:        http.MethodPost,
		Path:          "/v1/discounts",
		Summary:       "Create discount",
		Tags:          []string{"discounts"},
		Security:      nodeAuthSecurity,
		DefaultStatus: http.StatusCreated,
	}, func(ctx context.Context, in *jsonBody) (*nodeDataOutput, error) {
		req := nodeBridgeRequest(ctx, http.MethodPost, "/v1/discounts", bytes.NewReader(in.Body))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		g.handleCreateDiscount(rr, req)
		data, err := nodeBridgeRawSuccess(rr)
		if err != nil {
			return nil, err
		}
		return &nodeDataOutput{Body: data}, nil
	})

	type listQ struct {
		Page     string `query:"page"`
		PageSize string `query:"pageSize"`
		Status   string `query:"status"`
		Method   string `query:"method"`
		Q        string `query:"q"`
	}
	huma.Register(api, huma.Operation{
		OperationID: "discounts-get",
		Method:      http.MethodGet,
		Path:        "/v1/discounts",
		Summary:     "List discounts",
		Tags:        []string{"discounts"},
		Security:    nodeAuthSecurity,
	}, func(ctx context.Context, q *listQ) (*nodeDataOutput, error) {
		v := url.Values{}
		if q.Page != "" {
			v.Set("page", q.Page)
		}
		if q.PageSize != "" {
			v.Set("pageSize", q.PageSize)
		}
		if q.Status != "" {
			v.Set("status", q.Status)
		}
		if q.Method != "" {
			v.Set("method", q.Method)
		}
		if q.Q != "" {
			v.Set("q", q.Q)
		}
		rawURL := "/v1/discounts"
		if enc := v.Encode(); enc != "" {
			rawURL += "?" + enc
		}
		req := nodeBridgeRequest(ctx, http.MethodGet, rawURL, nil)
		rr := httptest.NewRecorder()
		g.handleListDiscounts(rr, req)
		data, err := nodeBridgeRawSuccess(rr)
		if err != nil {
			return nil, err
		}
		return &nodeDataOutput{Body: data}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "discounts-id-get",
		Method:      http.MethodGet,
		Path:        "/v1/discounts/{discountID}",
		Summary:     "Get discount",
		Tags:        []string{"discounts"},
		Security:    nodeAuthSecurity,
	}, func(ctx context.Context, in *discountIDPath) (*nodeDataOutput, error) {
		rawURL := "/v1/discounts/" + url.PathEscape(in.DiscountID)
		req := nodeBridgeRequestWithVars(ctx, http.MethodGet, rawURL, nil, map[string]string{"discountID": in.DiscountID})
		rr := httptest.NewRecorder()
		g.handleGetDiscount(rr, req)
		data, err := nodeBridgeSuccessData(rr)
		if err != nil {
			return nil, err
		}
		return &nodeDataOutput{Body: data}, nil
	})

	type discountPut struct {
		DiscountID string          `path:"discountID"`
		Body       json.RawMessage `json:",omitempty"`
	}
	huma.Register(api, huma.Operation{
		OperationID: "discounts-id-put",
		Method:      http.MethodPut,
		Path:        "/v1/discounts/{discountID}",
		Summary:     "Update discount",
		Tags:        []string{"discounts"},
		Security:    nodeAuthSecurity,
	}, func(ctx context.Context, in *discountPut) (*nodeDataOutput, error) {
		rawURL := "/v1/discounts/" + url.PathEscape(in.DiscountID)
		req := nodeBridgeRequestWithVars(ctx, http.MethodPut, rawURL, bytes.NewReader(in.Body), map[string]string{"discountID": in.DiscountID})
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		g.handleUpdateDiscount(rr, req)
		data, err := nodeBridgeSuccessData(rr)
		if err != nil {
			return nil, err
		}
		return &nodeDataOutput{Body: data}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "discounts-id-delete",
		Method:      http.MethodDelete,
		Path:        "/v1/discounts/{discountID}",
		Summary:     "Delete discount",
		Tags:        []string{"discounts"},
		Security:    nodeAuthSecurity,
	}, func(ctx context.Context, in *discountIDPath) (*nodeNoContentOutput, error) {
		rawURL := "/v1/discounts/" + url.PathEscape(in.DiscountID)
		req := nodeBridgeRequestWithVars(ctx, http.MethodDelete, rawURL, nil, map[string]string{"discountID": in.DiscountID})
		rr := httptest.NewRecorder()
		g.handleDeleteDiscount(rr, req)
		if err := nodeBridgeNoContent(rr); err != nil {
			return nil, err
		}
		return &nodeNoContentOutput{}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID:   "discounts-id-codes-post",
		Method:        http.MethodPost,
		Path:          "/v1/discounts/{discountID}/codes",
		Summary:       "Add discount codes",
		Tags:          []string{"discounts"},
		Security:      nodeAuthSecurity,
		DefaultStatus: http.StatusCreated,
	}, func(ctx context.Context, in *discountPut) (*nodeDataOutput, error) {
		rawURL := "/v1/discounts/" + url.PathEscape(in.DiscountID) + "/codes"
		req := nodeBridgeRequestWithVars(ctx, http.MethodPost, rawURL, bytes.NewReader(in.Body), map[string]string{"discountID": in.DiscountID})
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		g.handleAddDiscountCodes(rr, req)
		data, err := nodeBridgeRawSuccess(rr)
		if err != nil {
			return nil, err
		}
		return &nodeDataOutput{Body: data}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "discounts-id-codes-get",
		Method:      http.MethodGet,
		Path:        "/v1/discounts/{discountID}/codes",
		Summary:     "List discount codes",
		Tags:        []string{"discounts"},
		Security:    nodeAuthSecurity,
	}, func(ctx context.Context, in *discountIDPath) (*nodeDataOutput, error) {
		rawURL := "/v1/discounts/" + url.PathEscape(in.DiscountID) + "/codes"
		req := nodeBridgeRequestWithVars(ctx, http.MethodGet, rawURL, nil, map[string]string{"discountID": in.DiscountID})
		rr := httptest.NewRecorder()
		g.handleListDiscountCodes(rr, req)
		data, err := nodeBridgeSuccessData(rr)
		if err != nil {
			return nil, err
		}
		return &nodeDataOutput{Body: data}, nil
	})

	type codeDeletePath struct {
		DiscountID string `path:"discountID"`
		CodeID     string `path:"codeID"`
	}
	huma.Register(api, huma.Operation{
		OperationID: "discounts-id-codes-code-delete",
		Method:      http.MethodDelete,
		Path:        "/v1/discounts/{discountID}/codes/{codeID}",
		Summary:     "Delete a discount code",
		Tags:        []string{"discounts"},
		Security:    nodeAuthSecurity,
	}, func(ctx context.Context, in *codeDeletePath) (*nodeNoContentOutput, error) {
		rawURL := "/v1/discounts/" + url.PathEscape(in.DiscountID) + "/codes/" + url.PathEscape(in.CodeID)
		req := nodeBridgeRequestWithVars(ctx, http.MethodDelete, rawURL, nil, map[string]string{
			"discountID": in.DiscountID,
			"codeID":     in.CodeID,
		})
		rr := httptest.NewRecorder()
		g.handleDeleteDiscountCode(rr, req)
		if err := nodeBridgeNoContent(rr); err != nil {
			return nil, err
		}
		return &nodeNoContentOutput{}, nil
	})

	type redemptionsQ struct {
		DiscountID string `path:"discountID"`
		Page       string `query:"page"`
		PageSize   string `query:"pageSize"`
	}
	huma.Register(api, huma.Operation{
		OperationID: "discounts-id-redemptions-get",
		Method:      http.MethodGet,
		Path:        "/v1/discounts/{discountID}/redemptions",
		Summary:     "List discount redemptions",
		Tags:        []string{"discounts"},
		Security:    nodeAuthSecurity,
	}, func(ctx context.Context, q *redemptionsQ) (*nodeDataOutput, error) {
		v := url.Values{}
		if q.Page != "" {
			v.Set("page", q.Page)
		}
		if q.PageSize != "" {
			v.Set("pageSize", q.PageSize)
		}
		rawURL := "/v1/discounts/" + url.PathEscape(q.DiscountID) + "/redemptions"
		if enc := v.Encode(); enc != "" {
			rawURL += "?" + enc
		}
		req := nodeBridgeRequestWithVars(ctx, http.MethodGet, rawURL, nil, map[string]string{"discountID": q.DiscountID})
		rr := httptest.NewRecorder()
		g.handleListDiscountRedemptions(rr, req)
		data, err := nodeBridgeRawSuccess(rr)
		if err != nil {
			return nil, err
		}
		return &nodeDataOutput{Body: data}, nil
	})
}
