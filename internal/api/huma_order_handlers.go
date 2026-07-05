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

// registerNodeHumaOrderPublicOperations registers public order/checkout ops
// accessible without authentication (buyer storefront browsing).
func (g *Gateway) registerNodeHumaOrderPublicOperations(api huma.API) {
	g.registerGuestOrderQuotePublic(api)
	g.registerGuestOrderPostPublic(api)
	g.registerGuestOrderGetPublic(api)
	g.registerPaymentMethodsGet(api)
	g.registerPGPKeyGet(api) // PM-3a: buyer fetches vendor public key to encrypt shipping address
	g.registerAnalyticsShopEventsPost(api)
}

// registerNodeHumaOrderAdminOperations registers authenticated order lifecycle,
// checkout, and seller analytics ops.
func (g *Gateway) registerNodeHumaOrderAdminOperations(api huma.API) {
	g.registerOrdersRWATokenPaymentInfo(api)
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
	g.registerOrdersPostSupplyQuote(api)
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

	// Phase PS / B1: unified payment session read endpoint.
	g.registerOrderPaymentSessionGet(api)
	g.registerOrderPaymentSelectionQuotePost(api)
	g.registerOrderPaymentSessionPost(api)
	g.registerOrderRefundAddressPost(api)
	g.registerOrderSettlementActionPost(api)
	g.registerOrderSettlementActionStatusGet(api)

	g.registerGuestOrdersListAuth(api)
	g.registerGuestOrderShip(api)
	g.registerGuestOrderComplete(api)
	g.registerGuestOrderAdminDetail(api) // PM-3a: admin detail with shipping address ciphertext
	g.registerPGPKeyPut(api)             // PM-3a: set vendor PGP public key
	g.registerPGPKeyDelete(api)          // PM-3a: remove vendor PGP public key

	g.registerAnalyticsStatsGet(api)
}

