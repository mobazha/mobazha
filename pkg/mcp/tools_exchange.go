package mcp

import (
	"context"
	"fmt"
	"net/url"

	gomcp "github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func exchangeToolRegistrars(bf BridgeFactory) []ToolRegistrar {
	return []ToolRegistrar{
		{
			Name: "exchange_rates_get",
			Tool: gomcp.NewTool("exchange_rates_get",
				gomcp.WithDescription(
					"Get current cryptocurrency exchange rates for a specific fiat currency. "+
						"Returns exchange rates for all supported cryptocurrencies against the specified currency. "+
						"Use when asked about crypto prices, conversion rates, or how much a cryptocurrency is worth.",
				),
				gomcp.WithString("currency",
					gomcp.Description("Fiat currency code (e.g., 'USD', 'EUR', 'CNY'). Omit for all currencies."),
				),
			),
			Handler: makeExchangeRates(bf),
		},
	}
}

func makeExchangeRates(bf BridgeFactory) server.ToolHandlerFunc {
	return func(ctx context.Context, req gomcp.CallToolRequest) (*gomcp.CallToolResult, error) {
		currency := req.GetString("currency", "")
		var path string
		if currency != "" {
			path = fmt.Sprintf("/v1/exchange-rates/%s", url.PathEscape(currency))
		} else {
			path = "/v1/exchange-rates"
		}
		bridge := bf(req)
		code, body, err := bridge.Call(ctx, "GET", path, nil, nil)
		return HandleBridgeResult(code, body, err)
	}
}
