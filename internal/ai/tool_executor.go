package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

var toolHTTPClient = &http.Client{
	Timeout: 30 * time.Second,
}

// ToolExecutor calls the local Node API to execute tool functions.
// It uses the same REST API that the MCP Server's HTTPBridge calls.
type ToolExecutor struct {
	baseURL    string
	authToken  string
	httpClient *http.Client
}

// NewToolExecutor creates a tool executor targeting the local node API.
// authToken is the raw Authorization header value (e.g. "Bearer xxx" or "Basic xxx").
func NewToolExecutor(baseURL, authToken string) *ToolExecutor {
	return &ToolExecutor{
		baseURL:    strings.TrimSuffix(baseURL, "/"),
		authToken:  authToken,
		httpClient: toolHTTPClient,
	}
}

// toolRoute maps tool name to HTTP method + API path.
type toolRoute struct {
	Method string
	Path   string
}

var toolRoutes = map[string]func(args map[string]interface{}) toolRoute{
	"listings_list_mine":    func(_ map[string]interface{}) toolRoute { return toolRoute{"GET", "/v1/listings/index"} },
	"listings_get":          func(a map[string]interface{}) toolRoute { return toolRoute{"GET", "/v1/listings/mine/" + sanitizePathParam(a["slug"])} },
	"listings_get_template": func(_ map[string]interface{}) toolRoute { return toolRoute{"GET", "/v1/listings/template"} },
	"listings_create":       func(_ map[string]interface{}) toolRoute { return toolRoute{"POST", "/v1/listings"} },
	"listings_update":       func(_ map[string]interface{}) toolRoute { return toolRoute{"PUT", "/v1/listings"} },
	"listings_delete":       func(a map[string]interface{}) toolRoute { return toolRoute{"DELETE", "/v1/listings/" + sanitizePathParam(a["slug"])} },
	"orders_get_sales":      func(_ map[string]interface{}) toolRoute { return toolRoute{"POST", "/v1/sales"} },
	"orders_get_detail":     func(a map[string]interface{}) toolRoute { return toolRoute{"GET", "/v1/orders/" + sanitizePathParam(a["orderId"])} },
	"orders_confirm":        func(a map[string]interface{}) toolRoute { return toolRoute{"POST", "/v1/orders/" + sanitizePathParam(a["orderId"]) + "/confirm"} },
	"orders_decline":         func(a map[string]interface{}) toolRoute { return toolRoute{"POST", "/v1/orders/" + sanitizePathParam(a["orderId"]) + "/cancel"} },
	"orders_fulfill":        func(a map[string]interface{}) toolRoute { return toolRoute{"POST", "/v1/orders/" + sanitizePathParam(a["orderId"]) + "/fulfill"} },
	"orders_refund":         func(a map[string]interface{}) toolRoute { return toolRoute{"POST", "/v1/orders/" + sanitizePathParam(a["orderId"]) + "/refund"} },
	"orders_complete":       func(a map[string]interface{}) toolRoute { return toolRoute{"POST", "/v1/orders/" + sanitizePathParam(a["orderId"]) + "/complete"} },
	"profile_get":           func(_ map[string]interface{}) toolRoute { return toolRoute{"GET", "/v1/profiles"} },
	"profile_update":        func(_ map[string]interface{}) toolRoute { return toolRoute{"PUT", "/v1/profiles"} },
	"chat_get_conversations": func(_ map[string]interface{}) toolRoute { return toolRoute{"GET", "/v1/chat/conversations"} },
	"chat_get_messages":     func(a map[string]interface{}) toolRoute { return toolRoute{"GET", "/v1/chat/conversations/" + sanitizePathParam(a["peerID"]) + "/messages"} },
	"chat_send_message":     func(_ map[string]interface{}) toolRoute { return toolRoute{"POST", "/v1/chat/messages"} },
	"notifications_list":    func(_ map[string]interface{}) toolRoute { return toolRoute{"GET", "/v1/notifications"} },
	"exchange_rates_get":    func(_ map[string]interface{}) toolRoute { return toolRoute{"GET", "/v1/exchange-rates"} },
	"discounts_list":        func(_ map[string]interface{}) toolRoute { return toolRoute{"GET", "/v1/discounts"} },
	"discounts_create":      func(_ map[string]interface{}) toolRoute { return toolRoute{"POST", "/v1/discounts"} },
	"discounts_update":      func(a map[string]interface{}) toolRoute { return toolRoute{"PUT", "/v1/discounts/" + sanitizePathParam(a["discountId"])} },
	"discounts_delete":      func(a map[string]interface{}) toolRoute { return toolRoute{"DELETE", "/v1/discounts/" + sanitizePathParam(a["discountId"])} },
	"collections_list":      func(_ map[string]interface{}) toolRoute { return toolRoute{"GET", "/v1/collections"} },
	"collections_create":    func(_ map[string]interface{}) toolRoute { return toolRoute{"POST", "/v1/collections"} },
}