// registerOrderPaymentSelectionQuotePost registers the immutable Deal payment
// asset and conversion quote endpoint.
func (g *Gateway) registerOrderPaymentSelectionQuotePost(api huma.API) {
	type in struct {
		OrderID string          `path:"orderID" doc:"Order ID."`
		Body    json.RawMessage `json:",omitempty"`
	}
	huma.Register(api, huma.Operation{
		OperationID: "orders-post-payment-selection-quote",
		Method:      http.MethodPost,
		Path:        "/v1/orders/{orderID}/payment-selection-quotes",
		Summary:     "Create immutable Deal payment-selection quote",
		Description: "Freezes the signed pricing amount, canonical payment asset, conversion rate, numeric provider/platform costs, target amount, policy version and expiry before PaymentSession provisioning.",
		Tags:        []string{"orders", "payments"},
		Security:    nodeAuthSecurity,
	}, func(ctx context.Context, hi *in) (*nodeDataOutput, error) {
		rawURL := "/v1/orders/" + url.PathEscape(hi.OrderID) + "/payment-selection-quotes"
		req := nodeBridgeRequestWithVars(ctx, http.MethodPost, rawURL, bytes.NewReader(hi.Body), map[string]string{"orderID": hi.OrderID})
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		g.handlePOSTOrderPaymentSelectionQuote(rr, req)
		data, err := nodeBridgeSuccessData(rr)
		if err != nil {
			return nil, err
		}
		return &nodeDataOutput{Body: data}, nil
	})
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

func (g *Gateway) registerOrdersRWATokenPaymentInfo(api huma.API) {
	type in struct {
		OrderID string          `path:"orderID" doc:"Order ID."`
		Body    json.RawMessage `json:",omitempty"`
	}
	huma.Register(api, huma.Operation{
		OperationID: "orders-post-rwa-token-payment-info",
		Method:      http.MethodPost,
		Path:        "/v1/orders/{orderID}/rwa-token/payment-info",
		Summary:     "Get RWA token identity payment info",
		Description: "Returns buyer and vendor chain identity addresses for RWA token purchases.",
		Tags:        []string{"orders"},
		Security:    nodeAuthSecurity,
	}, func(ctx context.Context, hi *in) (*nodeDataOutput, error) {
		rawURL := "/v1/orders/" + url.PathEscape(hi.OrderID) + "/rwa-token/payment-info"
		req := nodeBridgeRequestWithVars(ctx, http.MethodPost, rawURL, bytes.NewReader(hi.Body), map[string]string{"orderID": hi.OrderID})
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		g.handleGetRWATokenPaymentInfoRequest(rr, req)
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
		Summary:     "Legacy completion payout instructions",
		Description: "Compatibility endpoint for client-signed moderated completion flows. " +
			"backend-managed moderated completion stays on the backend-owned completion path " +
			"and does not use this instructions contract as its primary entrypoint.",
		Tags:     []string{"orders"},
		Security: nodeAuthSecurity,
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

func (g *Gateway) registerOrdersPostSupplyQuote(api huma.API) {
	type in struct {
		Body json.RawMessage `json:",omitempty"`
	}
	huma.Register(api, huma.Operation{
		OperationID: "orders-post-supply-quote",
		Method:      http.MethodPost,
		Path:        "/v1/orders/supply-quote",
		Summary:     "Authenticated checkout supply preflight",
		Tags:        []string{"orders"},
		Security:    nodeAuthSecurity,
	}, func(ctx context.Context, hi *in) (*nodeDataOutput, error) {
		req := nodeBridgeRequest(ctx, http.MethodPost, "/v1/orders/supply-quote", bytes.NewReader(hi.Body))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		g.handlePOSTOrderSupplyQuote(rr, req)
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

// registerGuestOrdersListAuth, registerGuestOrderShip, registerGuestOrderComplete
// moved to huma_guest_payment_handlers.go (OP-1.3 Step 4a — build-neutral)

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

// registerOrderPaymentSessionGet registers GET /v1/orders/{orderID}/payment-session.
//
// Phase PS / B1: read-only projection — returns a unified PaymentSession view built
// from existing order, payment, and fiat metadata (no new DB table required).
// Create/refresh provisioning will be added in Phase B Step 2 + Step 3.
func (g *Gateway) registerOrderPaymentSessionGet(api huma.API) {
	type in struct {
		OrderID string `path:"orderID" doc:"Order ID."`
	}
	huma.Register(api, huma.Operation{
		OperationID: "orders-get-payment-session",
		Method:      http.MethodGet,
		Path:        "/v1/orders/{orderID}/payment-session",
		Summary:     "Unified payment session view for an order",
		Description: "Returns a PaymentSession projection built from existing order, payment, and " +
			"fiat metadata. Settlement modes include address_monitored (UTXO, Monero, backend-managed EVM, " +
			"and Solana escrow when persisted), escrow_v1 (legacy EVM / Solana / TRON flows that require buyer-signed escrow), " +
			"and provider_checkout (Stripe/PayPal).",
		Tags:     []string{"orders", "payments"},
		Security: nodeAuthSecurity,
	}, func(ctx context.Context, hi *in) (*nodeDataOutput, error) {
		rawURL := "/v1/orders/" + url.PathEscape(hi.OrderID) + "/payment-session"
		req := nodeBridgeRequestWithVars(ctx, http.MethodGet, rawURL, nil, map[string]string{"orderID": hi.OrderID})
		rr := httptest.NewRecorder()
		g.handleGETOrderPaymentSession(rr, req)
		data, err := nodeBridgeSuccessData(rr)
		if err != nil {
			return nil, err
		}
		return &nodeDataOutput{Body: data}, nil
	})
}

// registerOrderPaymentSessionPost registers POST /v1/orders/{orderID}/payment-session.
//
// Phase PS / B5: provisions checkout via PaymentSessionService (canonical paymentCoin):
// fiat (fiatAmountCents, etc.) and crypto (optional refundAddress/payerAddress/moderator).
func (g *Gateway) registerOrderPaymentSessionPost(api huma.API) {
	type in struct {
		OrderID string          `path:"orderID" doc:"Order ID."`
		Body    json.RawMessage `json:",omitempty"`
	}
	huma.Register(api, huma.Operation{
		OperationID: "orders-post-payment-session",
		Method:      http.MethodPost,
		Path:        "/v1/orders/{orderID}/payment-session",
		Summary:     "Provision unified payment session",
		Description: "Creates or idempotently returns a PaymentSession. Fiat: canonical " +
			"paymentCoin fiat:{provider}:{currency}, fiatAmountCents (>0), and optional PayPal return/cancel URLs. " +
			"Crypto: optional refundAddress/payerAddress/moderator; buyers should declare refundAddress before paying. " +
			"Deal cross-currency requests must include paymentSelectionQuoteID from the immutable quote endpoint.",
		Tags:     []string{"orders", "payments"},
		Security: nodeAuthSecurity,
	}, func(ctx context.Context, hi *in) (*nodeDataOutput, error) {
		rawURL := "/v1/orders/" + url.PathEscape(hi.OrderID) + "/payment-session"
		req := nodeBridgeRequestWithVars(ctx, http.MethodPost, rawURL, bytes.NewReader(hi.Body), map[string]string{"orderID": hi.OrderID})
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		g.handlePOSTOrderPaymentSession(rr, req)
		data, err := nodeBridgeSuccessData(rr)
		if err != nil {
			return nil, err
		}
		return &nodeDataOutput{Body: data}, nil
	})
}

// registerOrderRefundAddressPost registers POST /v1/orders/{orderID}/refund-address.
func (g *Gateway) registerOrderRefundAddressPost(api huma.API) {
	type in struct {
		OrderID string          `path:"orderID" doc:"Order ID."`
		Body    json.RawMessage `json:",omitempty"`
	}
	huma.Register(api, huma.Operation{
		OperationID: "orders-post-refund-address",
		Method:      http.MethodPost,
		Path:        "/v1/orders/{orderID}/refund-address",
		Summary:     "Set buyer refund wallet address",
		Description: "Validates and persists the buyer-controlled crypto refund destination. " +
			"Requires buyer role. paymentCoin is optional when the order already has a selected coin.",
		Tags:     []string{"orders", "payments"},
		Security: nodeAuthSecurity,
	}, func(ctx context.Context, hi *in) (*nodeDataOutput, error) {
		rawURL := "/v1/orders/" + url.PathEscape(hi.OrderID) + "/refund-address"
		req := nodeBridgeRequestWithVars(ctx, http.MethodPost, rawURL, bytes.NewReader(hi.Body), map[string]string{"orderID": hi.OrderID})
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		g.handlePOSTOrderRefundAddress(rr, req)
		data, err := nodeBridgeSuccessData(rr)
		if err != nil {
			return nil, err
		}
		return &nodeDataOutput{Body: data}, nil
	})
}

// registerOrderSettlementActionPost registers POST /v1/orders/{orderID}/settlement-actions/{action}.
//
// Backend settlement intents: confirm, cancel, seller-decline-refund, complete, dispute-release.
func (g *Gateway) registerOrderSettlementActionPost(api huma.API) {
	type in struct {
		OrderID string          `path:"orderID" doc:"Order ID."`
		Action  string          `path:"action" doc:"Settlement intent: confirm, cancel, seller-decline-refund, complete, or dispute-release."`
		Body    json.RawMessage `json:",omitempty"`
	}
	huma.Register(api, huma.Operation{
		OperationID: "orders-post-settlement-action",
		Method:      http.MethodPost,
		Path:        "/v1/orders/{orderID}/settlement-actions/{action}",
		Summary:     "Execute backend settlement action",
		Description: "Runs backend-submitted settlement for crypto orders (managed EVM, Solana Anchor, UTXO sync). " +
			"Supported actions: confirm, cancel, seller-decline-refund, complete, dispute-release. " +
			"Client-signed legacy chains use instruction endpoints. Fiat orders return 400. Optional body: payoutAddress.",
		Tags:     []string{"orders", "payments"},
		Security: nodeAuthSecurity,
	}, func(ctx context.Context, hi *in) (*nodeDataOutput, error) {
		rawURL := "/v1/orders/" + url.PathEscape(hi.OrderID) + "/settlement-actions/" + url.PathEscape(hi.Action)
		req := nodeBridgeRequestWithVars(ctx, http.MethodPost, rawURL, bytes.NewReader(hi.Body), map[string]string{
			"orderID": hi.OrderID,
			"action":  hi.Action,
		})
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		g.handlePOSTOrderSettlementAction(rr, req)
		data, err := nodeBridgeSuccessData(rr)
		if err != nil {
			return nil, err
		}
		return &nodeDataOutput{Body: data}, nil
	})
}

// registerOrderSettlementActionStatusGet registers
// GET /v1/orders/{orderID}/settlement-actions/{action}/status?actionId=...
func (g *Gateway) registerOrderSettlementActionStatusGet(api huma.API) {
	type in struct {
		OrderID  string `path:"orderID" doc:"Order ID."`
		Action   string `path:"action" doc:"Settlement intent: confirm, cancel, seller-decline-refund, complete, or dispute-release."`
		ActionID string `query:"actionId" required:"true" doc:"Opaque settlement action poll key returned by POST settlement-actions."`
	}
	huma.Register(api, huma.Operation{
		OperationID: "orders-get-settlement-action-status",
		Method:      http.MethodGet,
		Path:        "/v1/orders/{orderID}/settlement-actions/{action}/status",
		Summary:     "Read unified settlement action status",
		Description: "Returns the latest status for a previously issued backend settlement action. " +
			"backend-managed flows expose relay task correlation and confirmations through this endpoint.",
		Tags:     []string{"orders", "payments"},
		Security: nodeAuthSecurity,
	}, func(ctx context.Context, hi *in) (*nodeDataOutput, error) {
		rawURL := "/v1/orders/" + url.PathEscape(hi.OrderID) + "/settlement-actions/" + url.PathEscape(hi.Action) +
			"/status?actionId=" + url.QueryEscape(hi.ActionID)
		req := nodeBridgeRequestWithVars(ctx, http.MethodGet, rawURL, nil, map[string]string{
			"orderID": hi.OrderID,
			"action":  hi.Action,
		})
		rr := httptest.NewRecorder()
		g.handleGETOrderSettlementActionStatus(rr, req)
		data, err := nodeBridgeSuccessData(rr)
		if err != nil {
			return nil, err
		}
		return &nodeDataOutput{Body: data}, nil
	})
}

// registerGuestOrderPostPublic, registerGuestOrderGetPublic, registerPaymentMethodsGet
// moved to huma_guest_payment_handlers.go (OP-1.3 Step 4a — build-neutral)

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
