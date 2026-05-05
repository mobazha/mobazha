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

func (g *Gateway) registerNodeHumaWalletOperations(api huma.API) {
	g.registerWalletSpend(api)
	g.registerWalletMnemonic(api)
	g.registerWalletCurrencies(api)
	g.registerNodeHumaReceivingAccountOperations(api)
}

// --- POST /v1/wallet/spend ---

type walletSpendInput struct {
	Body json.RawMessage
}

func (g *Gateway) registerWalletSpend(api huma.API) {
	huma.Register(api, huma.Operation{
		OperationID: "wallet-spend",
		Method:      http.MethodPost,
		Path:        "/v1/wallet/spend",
		Summary:     "Send cryptocurrency from wallet",
		Tags:        []string{"wallet"},
		Security:    nodeAuthSecurity,
	}, func(ctx context.Context, in *walletSpendInput) (*nodeDataOutput, error) {
		req := nodeBridgeRequest(ctx, http.MethodPost, "/v1/wallet/spend", bytes.NewReader(in.Body))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		g.handlePOSTSpend(rr, req)
		data, err := nodeBridgeSuccessData(rr)
		if err != nil {
			return nil, err
		}
		return &nodeDataOutput{Body: data}, nil
	})
}

// --- GET /v1/wallet/mnemonic ---

func (g *Gateway) registerWalletMnemonic(api huma.API) {
	huma.Register(api, huma.Operation{
		OperationID: "wallet-get-mnemonic",
		Method:      http.MethodGet,
		Path:        "/v1/wallet/mnemonic",
		Summary:     "Get wallet mnemonic seed phrase",
		Tags:        []string{"wallet"},
		Security:    nodeAuthSecurity,
	}, func(ctx context.Context, _ *struct{}) (*nodeDataOutput, error) {
		req := nodeBridgeRequest(ctx, http.MethodGet, "/v1/wallet/mnemonic", nil)
		rr := httptest.NewRecorder()
		g.handleGETMnemonic(rr, req)
		data, err := nodeBridgeSuccessData(rr)
		if err != nil {
			return nil, err
		}
		return &nodeDataOutput{Body: data}, nil
	})
}

// --- GET /v1/wallet/currencies ---

func (g *Gateway) registerWalletCurrencies(api huma.API) {
	huma.Register(api, huma.Operation{
		OperationID: "wallet-get-currencies",
		Method:      http.MethodGet,
		Path:        "/v1/wallet/currencies",
		Summary:     "List supported wallet currencies",
		Tags:        []string{"wallet"},
		Security:    nodeAuthSecurity,
	}, func(ctx context.Context, _ *struct{}) (*nodeDataOutput, error) {
		req := nodeBridgeRequest(ctx, http.MethodGet, "/v1/wallet/currencies", nil)
		rr := httptest.NewRecorder()
		g.handleGETCurrencies(rr, req)
		data, err := nodeBridgeSuccessData(rr)
		if err != nil {
			return nil, err
		}
		return &nodeDataOutput{Body: data}, nil
	})
}