// sanitizePathParam prevents path traversal by stripping slashes, dots-sequences,
// and URL-encoding the value.
func sanitizePathParam(v interface{}) string {
	s := fmt.Sprintf("%v", v)
	s = strings.ReplaceAll(s, "/", "")
	s = strings.ReplaceAll(s, "..", "")
	return url.PathEscape(s)
}

// Execute runs a tool by calling the local Node API and returns the JSON result.
func (te *ToolExecutor) Execute(ctx context.Context, toolName string, argsJSON string) (string, error) {
	routeFn, ok := toolRoutes[toolName]
	if !ok {
		return "", fmt.Errorf("unknown tool: %s", toolName)
	}

	var args map[string]interface{}
	if argsJSON != "" && argsJSON != "{}" {
		if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
			return "", fmt.Errorf("invalid tool arguments: %w", err)
		}
	}
	if args == nil {
		args = map[string]interface{}{}
	}

	route := routeFn(args)

	var bodyReader io.Reader
	if route.Method == "POST" || route.Method == "PUT" {
		bodyBytes, err := buildRequestBody(toolName, args)
		if err != nil {
			return "", err
		}
		bodyReader = bytes.NewReader(bodyBytes)
	}

	apiURL := te.baseURL + route.Path
	if route.Method == "GET" {
		apiURL = appendQueryParams(apiURL, toolName, args)
	}

	req, err := http.NewRequestWithContext(ctx, route.Method, apiURL, bodyReader)
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if te.authToken != "" {
		req.Header.Set("Authorization", te.authToken)
	}

	resp, err := te.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("execute tool %s: %w", toolName, err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 64*1024))
	if err != nil {
		return "", fmt.Errorf("read tool response: %w", err)
	}

	if resp.StatusCode >= 400 {
		return "", fmt.Errorf("tool %s returned %d: %s", toolName, resp.StatusCode, truncate(string(respBody), 500))
	}

	return string(respBody), nil
}

func buildRequestBody(toolName string, args map[string]interface{}) ([]byte, error) {
	switch {
	case toolName == "listings_create" || toolName == "listings_update":
		if listing, ok := args["listing"]; ok {
			return json.Marshal(listing)
		}
		return json.Marshal(args)
	case toolName == "profile_update":
		if profile, ok := args["profile"]; ok {
			return json.Marshal(profile)
		}
		return json.Marshal(args)
	case toolName == "discounts_create" || toolName == "discounts_update":
		if discount, ok := args["discount"]; ok {
			return json.Marshal(discount)
		}
		return json.Marshal(args)
	case toolName == "collections_create":
		if collection, ok := args["collection"]; ok {
			return json.Marshal(collection)
		}
		return json.Marshal(args)
	default:
		return json.Marshal(args)
	}
}

func appendQueryParams(baseURL, toolName string, args map[string]interface{}) string {
	paramKeys := map[string][]string{
		"listings_list_mine": {"limit", "offset"},
		"orders_get_sales":   {"limit", "offset"},
		"notifications_list": {"limit", "offset"},
		"chat_get_messages":  {"limit", "offsetId"},
	}
	keys, ok := paramKeys[toolName]
	if !ok {
		return baseURL
	}
	params := url.Values{}
	for _, k := range keys {
		if v, exists := args[k]; exists {
			params.Set(k, fmt.Sprintf("%v", v))
		}
	}
	if len(params) == 0 {
		return baseURL
	}
	sep := "?"
	if strings.Contains(baseURL, "?") {
		sep = "&"
	}
	return baseURL + sep + params.Encode()
}

func truncate(s string, max int) string {
	runes := []rune(s)
	if len(runes) <= max {
		return s
	}
	return string(runes[:max]) + "..."
}
