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

// registerNodeHumaOrderOperations registers bridged order / checkout / analytics OpenAPI ops (AH-1.4 Batch 3).
func (g *Gateway) registerNodeHumaOrderOperations(api huma.API) {
	g.registerOrdersInstructionPayment(api)
	g.registerOrdersInstructionConfirm(api)
	g.registerOrdersInstructionRefund(api)
	g.registerOrdersInstructionComplete(api)
	g.registerOrdersInstructionCancel(api)

	g.registerPurchasesPost(api)
	g.registerPurchasesGet(api)
	g.registerSalesGet(api)
	g.registerSalesPost(api)
	g.registerCasesGet(api)
	g.registerCasesPost(api)

	g.registerOrdersPostEstimate(api)
	g.registerOrdersPostCheckoutBreakdown(api)
	g.registerOrdersPostPurchase(api)
	g.registerOrdersPostSpend(api)
	g.registerOrdersPostPayment(api)
	g.registerOrdersPostConfirmation(api)
	g.registerOrdersPostShipment(api)
	g.registerOrdersPostRefund(api)
	g.registerOrdersPostComplete(api)
	g.registerOrdersPostRate(api)
	g.registerOrdersPostCancel(api)
	g.registerOrdersPostExtendProtection(api)

	g.registerCaseGet(api)
	g.registerOrderGet(api)
	g.registerOrderPaymentRemainingGet(api)
	g.registerOrderPaymentCancelPartialPost(api)
	g.registerOrderPaymentWatchDelete(api)

	g.registerGuestOrdersListAuth(api)
	g.registerGuestOrderShip(api)
	g.registerGuestOrderComplete(api)

	g.registerAnalyticsStatsGet(api)

	g.registerGuestOrderPostPublic(api)
	g.registerGuestOrderGetPublic(api)
	g.registerPaymentMethodsGet(api)
	g.registerAnalyticsShopEventsPost(api)
}

func orderSearchQueryVals(state, search, sortBy, limit string) url.Values {
	q := url.Values{}
	if state != "" {
		q.Set("state", state)
	}
	if search != "" {
		q.Set("search", search)
	}
	if sortBy != "" {
		q.Set("sortBy", sortBy)
	}
	if limit != "" {
		q.Set("limit", limit)
	}
	return q
}

func salesSearchQueryVals(typeVal, state, search, sortBy, limit, page, pageSize string) url.Values {
	q := orderSearchQueryVals(state, search, sortBy, limit)
	if typeVal != "" {
		q.Set("type", typeVal)
	}
	if page != "" {
		q.Set("page", page)
	}
	if pageSize != "" {
		q.Set("pageSize", pageSize)
	}
	return q
}

func (g *Gateway) registerOrdersInstructionPayment(api huma.API) {
	type in struct {
		OrderID string          `path:"orderID" doc:"Order ID."`
		Body    json.RawMessage `json:",omitempty"`
	}
	huma.Register(api, huma.Operation{
		OperationID: "orders-post-instructions-payment",
		Method:      http.MethodPost,
		Path:        "/v1/orders/{orderID}/instructions/payment",
		Summary:     "Build payment funding instructions",
		Tags:        []string{"orders"},
		Security:    nodeAuthSecurity,
	}, func(ctx context.Context, hi *in) (*nodeDataOutput, error) {
		rawURL := "/v1/orders/" + url.PathEscape(hi.OrderID) + "/instructions/payment"
		req := nodeBridgeRequestWithVars(ctx, http.MethodPost, rawURL, bytes.NewReader(hi.Body), map[string]string{"orderID": hi.OrderID})
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		g.handleGetOrderPaymentInstructions(rr, req)
		data, err := nodeBridgeSuccessData(rr)
		if err != nil {
			return nil, err
		}
		return &nodeDataOutput{Body: data}, nil
	})
}

func (g *Gateway) registerOrdersInstructionConfirm(api huma.API) {
	type in struct {
		OrderID string          `path:"orderID" doc:"Order ID."`
		Body    json.RawMessage `json:",omitempty"`
	}
	reg := func(opID string, tail string, summary string) {
		huma.Register(api, huma.Operation{
			OperationID: opID,
			Method:      http.MethodPost,
			Path:        "/v1/orders/{orderID}/instructions/" + tail,
			Summary:     summary,
			Tags:        []string{"orders"},
			Security:    nodeAuthSecurity,
		}, func(ctx context.Context, hi *in) (*nodeDataOutput, error) {
			rawURL := "/v1/orders/" + url.PathEscape(hi.OrderID) + "/instructions/" + tail
			req := nodeBridgeRequestWithVars(ctx, http.MethodPost, rawURL, bytes.NewReader(hi.Body), map[string]string{"orderID": hi.OrderID})
			req.Header.Set("Content-Type", "application/json")
			rr := httptest.NewRecorder()
			g.handleGETOrderConfirmationInstructions(rr, req)
			data, err := nodeBridgeSuccessData(rr)
			if err != nil {
				return nil, err
			}
			return &nodeDataOutput{Body: data}, nil
		})
	}
	reg("orders-post-instructions-confirm", "confirm", "Confirmation / acceptance instructions")
	reg("orders-post-instructions-decline", "decline", "Decline / refund-flow instructions via confirmation handler")
}

