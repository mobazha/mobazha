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

// registerNodeHumaMiscAdminOperations registers admin misc operations
// (crypto, moderators, blocklist) that require authentication.
func (g *Gateway) registerNodeHumaMiscAdminOperations(api huma.API) {
	type jsonBody struct {
		Body json.RawMessage `json:",omitempty"`
	}

	type peerIDPath struct {
		PeerID string `path:"peerID"`
	}

	type moderatorsQuery struct {
		Async   bool   `query:"async"`
		AsyncID string `query:"asyncID"`
		Include string `query:"include"`
	}

	buildModeratorsURL := func(q moderatorsQuery) string {
		v := url.Values{}
		if q.Async {
			v.Set("async", "true")
		}
		if q.AsyncID != "" {
			v.Set("asyncID", q.AsyncID)
		}
		if q.Include != "" {
			v.Set("include", q.Include)
		}
		raw := "/v1/moderators"
		if enc := v.Encode(); enc != "" {
			raw += "?" + enc
		}
		return raw
	}

	huma.Register(api, huma.Operation{
		OperationID: "crypto-sign-post",
		Method:      http.MethodPost,
		Path:        "/v1/crypto/sign",
		Summary:     "Sign a payload with seller keys",
		Tags:        []string{"crypto"},
		Security:    nodeAuthSecurity,
	}, func(ctx context.Context, in *jsonBody) (*nodeDataOutput, error) {
		req := nodeBridgeRequest(ctx, http.MethodPost, "/v1/crypto/sign", bytes.NewReader(in.Body))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		g.handlePOSTSignMessage(rr, req)
		data, err := nodeBridgeSuccessData(rr)
		if err != nil {
			return nil, err
		}
		return &nodeDataOutput{Body: data}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "crypto-verify-post",
		Method:      http.MethodPost,
		Path:        "/v1/crypto/verify",
		Summary:     "Verify a detached signature",
		Tags:        []string{"crypto"},
		Security:    nodeAuthSecurity,
	}, func(ctx context.Context, in *jsonBody) (*nodeDataOutput, error) {
		req := nodeBridgeRequest(ctx, http.MethodPost, "/v1/crypto/verify", bytes.NewReader(in.Body))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		g.handlePOSTVerifyMessage(rr, req)
		data, err := nodeBridgeSuccessData(rr)
		if err != nil {
			return nil, err
		}
		return &nodeDataOutput{Body: data}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "crypto-hash-post",
		Method:      http.MethodPost,
		Path:        "/v1/crypto/hash",
		Summary:     "Stable hash helper for payloads",
		Tags:        []string{"crypto"},
		Security:    nodeAuthSecurity,
	}, func(ctx context.Context, in *jsonBody) (*nodeDataOutput, error) {
		req := nodeBridgeRequest(ctx, http.MethodPost, "/v1/crypto/hash", bytes.NewReader(in.Body))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		g.handlePOSTHashMessage(rr, req)
		data, err := nodeBridgeSuccessData(rr)
		if err != nil {
			return nil, err
		}
		return &nodeDataOutput{Body: data}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "moderators-post",
		Method:      http.MethodPost,
		Path:        "/v1/moderators",
		Summary:     "Declare self as dispute moderator candidate",
		Tags:        []string{"moderators"},
		Security:    nodeAuthSecurity,
	}, func(ctx context.Context, in *jsonBody) (*nodeDataOutput, error) {
		req := nodeBridgeRequest(ctx, http.MethodPost, "/v1/moderators", bytes.NewReader(in.Body))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		g.handleSetModerator(rr, req)
		data, err := nodeBridgeSuccessData(rr)
		if err != nil {
			return nil, err
		}
		return &nodeDataOutput{Body: data}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "moderators-delete",
		Method:      http.MethodDelete,
		Path:        "/v1/moderators",
		Summary:     "Withdraw moderator advertisement",
		Tags:        []string{"moderators"},
		Security:    nodeAuthSecurity,
	}, func(ctx context.Context, _ *struct{}) (*nodeDataOutput, error) {
		req := nodeBridgeRequest(ctx, http.MethodDelete, "/v1/moderators", http.NoBody)
		rr := httptest.NewRecorder()
		g.handleUnsetModerator(rr, req)
		data, err := nodeBridgeSuccessData(rr)
		if err != nil {
			return nil, err
		}
		return &nodeDataOutput{Body: data}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "moderators-get",
		Method:      http.MethodGet,
		Path:        "/v1/moderators",
		Summary:     "Enumerate candidate moderators (async/sync)",
		Tags:        []string{"moderators"},
		Security:    nodeAuthSecurity,
	}, func(ctx context.Context, q *moderatorsQuery) (*nodeDataOutput, error) {
		raw := buildModeratorsURL(*q)
		req := nodeBridgeRequest(ctx, http.MethodGet, raw, nil)
		rr := httptest.NewRecorder()
		g.handleGetModerators(rr, req)
		data, err := nodeBridgeSuccessData(rr)
		if err != nil {
			return nil, err
		}
		return &nodeDataOutput{Body: data}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "blocklist-peer-id-put",
		Method:      http.MethodPut,
		Path:        "/v1/blocklist/{peerID}",
		Summary:     "Block storefront peer IDs",
		Tags:        []string{"moderation"},
		Security:    nodeAuthSecurity,
	}, func(ctx context.Context, in *peerIDPath) (*nodeDataOutput, error) {
		raw := "/v1/blocklist/" + url.PathEscape(in.PeerID)
		req := nodeBridgeRequestWithVars(ctx, http.MethodPut, raw, http.NoBody, map[string]string{"peerID": in.PeerID})
		rr := httptest.NewRecorder()
		g.handleBlockNode(rr, req)
		data, err := nodeBridgeSuccessData(rr)
		if err != nil {
			return nil, err
		}
		return &nodeDataOutput{Body: data}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "blocklist-peer-id-delete",
		Method:      http.MethodDelete,
		Path:        "/v1/blocklist/{peerID}",
		Summary:     "Remove blocklist entry",
		Tags:        []string{"moderation"},
		Security:    nodeAuthSecurity,
	}, func(ctx context.Context, in *peerIDPath) (*nodeDataOutput, error) {
		raw := "/v1/blocklist/" + url.PathEscape(in.PeerID)
		req := nodeBridgeRequestWithVars(ctx, http.MethodDelete, raw, http.NoBody, map[string]string{"peerID": in.PeerID})
		rr := httptest.NewRecorder()
		g.handleUnBlockNode(rr, req)
		data, err := nodeBridgeSuccessData(rr)
		if err != nil {
			return nil, err
		}
		return &nodeDataOutput{Body: data}, nil
	})

}

