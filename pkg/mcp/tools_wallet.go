package mcp

import (
	"context"

	gomcp "github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func walletToolRegistrars(bf BridgeFactory) []ToolRegistrar {
	return []ToolRegistrar{
		{
			Name: "wallet_get_receiving_accounts",
			Tool: gomcp.NewTool("wallet_get_receiving_accounts",
				gomcp.WithDescription(
					"Get the store's cryptocurrency receiving addresses for all supported chains. "+
						"Returns addresses for each enabled cryptocurrency (e.g., Ethereum, BNB, Solana). "+
						"Use when asked about wallet addresses, how to receive payments, or supported cryptocurrencies.",
				),
			),
			Handler: makeWalletReceivingAccounts(bf),
		},
	}
}

func makeWalletReceivingAccounts(bf BridgeFactory) server.ToolHandlerFunc {
	return func(ctx context.Context, req gomcp.CallToolRequest) (*gomcp.CallToolResult, error) {
		bridge := bf(req)
		code, body, err := bridge.Call(ctx, "GET", "/v1/wallet/receiving-accounts", nil, nil)
		return HandleBridgeResult(code, body, err)
	}
}