func (g *Gateway) registerOrdersInstructionRefund(api huma.API) {
	type in struct {
		OrderID string          `path:"orderID" doc:"Order ID."`
		Body    json.RawMessage `json:",omitempty"`
	}
	huma.Register(api, huma.Operation{
		OperationID: "orders-post-instructions-refund",
		Method:      http.MethodPost,
		Path:        "/v1/orders/{orderID}/instructions/refund",
		Summary:     "Refund instructions",
		Tags:        []string{"orders"},
		Security:    nodeAuthSecurity,
	}, func(ctx context.Context, hi *in) (*nodeDataOutput, error) {
		rawURL := "/v1/orders/" + url.PathEscape(hi.OrderID) + "/instructions/refund"
		req := nodeBridgeRequestWithVars(ctx, http.MethodPost, rawURL, bytes.NewReader(hi.Body), map[string]string{"orderID": hi.OrderID})
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		g.handleGETOrderRefundInstructions(rr, req)
		data, err := nodeBridgeSuccessData(rr)
		if err != nil {
			return nil, err
		}
		return &nodeDataOutput{Body: data}, nil
	})
}

func (g *Gateway) registerOrdersInstructionComplete(api huma.API) {
	type in struct {
		OrderID string          `path:"orderID" doc:"Order ID."`
		Body    json.RawMessage `json:",omitempty"`
	}
	huma.Register(api, huma.Operation{
		OperationID: "orders-post-instructions-complete",
		Method:      http.MethodPost,
		Path:        "/v1/orders/{orderID}/instructions/complete",
		Summary:     "Order completion payout instructions",
		Tags:        []string{"orders"},
		Security:    nodeAuthSecurity,
	}, func(ctx context.Context, hi *in) (*nodeDataOutput, error) {
		rawURL := "/v1/orders/" + url.PathEscape(hi.OrderID) + "/instructions/complete"
		req := nodeBridgeRequestWithVars(ctx, http.MethodPost, rawURL, bytes.NewReader(hi.Body), map[string]string{"orderID": hi.OrderID})
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		g.handleGETOrderCompleteInstructions(rr, req)
		data, err := nodeBridgeSuccessData(rr)
		if err != nil {
			return nil, err
		}
		return &nodeDataOutput{Body: data}, nil
	})
}

func (g *Gateway) registerOrdersInstructionCancel(api huma.API) {
	type in struct {
		OrderID string          `path:"orderID" doc:"Order ID."`
		Body    json.RawMessage `json:",omitempty"`
	}
	huma.Register(api, huma.Operation{
		OperationID: "orders-post-instructions-cancel",
		Method:      http.MethodPost,
		Path:        "/v1/orders/{orderID}/instructions/cancel",
		Summary:     "Cancellation / refund-before-ship instructions",
		Tags:        []string{"orders"},
		Security:    nodeAuthSecurity,
	}, func(ctx context.Context, hi *in) (*nodeDataOutput, error) {
		rawURL := "/v1/orders/" + url.PathEscape(hi.OrderID) + "/instructions/cancel"
		req := nodeBridgeRequestWithVars(ctx, http.MethodPost, rawURL, bytes.NewReader(hi.Body), map[string]string{"orderID": hi.OrderID})
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		g.handleGETOrderCancelInstructions(rr, req)
		data, err := nodeBridgeSuccessData(rr)
		if err != nil {
			return nil, err
		}
		return &nodeDataOutput{Body: data}, nil
	})
}

func (g *Gateway) registerPurchasesPost(api huma.API) {
	type in struct {
		Body json.RawMessage `json:",omitempty"`
	}
	huma.Register(api, huma.Operation{
		OperationID: "purchases-post-query",
		Method:      http.MethodPost,
		Path:        "/v1/purchases",
		Summary:     "Search purchases via POST JSON filter",
		Tags:        []string{"orders", "purchases"},
		Security:    nodeAuthSecurity,
	}, func(ctx context.Context, hi *in) (*nodeDataOutput, error) {
		req := nodeBridgeRequest(ctx, http.MethodPost, "/v1/purchases", bytes.NewReader(hi.Body))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		g.handlePOSTPurchases(rr, req)
		data, err := nodeBridgeSuccessData(rr)
		if err != nil {
			return nil, err
		}
		return &nodeDataOutput{Body: data}, nil
	})
}

func (g *Gateway) registerPurchasesGet(api huma.API) {
	type q struct {
		State  string `query:"state"`
		Search string `query:"search"`
		SortBy string `query:"sortBy"`
		Limit  string `query:"limit"`
	}
	huma.Register(api, huma.Operation{
		OperationID: "purchases-get-query",
		Method:      http.MethodGet,
		Path:        "/v1/purchases",
		Summary:     "Buyer purchase history",
		Tags:        []string{"orders", "purchases"},
		Security:    nodeAuthSecurity,
	}, func(ctx context.Context, hq *q) (*nodeDataOutput, error) {
		qs := orderSearchQueryVals(hq.State, hq.Search, hq.SortBy, hq.Limit).Encode()
		rawURL := "/v1/purchases"
		if qs != "" {
			rawURL += "?" + qs
		}
		req := nodeBridgeRequest(ctx, http.MethodGet, rawURL, nil)
		rr := httptest.NewRecorder()
		g.handleGETPurchases(rr, req)
		data, err := nodeBridgeSuccessData(rr)
		if err != nil {
			return nil, err
		}
		return &nodeDataOutput{Body: data}, nil
	})
}