// registerNodeHumaMiscPublicOperations registers public misc operations
// (exchange rates, peers) that do not require authentication.
func (g *Gateway) registerNodeHumaMiscPublicOperations(api huma.API) {
	type exchangeRatesQuery struct {
		Refresh bool `query:"refresh"`
	}

	buildExchangeRatesQuery := func(in exchangeRatesQuery) string {
		if in.Refresh {
			return "?refresh=true"
		}
		return ""
	}

	type fxByCode struct {
		CurrencyCode string `path:"currencyCode"`
		exchangeRatesQuery
	}

	huma.Register(api, huma.Operation{
		OperationID: "exchange-rates-currency-code-get",
		Method:      http.MethodGet,
		Path:        "/v1/exchange-rates/{currencyCode}",
		Summary:     "Retrieve FX matrix for denominated currency",
		Tags:        []string{"wallet"},
	}, func(ctx context.Context, q *fxByCode) (*nodeDataOutput, error) {
		raw := "/v1/exchange-rates/" + url.PathEscape(q.CurrencyCode)
		raw += buildExchangeRatesQuery(q.exchangeRatesQuery)
		req := nodeBridgeRequestWithVars(ctx, http.MethodGet, raw, nil, map[string]string{"currencyCode": q.CurrencyCode})
		rr := httptest.NewRecorder()
		g.handleGETExchangeRates(rr, req)
		data, err := nodeBridgeSuccessData(rr)
		if err != nil {
			return nil, err
		}
		return &nodeDataOutput{Body: data}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "exchange-rates-get",
		Method:      http.MethodGet,
		Path:        "/v1/exchange-rates",
		Summary:     "Retrieve FX matrix for reserve currency baseline",
		Tags:        []string{"wallet"},
	}, func(ctx context.Context, q *exchangeRatesQuery) (*nodeDataOutput, error) {
		raw := "/v1/exchange-rates" + buildExchangeRatesQuery(*q)
		req := nodeBridgeRequest(ctx, http.MethodGet, raw, nil)
		rr := httptest.NewRecorder()
		g.handleGETExchangeRates(rr, req)
		data, err := nodeBridgeSuccessData(rr)
		if err != nil {
			return nil, err
		}
		return &nodeDataOutput{Body: data}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "peers-get",
		Method:      http.MethodGet,
		Path:        "/v1/peers",
		Summary:     "Enumerate libp2p peers (standalone only)",
		Tags:        []string{"system"},
	}, func(ctx context.Context, _ *struct{}) (*nodeDataOutput, error) {
		req := nodeBridgeRequest(ctx, http.MethodGet, "/v1/peers", nil)
		rr := httptest.NewRecorder()
		g.handleGETPeers(rr, req)
		data, err := nodeBridgeSuccessData(rr)
		if err != nil {
			return nil, err
		}
		return &nodeDataOutput{Body: data}, nil
	})
}
