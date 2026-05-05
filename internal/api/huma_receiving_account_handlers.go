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

type receivingAccountBodyInput struct {
	Body json.RawMessage
}

// registerNodeHumaReceivingAccountOperations registers Huma operations for
// ReceivingAccount CRUD. Build-neutral — called by both the full-build
// wallet aggregation and the private_distribution admin aggregation.
func (g *Gateway) registerNodeHumaReceivingAccountOperations(api huma.API) {
	g.registerWalletReceivingAccountsList(api)
	g.registerWalletReceivingAccountsCreate(api)
	g.registerWalletReceivingAccountsUpdate(api)
	g.registerWalletReceivingAccountsDelete(api)
}

func (g *Gateway) registerWalletReceivingAccountsList(api huma.API) {
	huma.Register(api, huma.Operation{
		OperationID: "wallet-list-receiving-accounts",
		Method:      http.MethodGet,
		Path:        "/v1/wallet/receiving-accounts",
		Summary:     "List receiving accounts",
		Tags:        []string{"wallet"},
		Security:    nodeAuthSecurity,
	}, func(ctx context.Context, _ *struct{}) (*nodeDataOutput, error) {
		req := nodeBridgeRequest(ctx, http.MethodGet, "/v1/wallet/receiving-accounts", nil)
		rr := httptest.NewRecorder()
		g.GetReceivingAccounts(rr, req)
		data, err := nodeBridgeSuccessData(rr)
		if err != nil {
			return nil, err
		}
		return &nodeDataOutput{Body: data}, nil
	})
}

func (g *Gateway) registerWalletReceivingAccountsCreate(api huma.API) {
	huma.Register(api, huma.Operation{
		OperationID: "wallet-create-receiving-account",
		Method:      http.MethodPost,
		Path:        "/v1/wallet/receiving-accounts",
		Summary:     "Add a receiving account",
		Tags:        []string{"wallet"},
		Security:    nodeAuthSecurity,
	}, func(ctx context.Context, in *receivingAccountBodyInput) (*nodeDataOutput, error) {
		req := nodeBridgeRequest(ctx, http.MethodPost, "/v1/wallet/receiving-accounts", bytes.NewReader(in.Body))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		g.AddReceivingAccount(rr, req)
		data, err := nodeBridgeSuccessData(rr)
		if err != nil {
			return nil, err
		}
		return &nodeDataOutput{Body: data}, nil
	})
}

func (g *Gateway) registerWalletReceivingAccountsUpdate(api huma.API) {
	huma.Register(api, huma.Operation{
		OperationID: "wallet-update-receiving-account",
		Method:      http.MethodPut,
		Path:        "/v1/wallet/receiving-accounts",
		Summary:     "Update a receiving account",
		Tags:        []string{"wallet"},
		Security:    nodeAuthSecurity,
	}, func(ctx context.Context, in *receivingAccountBodyInput) (*nodeDataOutput, error) {
		req := nodeBridgeRequest(ctx, http.MethodPut, "/v1/wallet/receiving-accounts", bytes.NewReader(in.Body))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		g.UpdateReceivingAccount(rr, req)
		data, err := nodeBridgeSuccessData(rr)
		if err != nil {
			return nil, err
		}
		return &nodeDataOutput{Body: data}, nil
	})
}

type walletDeleteAccountInput struct {
	ID string `path:"id" doc:"Receiving account ID to delete."`
}

func (g *Gateway) registerWalletReceivingAccountsDelete(api huma.API) {
	huma.Register(api, huma.Operation{
		OperationID: "wallet-delete-receiving-account",
		Method:      http.MethodDelete,
		Path:        "/v1/wallet/receiving-accounts/{id}",
		Summary:     "Delete a receiving account",
		Tags:        []string{"wallet"},
		Security:    nodeAuthSecurity,
	}, func(ctx context.Context, in *walletDeleteAccountInput) (*nodeNoContentOutput, error) {
		rawURL := "/v1/wallet/receiving-accounts/" + url.PathEscape(in.ID)
		req := nodeBridgeRequestWithVars(ctx, http.MethodDelete, rawURL, nil, map[string]string{"id": in.ID})
		rr := httptest.NewRecorder()
		g.DeleteReceivingAccount(rr, req)
		if err := nodeBridgeNoContent(rr); err != nil {
			return nil, err
		}
		return nil, nil
	})
}