func (g *Gateway) registerSalesGet(api huma.API) {
	type q struct {
		Type     string `query:"type"`
		State    string `query:"state"`
		Search   string `query:"search"`
		SortBy   string `query:"sortBy"`
		Limit    string `query:"limit"`
		Page     string `query:"page"`
		PageSize string `query:"pageSize"`
	}
	huma.Register(api, huma.Operation{
		OperationID: "sales-get-query",
		Method:      http.MethodGet,
		Path:        "/v1/sales",
		Summary:     "Seller sales feed",
		Tags:        []string{"orders", "sales"},
		Security:    nodeAuthSecurity,
	}, func(ctx context.Context, hq *q) (*nodeDataOutput, error) {
		v := salesSearchQueryVals(hq.Type, hq.State, hq.Search, hq.SortBy, hq.Limit, hq.Page, hq.PageSize)
		rawURL := "/v1/sales"
		if enc := v.Encode(); enc != "" {
			rawURL += "?" + enc
		}
		req := nodeBridgeRequest(ctx, http.MethodGet, rawURL, nil)
		rr := httptest.NewRecorder()
		g.handleGETSales(rr, req)
		data, err := nodeBridgeSuccessData(rr)
		if err != nil {
			return nil, err
		}
		return &nodeDataOutput{Body: data}, nil
	})
}

func (g *Gateway) registerSalesPost(api huma.API) {
	type in struct {
		Body json.RawMessage `json:",omitempty"`
	}
	huma.Register(api, huma.Operation{
		OperationID: "sales-post-query",
		Method:      http.MethodPost,
		Path:        "/v1/sales",
		Summary:     "Seller sales POST filter",
		Tags:        []string{"orders", "sales"},
		Security:    nodeAuthSecurity,
	}, func(ctx context.Context, hi *in) (*nodeDataOutput, error) {
		req := nodeBridgeRequest(ctx, http.MethodPost, "/v1/sales", bytes.NewReader(hi.Body))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		g.handlePostSales(rr, req)
		data, err := nodeBridgeSuccessData(rr)
		if err != nil {
			return nil, err
		}
		return &nodeDataOutput{Body: data}, nil
	})
}

func (g *Gateway) registerCasesGet(api huma.API) {
	type q struct {
		State  string `query:"state"`
		Search string `query:"search"`
		SortBy string `query:"sortBy"`
		Limit  string `query:"limit"`
	}
	huma.Register(api, huma.Operation{
		OperationID: "cases-get-query",
		Method:      http.MethodGet,
		Path:        "/v1/cases",
		Summary:     "List dispute cases",
		Tags:        []string{"orders", "cases"},
		Security:    nodeAuthSecurity,
	}, func(ctx context.Context, hq *q) (*nodeDataOutput, error) {
		qs := orderSearchQueryVals(hq.State, hq.Search, hq.SortBy, hq.Limit).Encode()
		rawURL := "/v1/cases"
		if qs != "" {
			rawURL += "?" + qs
		}
		req := nodeBridgeRequest(ctx, http.MethodGet, rawURL, nil)
		rr := httptest.NewRecorder()
		g.handleGETCases(rr, req)
		data, err := nodeBridgeSuccessData(rr)
		if err != nil {
			return nil, err
		}
		return &nodeDataOutput{Body: data}, nil
	})
}

func (g *Gateway) registerCasesPost(api huma.API) {
	type in struct {
		Body json.RawMessage `json:",omitempty"`
	}
	huma.Register(api, huma.Operation{
		OperationID: "cases-post-query",
		Method:      http.MethodPost,
		Path:        "/v1/cases",
		Summary:     "List dispute cases via JSON filter",
		Tags:        []string{"orders", "cases"},
		Security:    nodeAuthSecurity,
	}, func(ctx context.Context, hi *in) (*nodeDataOutput, error) {
		req := nodeBridgeRequest(ctx, http.MethodPost, "/v1/cases", bytes.NewReader(hi.Body))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		g.handlePostCases(rr, req)
		data, err := nodeBridgeSuccessData(rr)
		if err != nil {
			return nil, err
		}
		return &nodeDataOutput{Body: data}, nil
	})
}

func (g *Gateway) registerOrdersPostEstimate(api huma.API) {
	type in struct {
		Body json.RawMessage `json:",omitempty"`
	}
	huma.Register(api, huma.Operation{
		OperationID: "orders-post-estimate-total",
		Method:      http.MethodPost,
		Path:        "/v1/orders/estimate",
		Summary:     "Estimate totals for checkout",
		Tags:        []string{"orders"},
		Security:    nodeAuthSecurity,
	}, func(ctx context.Context, hi *in) (*nodeDataOutput, error) {
		req := nodeBridgeRequest(ctx, http.MethodPost, "/v1/orders/estimate", bytes.NewReader(hi.Body))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		g.handlePOSTEstimateTotal(rr, req)
		data, err := nodeBridgeSuccessData(rr)
		if err != nil {
			return nil, err
		}
		return &nodeDataOutput{Body: data}, nil
	})
}

func (g *Gateway) registerOrdersPostCheckoutBreakdown(api huma.API) {
	type in struct {
		Body json.RawMessage `json:",omitempty"`
	}
	huma.Register(api, huma.Operation{
		OperationID: "orders-post-checkout-breakdown",
		Method:      http.MethodPost,
		Path:        "/v1/orders/checkout-breakdown",
		Summary:     "Pricing breakdown preview",
		Tags:        []string{"orders"},
		Security:    nodeAuthSecurity,
	}, func(ctx context.Context, hi *in) (*nodeDataOutput, error) {
		req := nodeBridgeRequest(ctx, http.MethodPost, "/v1/orders/checkout-breakdown", bytes.NewReader(hi.Body))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		g.handlePOSTCheckoutBreakdown(rr, req)
		data, err := nodeBridgeSuccessData(rr)
		if err != nil {
			return nil, err
		}
		return &nodeDataOutput{Body: data}, nil
	})
}

func (g *Gateway) registerOrdersPostPurchase(api huma.API) {
	type in struct {
		Body json.RawMessage `json:",omitempty"`
	}
	huma.Register(api, huma.Operation{
		OperationID: "orders-post-create-purchase",
		Method:      http.MethodPost,
		Path:        "/v1/orders",
		Summary:     "Submit a new multi-item purchase order",
		Tags:        []string{"orders"},
		Security:    nodeAuthSecurity,
	}, func(ctx context.Context, hi *in) (*nodeDataOutput, error) {
		req := nodeBridgeRequest(ctx, http.MethodPost, "/v1/orders", bytes.NewReader(hi.Body))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		g.handlePOSTPurchase(rr, req)
		data, err := nodeBridgeSuccessData(rr)
		if err != nil {
			return nil, err
		}
		return &nodeDataOutput{Body: data}, nil
	})
}

func (g *Gateway) registerOrdersPostSpend(api huma.API) {
	type in struct {
		OrderID string          `path:"orderID" doc:"Order ID targeted for spends."`
		Body    json.RawMessage `json:",omitempty"`
	}
	huma.Register(api, huma.Operation{
		OperationID: "orders-post-spend-for-order",
		Method:      http.MethodPost,
		Path:        "/v1/orders/{orderID}/spend",
		Summary:     "Execute wallet spend in order context",
		Tags:        []string{"orders", "wallet"},
		Security:    nodeAuthSecurity,
	}, func(ctx context.Context, hi *in) (*nodeDataOutput, error) {
		rawURL := "/v1/orders/" + url.PathEscape(hi.OrderID) + "/spend"
		req := nodeBridgeRequestWithVars(ctx, http.MethodPost, rawURL, bytes.NewReader(hi.Body), map[string]string{"orderID": hi.OrderID})
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		g.handlePostSpendForOrder(rr, req)
		data, err := nodeBridgeSuccessData(rr)
		if err != nil {
			return nil, err
		}
		return &nodeDataOutput{Body: data}, nil
	})
}

func (g *Gateway) registerOrdersPostPayment(api huma.API) {
	type in struct {
		OrderID string          `path:"orderID" doc:"Order ID."`
		Body    json.RawMessage `json:",omitempty"`
	}
	huma.Register(api, huma.Operation{
		OperationID: "orders-post-payment",
		Method:      http.MethodPost,
		Path:        "/v1/orders/{orderID}/payment",
		Summary:     "Buyer submit payment payloads",
		Tags:        []string{"orders"},
		Security:    nodeAuthSecurity,
	}, func(ctx context.Context, hi *in) (*nodeDataOutput, error) {
		rawURL := "/v1/orders/" + url.PathEscape(hi.OrderID) + "/payment"
		req := nodeBridgeRequestWithVars(ctx, http.MethodPost, rawURL, bytes.NewReader(hi.Body), map[string]string{"orderID": hi.OrderID})
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		g.handlePOSTPayment(rr, req)
		data, err := nodeBridgeSuccessData(rr)
		if err != nil {
			return nil, err
		}
		return &nodeDataOutput{Body: data}, nil
	})
}

func (g *Gateway) registerOrdersPostConfirmation(api huma.API) {
	type in struct {
		OrderID string          `path:"orderID" doc:"Order ID."`
		Body    json.RawMessage `json:",omitempty"`
	}
	huma.Register(api, huma.Operation{
		OperationID: "orders-post-confirm",
		Method:      http.MethodPost,
		Path:        "/v1/orders/{orderID}/confirm",
		Summary:     "Buyer confirm or decline order",
		Tags:        []string{"orders"},
		Security:    nodeAuthSecurity,
	}, func(ctx context.Context, hi *in) (*nodeDataOutput, error) {
		rawURL := "/v1/orders/" + url.PathEscape(hi.OrderID) + "/confirm"
		req := nodeBridgeRequestWithVars(ctx, http.MethodPost, rawURL, bytes.NewReader(hi.Body), map[string]string{"orderID": hi.OrderID})
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		g.handlePOSTOrderConfirmation(rr, req)
		data, err := nodeBridgeSuccessData(rr)
		if err != nil {
			return nil, err
		}
		return &nodeDataOutput{Body: data}, nil
	})
}

func (g *Gateway) registerOrdersPostShipment(api huma.API) {
	type in struct {
		OrderID string          `path:"orderID" doc:"Order ID."`
		Body    json.RawMessage `json:",omitempty"`
	}
	huma.Register(api, huma.Operation{
		OperationID: "orders-post-ship",
		Method:      http.MethodPost,
		Path:        "/v1/orders/{orderID}/ship",
		Summary:     "Fulfill shipments",
		Tags:        []string{"orders"},
		Security:    nodeAuthSecurity,
	}, func(ctx context.Context, hi *in) (*nodeDataOutput, error) {
		rawURL := "/v1/orders/" + url.PathEscape(hi.OrderID) + "/ship"
		req := nodeBridgeRequestWithVars(ctx, http.MethodPost, rawURL, bytes.NewReader(hi.Body), map[string]string{"orderID": hi.OrderID})
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		g.handlePOSTOrderShipment(rr, req)
		data, err := nodeBridgeSuccessData(rr)
		if err != nil {
			return nil, err
		}
		return &nodeDataOutput{Body: data}, nil
	})
}

func (g *Gateway) registerOrdersPostRefund(api huma.API) {
	type in struct {
		OrderID string          `path:"orderID" doc:"Order ID."`
		Body    json.RawMessage `json:",omitempty"`
	}
	huma.Register(api, huma.Operation{
		OperationID: "orders-post-refund",
		Method:      http.MethodPost,
		Path:        "/v1/orders/{orderID}/refund",
		Summary:     "Finalize refund transaction",
		Tags:        []string{"orders"},
		Security:    nodeAuthSecurity,
	}, func(ctx context.Context, hi *in) (*nodeDataOutput, error) {
		rawURL := "/v1/orders/" + url.PathEscape(hi.OrderID) + "/refund"
		req := nodeBridgeRequestWithVars(ctx, http.MethodPost, rawURL, bytes.NewReader(hi.Body), map[string]string{"orderID": hi.OrderID})
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		g.handlePOSTOrderRefund(rr, req)
		data, err := nodeBridgeSuccessData(rr)
		if err != nil {
			return nil, err
		}
		return &nodeDataOutput{Body: data}, nil
	})
}

func (g *Gateway) registerOrdersPostComplete(api huma.API) {
	type in struct {
		OrderID string          `path:"orderID" doc:"Order ID."`
		Body    json.RawMessage `json:",omitempty"`
	}
	huma.Register(api, huma.Operation{
		OperationID: "orders-post-complete",
		Method:      http.MethodPost,
		Path:        "/v1/orders/{orderID}/complete",
		Summary:     "Finalize fully paid order (+ optional ratings)",
		Tags:        []string{"orders"},
		Security:    nodeAuthSecurity,
	}, func(ctx context.Context, hi *in) (*nodeDataOutput, error) {
		rawURL := "/v1/orders/" + url.PathEscape(hi.OrderID) + "/complete"
		req := nodeBridgeRequestWithVars(ctx, http.MethodPost, rawURL, bytes.NewReader(hi.Body), map[string]string{"orderID": hi.OrderID})
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		g.handlePOSTOrderCompletion(rr, req)
		data, err := nodeBridgeSuccessData(rr)
		if err != nil {
			return nil, err
		}
		return &nodeDataOutput{Body: data}, nil
	})
}

func (g *Gateway) registerOrdersPostRate(api huma.API) {
	type in struct {
		OrderID string          `path:"orderID" doc:"Order ID."`
		Body    json.RawMessage `json:",omitempty"`
	}
	huma.Register(api, huma.Operation{
		OperationID: "orders-post-rate",
		Method:      http.MethodPost,
		Path:        "/v1/orders/{orderID}/rate",
		Summary:     "Seller/buyer rate order fulfillment",
		Tags:        []string{"orders", "ratings"},
		Security:    nodeAuthSecurity,
	}, func(ctx context.Context, hi *in) (*nodeDataOutput, error) {
		rawURL := "/v1/orders/" + url.PathEscape(hi.OrderID) + "/rate"
		req := nodeBridgeRequestWithVars(ctx, http.MethodPost, rawURL, bytes.NewReader(hi.Body), map[string]string{"orderID": hi.OrderID})
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		g.handlePOSTOrderRate(rr, req)
		data, err := nodeBridgeSuccessData(rr)
		if err != nil {
			return nil, err
		}
		return &nodeDataOutput{Body: data}, nil
	})
}

func (g *Gateway) registerOrdersPostCancel(api huma.API) {
	type in struct {
		OrderID string          `path:"orderID" doc:"Order ID."`
		Body    json.RawMessage `json:",omitempty"`
	}
	huma.Register(api, huma.Operation{
		OperationID: "orders-post-cancel",
		Method:      http.MethodPost,
		Path:        "/v1/orders/{orderID}/cancel",
		Summary:     "Broadcast cancel escrow transaction",
		Tags:        []string{"orders"},
		Security:    nodeAuthSecurity,
	}, func(ctx context.Context, hi *in) (*nodeDataOutput, error) {
		rawURL := "/v1/orders/" + url.PathEscape(hi.OrderID) + "/cancel"
		req := nodeBridgeRequestWithVars(ctx, http.MethodPost, rawURL, bytes.NewReader(hi.Body), map[string]string{"orderID": hi.OrderID})
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		g.handlePOSTOrderCancel(rr, req)
		data, err := nodeBridgeSuccessData(rr)
		if err != nil {
			return nil, err
		}
		return &nodeDataOutput{Body: data}, nil
	})
}

func (g *Gateway) registerOrdersPostExtendProtection(api huma.API) {
	type in struct {
		OrderID string `path:"orderID" doc:"Order ID."`
	}
	huma.Register(api, huma.Operation{
		OperationID: "orders-post-extend-protection",
		Method:      http.MethodPost,
		Path:        "/v1/orders/{orderID}/extend-protection",
		Summary:     "Buyer protection extension preview",
		Tags:        []string{"orders"},
		Security:    nodeAuthSecurity,
	}, func(ctx context.Context, hi *in) (*nodeDataOutput, error) {
		rawURL := "/v1/orders/" + url.PathEscape(hi.OrderID) + "/extend-protection"
		req := nodeBridgeRequestWithVars(ctx, http.MethodPost, rawURL, nil, map[string]string{"orderID": hi.OrderID})
		rr := httptest.NewRecorder()
		g.handlePOSTExtendProtection(rr, req)
		data, err := nodeBridgeSuccessData(rr)
		if err != nil {
			return nil, err
		}
		return &nodeDataOutput{Body: data}, nil
	})
}

func (g *Gateway) registerCaseGet(api huma.API) {
	type in struct {
		OrderID string `path:"orderID" doc:"Underlying order ID for case."`
	}
	huma.Register(api, huma.Operation{
		OperationID: "cases-get-detail",
		Method:      http.MethodGet,
		Path:        "/v1/cases/{orderID}",
		Summary:     "Retrieve dispute case envelope",
		Tags:        []string{"cases"},
		Security:    nodeAuthSecurity,
	}, func(ctx context.Context, hi *in) (*nodeDataOutput, error) {
		rawURL := "/v1/cases/" + url.PathEscape(hi.OrderID)
		req := nodeBridgeRequestWithVars(ctx, http.MethodGet, rawURL, nil, map[string]string{"orderID": hi.OrderID})
		rr := httptest.NewRecorder()
		g.handleGetCase(rr, req)
		data, err := nodeBridgeSuccessData(rr)
		if err != nil {
			return nil, err
		}
		return &nodeDataOutput{Body: data}, nil
	})
}

func (g *Gateway) registerOrderGet(api huma.API) {
	type in struct {
		OrderID string `path:"orderID" doc:"Order ID."`
	}
	huma.Register(api, huma.Operation{
		OperationID: "orders-get-detail",
		Method:      http.MethodGet,
		Path:        "/v1/orders/{orderID}",
		Summary:     "Order detail snapshot",
		Tags:        []string{"orders"},
		Security:    nodeAuthSecurity,
	}, func(ctx context.Context, hi *in) (*nodeDataOutput, error) {
		rawURL := "/v1/orders/" + url.PathEscape(hi.OrderID)
		req := nodeBridgeRequestWithVars(ctx, http.MethodGet, rawURL, nil, map[string]string{"orderID": hi.OrderID})
		rr := httptest.NewRecorder()
		g.handleGETOrder(rr, req)
		data, err := nodeBridgeSuccessData(rr)
		if err != nil {
			return nil, err
		}
		return &nodeDataOutput{Body: data}, nil
	})
}

func (g *Gateway) registerOrderPaymentRemainingGet(api huma.API) {
	type in struct {
		OrderID string `path:"orderID" doc:"Order ID."`
	}
	huma.Register(api, huma.Operation{
		OperationID: "orders-get-payment-remaining",
		Method:      http.MethodGet,
		Path:        "/v1/orders/{orderID}/payment/remaining",
		Summary:     "Outstanding underpayment totals",
		Tags:        []string{"orders", "payments"},
		Security:    nodeAuthSecurity,
	}, func(ctx context.Context, hi *in) (*nodeDataOutput, error) {
		rawURL := "/v1/orders/" + url.PathEscape(hi.OrderID) + "/payment/remaining"
		req := nodeBridgeRequestWithVars(ctx, http.MethodGet, rawURL, nil, map[string]string{"orderID": hi.OrderID})
		rr := httptest.NewRecorder()
		g.handleGETPaymentRemaining(rr, req)
		data, err := nodeBridgeSuccessData(rr)
		if err != nil {
			return nil, err
		}
		return &nodeDataOutput{Body: data}, nil
	})
}

func (g *Gateway) registerOrderPaymentCancelPartialPost(api huma.API) {
	type in struct {
		OrderID string `path:"orderID" doc:"Order ID."`
	}
	huma.Register(api, huma.Operation{
		OperationID: "orders-post-payment-cancel-partial",
		Method:      http.MethodPost,
		Path:        "/v1/orders/{orderID}/payment/cancel-partial",
		Summary:     "Revert partial-payment watch / refund leftover",
		Tags:        []string{"orders", "payments"},
		Security:    nodeAuthSecurity,
	}, func(ctx context.Context, hi *in) (*nodeDataOutput, error) {
		rawURL := "/v1/orders/" + url.PathEscape(hi.OrderID) + "/payment/cancel-partial"
		req := nodeBridgeRequestWithVars(ctx, http.MethodPost, rawURL, nil, map[string]string{"orderID": hi.OrderID})
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		g.handlePOSTCancelPartialPayment(rr, req)
		data, err := nodeBridgeSuccessData(rr)
		if err != nil {
			return nil, err
		}
		return &nodeDataOutput{Body: data}, nil
	})
}

func (g *Gateway) registerOrderPaymentWatchDelete(api huma.API) {
	type in struct {
		OrderID string `path:"orderID" doc:"Order ID."`
	}
	huma.Register(api, huma.Operation{
		OperationID: "orders-delete-payment-watch",
		Method:      http.MethodDelete,
		Path:        "/v1/orders/{orderID}/payment/watch",
		Summary:     "Stop address watcher for UX tear-down",
		Tags:        []string{"orders", "payments"},
		Security:    nodeAuthSecurity,
	}, func(ctx context.Context, hi *in) (*nodeDataOutput, error) {
		rawURL := "/v1/orders/" + url.PathEscape(hi.OrderID) + "/payment/watch"
		req := nodeBridgeRequestWithVars(ctx, http.MethodDelete, rawURL, nil, map[string]string{"orderID": hi.OrderID})
		rr := httptest.NewRecorder()
		g.handleDELETEPaymentWatch(rr, req)
		data, err := nodeBridgeSuccessData(rr)
		if err != nil {
			return nil, err
		}
		return &nodeDataOutput{Body: data}, nil
	})
}

func (g *Gateway) registerGuestOrdersListAuth(api huma.API) {
	type q struct {
		State    string `query:"state"`
		Page     string `query:"page"`
		PageSize string `query:"pageSize"`
	}
	huma.Register(api, huma.Operation{
		OperationID: "guest-orders-list-auth",
		Method:      http.MethodGet,
		Path:        "/v1/guest/orders",
		Summary:     "Seller-visible guest-checkout orders",
		Tags:        []string{"orders", "guest"},
		Security:    nodeAuthSecurity,
	}, func(ctx context.Context, hq *q) (*nodeDataOutput, error) {
		v := url.Values{}
		if hq.State != "" {
			v.Set("state", hq.State)
		}
		if hq.Page != "" {
			v.Set("page", hq.Page)
		}
		if hq.PageSize != "" {
			v.Set("pageSize", hq.PageSize)
		}
		rawURL := "/v1/guest/orders"
		if qs := v.Encode(); qs != "" {
			rawURL += "?" + qs
		}
		req := nodeBridgeRequest(ctx, http.MethodGet, rawURL, nil)
		rr := httptest.NewRecorder()
		g.handleGETGuestOrders(rr, req)
		data, err := nodeBridgeSuccessData(rr)
		if err != nil {
			return nil, err
		}
		return &nodeDataOutput{Body: data}, nil
	})
}

func (g *Gateway) registerGuestOrderShip(api huma.API) {
	type in struct {
		Token string          `path:"token" doc:"Guest checkout token."`
		Body  json.RawMessage `json:",omitempty"`
	}
	huma.Register(api, huma.Operation{
		OperationID: "guest-orders-ship-token",
		Method:      http.MethodPut,
		Path:        "/v1/guest/orders/{token}/ship",
		Summary:     "Mark guest order shipped with tracking payload",
		Tags:        []string{"orders", "guest"},
		Security:    nodeAuthSecurity,
	}, func(ctx context.Context, hi *in) (*nodeNoContentOutput, error) {
		rawURL := "/v1/guest/orders/" + url.PathEscape(hi.Token) + "/ship"
		req := nodeBridgeRequestWithVars(ctx, http.MethodPut, rawURL, bytes.NewReader(hi.Body), map[string]string{"token": hi.Token})
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		g.handleShipGuestOrder(rr, req)
		if err := nodeBridgeNoContent(rr); err != nil {
			return nil, err
		}
		return nil, nil
	})
}

func (g *Gateway) registerGuestOrderComplete(api huma.API) {
	type in struct {
		Token string `path:"token" doc:"Guest checkout token."`
	}
	huma.Register(api, huma.Operation{
		OperationID: "guest-orders-complete-token",
		Method:      http.MethodPut,
		Path:        "/v1/guest/orders/{token}/complete",
		Summary:     "Manually finalize guest-funded order",
		Tags:        []string{"orders", "guest"},
		Security:    nodeAuthSecurity,
	}, func(ctx context.Context, hi *in) (*nodeNoContentOutput, error) {
		rawURL := "/v1/guest/orders/" + url.PathEscape(hi.Token) + "/complete"
		req := nodeBridgeRequestWithVars(ctx, http.MethodPut, rawURL, nil, map[string]string{"token": hi.Token})
		rr := httptest.NewRecorder()
		g.handleCompleteGuestOrder(rr, req)
		if err := nodeBridgeNoContent(rr); err != nil {
			return nil, err
		}
		return nil, nil
	})
}

func (g *Gateway) registerAnalyticsStatsGet(api huma.API) {
	type q struct {
		Days string `query:"days"`
	}
	huma.Register(api, huma.Operation{
		OperationID: "analytics-get-stats",
		Method:      http.MethodGet,
		Path:        "/v1/analytics/stats",
		Summary:     "Authenticated seller analytics rollup",
		Tags:        []string{"analytics"},
		Security:    nodeAuthSecurity,
	}, func(ctx context.Context, hq *q) (*nodeDataOutput, error) {
		rawURL := "/v1/analytics/stats"
		if hq.Days != "" {
			rawURL += "?days=" + url.QueryEscape(hq.Days)
		}
		req := nodeBridgeRequest(ctx, http.MethodGet, rawURL, nil)
		rr := httptest.NewRecorder()
		g.handleGETAnalyticsStats(rr, req)
		data, err := nodeBridgeSuccessData(rr)
		if err != nil {
			return nil, err
		}
		return &nodeDataOutput{Body: data}, nil
	})
}

func (g *Gateway) registerGuestOrderPostPublic(api huma.API) {
	type in struct {
		Body json.RawMessage `json:",omitempty"`
	}
	huma.Register(api, huma.Operation{
		OperationID: "guest-orders-post-public",
		Method:      http.MethodPost,
		Path:        "/v1/guest/orders",
		Summary:     "Public guest checkout initiation (skip rate-limit middleware)",
		Tags:        []string{"orders", "guest"},
	}, func(ctx context.Context, hi *in) (*nodeDataOutput, error) {
		req := nodeBridgeRequest(ctx, http.MethodPost, "/v1/guest/orders", bytes.NewReader(hi.Body))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		g.handlePOSTGuestOrder(rr, req)
		data, err := nodeBridgeSuccessData(rr)
		if err != nil {
			return nil, err
		}
		return &nodeDataOutput{Body: data}, nil
	})
}

func (g *Gateway) registerGuestOrderGetPublic(api huma.API) {
	type in struct {
		Token string `path:"token" doc:"Guest checkout token."`
	}
	huma.Register(api, huma.Operation{
		OperationID: "guest-orders-get-public",
		Method:      http.MethodGet,
		Path:        "/v1/guest/orders/{token}",
		Summary:     "Public guest order tracker",
		Tags:        []string{"orders", "guest"},
	}, func(ctx context.Context, hi *in) (*nodeDataOutput, error) {
		rawURL := "/v1/guest/orders/" + url.PathEscape(hi.Token)
		req := nodeBridgeRequestWithVars(ctx, http.MethodGet, rawURL, nil, map[string]string{"token": hi.Token})
		rr := httptest.NewRecorder()
		g.handleGETGuestOrder(rr, req)
		data, err := nodeBridgeSuccessData(rr)
		if err != nil {
			return nil, err
		}
		return &nodeDataOutput{Body: data}, nil
	})
}

func (g *Gateway) registerPaymentMethodsGet(api huma.API) {
	type in struct {
		PeerID string `path:"peerID" doc:"Storefront seller peer scope."`
	}
	huma.Register(api, huma.Operation{
		OperationID: "payment-methods-get-by-peer-id",
		Method:      http.MethodGet,
		Path:        "/v1/payment-methods/{peerID}",
		Summary:     "Public storefront payment rails summary",
		Tags:        []string{"payments"},
	}, func(ctx context.Context, hi *in) (*nodeDataOutput, error) {
		rawURL := "/v1/payment-methods/" + url.PathEscape(hi.PeerID)
		req := nodeBridgeRequestWithVars(ctx, http.MethodGet, rawURL, nil, map[string]string{"peerID": hi.PeerID})
		rr := httptest.NewRecorder()
		g.handleGETPaymentMethods(rr, req)
		data, err := nodeBridgeSuccessData(rr)
		if err != nil {
			return nil, err
		}
		return &nodeDataOutput{Body: data}, nil
	})
}

func (g *Gateway) registerAnalyticsShopEventsPost(api huma.API) {
	type in struct {
		PeerID string          `path:"peerID" doc:"Peer scope baked into storefront URL."`
		Body   json.RawMessage `json:",omitempty"`
	}
	huma.Register(api, huma.Operation{
		OperationID: "analytics-shop-post-events-public",
		Method:      http.MethodPost,
		Path:        "/v1/analytics/{peerID}/events",
		Summary:     "Fan-in analytics events without auth",
		Tags:        []string{"analytics"},
	}, func(ctx context.Context, hi *in) (*nodeNoContentOutput, error) {
		rawURL := "/v1/analytics/" + url.PathEscape(hi.PeerID) + "/events"
		req := nodeBridgeRequestWithVars(ctx, http.MethodPost, rawURL, bytes.NewReader(hi.Body), map[string]string{"peerID": hi.PeerID})
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		g.handlePOSTAnalyticsEvent(rr, req)
		if err := nodeBridgeNoContent(rr); err != nil {
			return nil, err
		}
		return nil, nil
	})
}
