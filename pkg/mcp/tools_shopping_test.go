package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"testing"
	"time"

	gomcp "github.com/mark3labs/mcp-go/mcp"
)

// --- mockBridgeForShopping ---

type mockShoppingBridge struct {
	responses map[string]mockResponse // key: "METHOD path"
	calls     []string
}

type mockResponse struct {
	code int
	body []byte
}

func (m *mockShoppingBridge) Call(_ context.Context, method, path string, _ url.Values, _ interface{}) (int, []byte, error) {
	key := method + " " + path
	m.calls = append(m.calls, key)
	if resp, ok := m.responses[key]; ok {
		return resp.code, resp.body, nil
	}
	return 404, []byte(`{"error":{"code":"NOT_FOUND","message":"not found"}}`), nil
}

func (m *mockShoppingBridge) CallMultipart(_ context.Context, _, _ string, _, _ string, _ []byte) (int, []byte, error) {
	return 200, []byte(`{}`), nil
}

func newMockShoppingBridge(responses map[string]mockResponse) *mockShoppingBridge {
	return &mockShoppingBridge{responses: responses}
}

// --- Test helpers ---

var testShoppingConfig = ShoppingConfig{
	DemoStorePeerID: "QmDemoStore123",
	AllowedSlugs:    []string{"test-sticker", "digital-wallpaper"},
	MaxOrderAmount:  10.0,
	DemoOrderToken:  "guest_demo_token_123",
}

const testPaymentCoin = "crypto:eip155:56:erc20:0x55d398326f99059fF775485246999027B3197955"

func testListingResponse() []byte {
	return []byte(`{"data":{"listing":{"slug":"test-sticker","item":{"title":"Test Sticker","price":"199"},"metadata":{"pricingCurrency":{"code":"USD","divisibility":2}}},"hash":"QmListingHash123"}}`)
}

func testPaymentMethodsResponse() []byte {
	return []byte(`{"data":{"crypto":["crypto:eip155:56:erc20:0x55d398326f99059fF775485246999027B3197955","crypto:eip155:1:native"]}}`)
}

func testSearchResponse() []byte {
	return []byte(`{"data":[{"listing":{"slug":"test-sticker","item":{"title":"Test Sticker"}}},{"listing":{"slug":"hidden-slug","item":{"title":"Hidden"}}}]}`)
}

func testGuestOrderResponse() []byte {
	return []byte(`{"data":{"orderToken":"guest_abc123","paymentAddress":"0x1234567890abcdef","paymentAmount":"1990000","chainName":"BSC","paymentCoin":"crypto:eip155:56:erc20:0x55d398326f99059fF775485246999027B3197955","expiresAt":"2026-05-19T12:00:00Z"}}`)
}

func testOrderStatusResponse() []byte {
	return []byte(`{"data":{"status":"AWAITING_PAYMENT","orderToken":"guest_abc123"}}`)
}

func makeToolRequest(args map[string]interface{}) gomcp.CallToolRequest {
	req := gomcp.CallToolRequest{}
	req.Params.Arguments = args
	return req
}

// --- QuoteToken Tests ---

func TestQuoteTokenSigner_SignAndVerify(t *testing.T) {
	signer := NewQuoteTokenSigner([]byte("test-secret-key-32-bytes-long!!"))

	payload := &QuotePayload{
		StorePeerID:  "QmDemoStore123",
		Slug:         "test-sticker",
		ListingHash:  "QmListingHash123",
		Quantity:     1,
		CoinType:     testPaymentCoin,
		MaxTotalSats: "1990000",
	}

	token, err := signer.Sign(payload)
	if err != nil {
		t.Fatalf("Sign failed: %v", err)
	}

	if token == "" {
		t.Fatal("expected non-empty token")
	}

	verified, err := signer.Verify(token)
	if err != nil {
		t.Fatalf("Verify failed: %v", err)
	}

	if verified.StorePeerID != payload.StorePeerID {
		t.Errorf("StorePeerID mismatch: got %q, want %q", verified.StorePeerID, payload.StorePeerID)
	}
	if verified.Slug != payload.Slug {
		t.Errorf("Slug mismatch: got %q, want %q", verified.Slug, payload.Slug)
	}
	if verified.Quantity != payload.Quantity {
		t.Errorf("Quantity mismatch: got %d, want %d", verified.Quantity, payload.Quantity)
	}
	if verified.CoinType != payload.CoinType {
		t.Errorf("CoinType mismatch: got %q, want %q", verified.CoinType, payload.CoinType)
	}
}

func TestQuoteTokenSigner_Expired(t *testing.T) {
	signer := NewQuoteTokenSigner([]byte("test-secret-key-32-bytes-long!!"))

	payload := &QuotePayload{
		StorePeerID: "QmDemoStore123",
		Slug:        "test-sticker",
		ListingHash: "QmListingHash123",
		Quantity:    1,
		CoinType:    testPaymentCoin,
		ExpiresAt:   time.Now().Add(-1 * time.Minute).Unix(),
	}

	token, err := signer.Sign(payload)
	if err != nil {
		t.Fatalf("Sign failed: %v", err)
	}

	_, err = signer.Verify(token)
	if err == nil {
		t.Fatal("expected error for expired token")
	}
	if !containsString(err.Error(), "expired") {
		t.Errorf("expected 'expired' in error, got: %v", err)
	}
}

func TestQuoteTokenSigner_Tampered(t *testing.T) {
	signer := NewQuoteTokenSigner([]byte("test-secret-key-32-bytes-long!!"))

	payload := &QuotePayload{
		StorePeerID: "QmDemoStore123",
		Slug:        "test-sticker",
		ListingHash: "QmListingHash123",
		Quantity:    1,
		CoinType:    testPaymentCoin,
	}

	token, err := signer.Sign(payload)
	if err != nil {
		t.Fatalf("Sign failed: %v", err)
	}

	// Tamper with the token
	tampered := token[:len(token)-4] + "XXXX"
	_, err = signer.Verify(tampered)
	if err == nil {
		t.Fatal("expected error for tampered token")
	}
}

func TestQuoteTokenSigner_WrongKey(t *testing.T) {
	signer1 := NewQuoteTokenSigner([]byte("key-one-32-bytes-long-enough!!!"))
	signer2 := NewQuoteTokenSigner([]byte("key-two-32-bytes-long-enough!!!"))

	payload := &QuotePayload{
		StorePeerID: "QmDemoStore123",
		Slug:        "test-sticker",
		ListingHash: "QmListingHash123",
		Quantity:    1,
		CoinType:    "USDT",
	}

	token, err := signer1.Sign(payload)
	if err != nil {
		t.Fatalf("Sign failed: %v", err)
	}

	_, err = signer2.Verify(token)
	if err == nil {
		t.Fatal("expected error for wrong key")
	}
}

func TestQuoteTokenSigner_IncompletePayload(t *testing.T) {
	signer := NewQuoteTokenSigner([]byte("test-secret-key-32-bytes-long!!"))

	tests := []struct {
		name    string
		payload *QuotePayload
	}{
		{"missing store", &QuotePayload{Slug: "s", Quantity: 1, CoinType: "USDT"}},
		{"missing slug", &QuotePayload{StorePeerID: "Qm", Quantity: 1, CoinType: "USDT"}},
		{"zero quantity", &QuotePayload{StorePeerID: "Qm", Slug: "s", Quantity: 0, CoinType: "USDT"}},
		{"missing coin", &QuotePayload{StorePeerID: "Qm", Slug: "s", Quantity: 1}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := signer.Sign(tt.payload)
			if err == nil {
				t.Error("expected error for incomplete payload")
			}
		})
	}
}

func TestQuoteTokenSigner_RandomSecret(t *testing.T) {
	signer := NewQuoteTokenSigner(nil)

	payload := &QuotePayload{
		StorePeerID:  "QmDemoStore123",
		Slug:         "test-sticker",
		ListingHash:  "QmListingHash123",
		Quantity:     1,
		CoinType:     testPaymentCoin,
		MaxTotalSats: "199",
	}

	token, err := signer.Sign(payload)
	if err != nil {
		t.Fatalf("Sign failed: %v", err)
	}

	verified, err := signer.Verify(token)
	if err != nil {
		t.Fatalf("Verify failed: %v", err)
	}
	if verified.StorePeerID != "QmDemoStore123" {
		t.Errorf("unexpected StorePeerID: %s", verified.StorePeerID)
	}
}

// --- Shopping Tool Tests ---

func TestShoppingSearchDemo_ReturnsResults(t *testing.T) {
	storeBridge := newMockShoppingBridge(map[string]mockResponse{
		"GET /v1/listings/QmDemoStore123": {200, []byte(`{"data":[{"slug":"test-sticker","title":"Test Sticker"}]}`)},
	})

	handler := makeShoppingSearchDemo(nil, storeBridge, testShoppingConfig)
	result, err := handler(context.Background(), makeToolRequest(map[string]interface{}{}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatalf("unexpected tool error: %v", result.Content)
	}
}

func TestShoppingSearchDemo_FiltersDisallowedSlugs(t *testing.T) {
	searchBridge := newMockShoppingBridge(map[string]mockResponse{
		"GET /search/v1/listings": {200, testSearchResponse()},
	})

	handler := makeShoppingSearchDemo(searchBridge, nil, testShoppingConfig)
	result, err := handler(context.Background(), makeToolRequest(map[string]interface{}{}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatalf("unexpected tool error: %v", result.Content)
	}

	text := extractTextContent(result)
	if containsString(text, "hidden-slug") {
		t.Fatalf("expected disallowed slug to be filtered out, got %s", text)
	}
}

func TestShoppingGetDetail_RequiresSlug(t *testing.T) {
	storeBridge := newMockShoppingBridge(map[string]mockResponse{})
	handler := makeShoppingGetDetail(storeBridge, testShoppingConfig)

	result, err := handler(context.Background(), makeToolRequest(map[string]interface{}{}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsError {
		t.Error("expected tool error for missing slug")
	}
}

func TestShoppingGetDetail_RejectsUnallowedSlug(t *testing.T) {
	storeBridge := newMockShoppingBridge(map[string]mockResponse{})
	handler := makeShoppingGetDetail(storeBridge, testShoppingConfig)

	result, err := handler(context.Background(), makeToolRequest(map[string]interface{}{
		"slug": "unauthorized-product",
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsError {
		t.Error("expected tool error for unallowed slug")
	}
}

func TestShoppingGetDetail_ReturnsListingAndPaymentMethods(t *testing.T) {
	storeBridge := newMockShoppingBridge(map[string]mockResponse{
		"GET /v1/listings/QmDemoStore123/test-sticker": {200, testListingResponse()},
		"GET /v1/payment-methods/QmDemoStore123":       {200, testPaymentMethodsResponse()},
	})

	handler := makeShoppingGetDetail(storeBridge, testShoppingConfig)
	result, err := handler(context.Background(), makeToolRequest(map[string]interface{}{
		"slug": "test-sticker",
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatalf("unexpected tool error")
	}

	text := extractTextContent(result)
	var parsed map[string]json.RawMessage
	if err := json.Unmarshal([]byte(text), &parsed); err != nil {
		t.Fatalf("failed to parse result JSON: %v", err)
	}
	if _, ok := parsed["listing"]; !ok {
		t.Error("expected 'listing' in result")
	}
	if _, ok := parsed["paymentMethods"]; !ok {
		t.Error("expected 'paymentMethods' in result")
	}
}

func TestShoppingPrepareCheckout_RequiresParams(t *testing.T) {
	storeBridge := newMockShoppingBridge(map[string]mockResponse{})
	signer := NewQuoteTokenSigner([]byte("test-secret-key-32-bytes-long!!"))
	handler := makeShoppingPrepareCheckout(storeBridge, testShoppingConfig, signer)

	tests := []struct {
		name string
		args map[string]interface{}
	}{
		{"missing slug", map[string]interface{}{"coin": "USDT"}},
		{"missing coin", map[string]interface{}{"slug": "test-sticker"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := handler(context.Background(), makeToolRequest(tt.args))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !result.IsError {
				t.Error("expected tool error for missing params")
			}
		})
	}
}

func TestShoppingPrepareCheckout_ReturnsQuoteToken(t *testing.T) {
	storeBridge := newMockShoppingBridge(map[string]mockResponse{
		"GET /v1/listings/QmDemoStore123/test-sticker": {200, testListingResponse()},
		"GET /v1/payment-methods/QmDemoStore123":       {200, testPaymentMethodsResponse()},
	})
	signer := NewQuoteTokenSigner([]byte("test-secret-key-32-bytes-long!!"))
	handler := makeShoppingPrepareCheckout(storeBridge, testShoppingConfig, signer)

	result, err := handler(context.Background(), makeToolRequest(map[string]interface{}{
		"slug": "test-sticker",
		"coin": testPaymentCoin,
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatalf("unexpected tool error")
	}

	text := extractTextContent(result)
	var parsed map[string]json.RawMessage
	if err := json.Unmarshal([]byte(text), &parsed); err != nil {
		t.Fatalf("failed to parse result: %v", err)
	}

	if _, ok := parsed["quoteToken"]; !ok {
		t.Error("expected 'quoteToken' in result")
	}
	var quoteToken string
	if err := json.Unmarshal(parsed["quoteToken"], &quoteToken); err != nil {
		t.Fatalf("failed to parse quote token: %v", err)
	}
	quote, err := signer.Verify(quoteToken)
	if err != nil {
		t.Fatalf("failed to verify quote token: %v", err)
	}
	if quote.CoinType != testPaymentCoin {
		t.Fatalf("quote coin = %q, want %q", quote.CoinType, testPaymentCoin)
	}
	if _, ok := parsed["summary"]; !ok {
		t.Error("expected 'summary' in result")
	}
}

func TestShoppingPrepareCheckout_RejectsOverMaxOrderAmount(t *testing.T) {
	cfg := testShoppingConfig
	cfg.MaxOrderAmount = 1.50

	storeBridge := newMockShoppingBridge(map[string]mockResponse{
		"GET /v1/listings/QmDemoStore123/test-sticker": {200, testListingResponse()},
		"GET /v1/payment-methods/QmDemoStore123":       {200, testPaymentMethodsResponse()},
	})
	signer := NewQuoteTokenSigner([]byte("test-secret-key-32-bytes-long!!"))
	handler := makeShoppingPrepareCheckout(storeBridge, cfg, signer)

	result, err := handler(context.Background(), makeToolRequest(map[string]interface{}{
		"slug": "test-sticker",
		"coin": testPaymentCoin,
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsError {
		t.Fatal("expected tool error for order above max amount")
	}
}

func TestShoppingConfirmCheckout_RequiresQuoteToken(t *testing.T) {
	storeBridge := newMockShoppingBridge(map[string]mockResponse{})
	signer := NewQuoteTokenSigner([]byte("test-secret-key-32-bytes-long!!"))
	handler := makeShoppingConfirmCheckout(storeBridge, testShoppingConfig, signer)

	result, err := handler(context.Background(), makeToolRequest(map[string]interface{}{}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsError {
		t.Error("expected tool error for missing quote_token")
	}
}

func TestShoppingConfirmCheckout_RejectsExpiredToken(t *testing.T) {
	storeBridge := newMockShoppingBridge(map[string]mockResponse{})
	signer := NewQuoteTokenSigner([]byte("test-secret-key-32-bytes-long!!"))

	payload := &QuotePayload{
		StorePeerID: "QmDemoStore123",
		Slug:        "test-sticker",
		ListingHash: "QmHash",
		Quantity:    1,
		CoinType:    "USDT",
		ExpiresAt:   time.Now().Add(-1 * time.Minute).Unix(),
	}
	token, _ := signer.Sign(payload)

	handler := makeShoppingConfirmCheckout(storeBridge, testShoppingConfig, signer)
	result, err := handler(context.Background(), makeToolRequest(map[string]interface{}{
		"quote_token": token,
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsError {
		t.Error("expected tool error for expired token")
	}
	text := extractTextContent(result)
	if !containsString(text, "expired") && !containsString(text, "Expired") {
		t.Errorf("expected 'expired' in error message, got: %s", text)
	}
}

func TestShoppingConfirmCheckout_RejectsWrongStore(t *testing.T) {
	storeBridge := newMockShoppingBridge(map[string]mockResponse{})
	signer := NewQuoteTokenSigner([]byte("test-secret-key-32-bytes-long!!"))

	payload := &QuotePayload{
		StorePeerID: "QmWrongStorePeerID",
		Slug:        "test-sticker",
		ListingHash: "QmHash",
		Quantity:    1,
		CoinType:    "USDT",
	}
	token, _ := signer.Sign(payload)

	handler := makeShoppingConfirmCheckout(storeBridge, testShoppingConfig, signer)
	result, err := handler(context.Background(), makeToolRequest(map[string]interface{}{
		"quote_token": token,
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsError {
		t.Error("expected tool error for wrong store")
	}
}

func TestShoppingConfirmCheckout_CreatesOrder(t *testing.T) {
	signer := NewQuoteTokenSigner([]byte("test-secret-key-32-bytes-long!!"))

	payload := &QuotePayload{
		StorePeerID:  "QmDemoStore123",
		Slug:         "test-sticker",
		ListingHash:  "QmListingHash123",
		Quantity:     1,
		CoinType:     testPaymentCoin,
		MaxTotalSats: "199",
	}
	token, _ := signer.Sign(payload)

	storeBridge := newMockShoppingBridge(map[string]mockResponse{
		"GET /v1/listings/QmDemoStore123/test-sticker": {200, testListingResponse()},
		"GET /v1/payment-methods/QmDemoStore123":       {200, testPaymentMethodsResponse()},
		"POST /v1/guest/orders":                        {201, testGuestOrderResponse()},
	})

	handler := makeShoppingConfirmCheckout(storeBridge, testShoppingConfig, signer)
	result, err := handler(context.Background(), makeToolRequest(map[string]interface{}{
		"quote_token": token,
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatalf("unexpected tool error: %s", extractTextContent(result))
	}

	text := extractTextContent(result)
	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(text), &parsed); err != nil {
		t.Fatalf("failed to parse result: %v", err)
	}
	if parsed["isDemo"] != false {
		t.Error("expected isDemo=false for real order")
	}
	for _, key := range []string{"copyableAddress", "paymentURI", "qrPayload", "amountDisplay"} {
		if _, ok := parsed[key]; !ok {
			t.Errorf("expected %q in payment payload", key)
		}
	}
	if parsed["amountDisplay"] != "1.99 USDT" {
		t.Fatalf("amountDisplay = %v, want 1.99 USDT", parsed["amountDisplay"])
	}
	if !containsString(fmt.Sprint(parsed["paymentURI"]), "amount=1.99") {
		t.Fatalf("paymentURI should use decimal amount, got %v", parsed["paymentURI"])
	}
}

func TestShoppingConfirmCheckout_RejectsChangedListing(t *testing.T) {
	signer := NewQuoteTokenSigner([]byte("test-secret-key-32-bytes-long!!"))

	payload := &QuotePayload{
		StorePeerID:  "QmDemoStore123",
		Slug:         "test-sticker",
		ListingHash:  "QmListingHash123",
		Quantity:     1,
		CoinType:     testPaymentCoin,
		MaxTotalSats: "199",
	}
	token, _ := signer.Sign(payload)

	storeBridge := newMockShoppingBridge(map[string]mockResponse{
		"GET /v1/listings/QmDemoStore123/test-sticker": {200, []byte(`{"data":{"listing":{"slug":"test-sticker","item":{"title":"Test Sticker","price":"299"},"metadata":{"pricingCurrency":{"code":"USD","divisibility":2}}},"hash":"QmListingHash999"}}`)},
	})

	handler := makeShoppingConfirmCheckout(storeBridge, testShoppingConfig, signer)
	result, err := handler(context.Background(), makeToolRequest(map[string]interface{}{
		"quote_token": token,
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsError {
		t.Fatal("expected tool error for changed listing")
	}
	for _, call := range storeBridge.calls {
		if call == "POST /v1/guest/orders" {
			t.Fatal("guest order should not be created when listing hash changes")
		}
	}
}

func TestShoppingOrderStatus_RequiresToken(t *testing.T) {
	storeBridge := newMockShoppingBridge(map[string]mockResponse{})
	handler := makeShoppingOrderStatus(storeBridge, testShoppingConfig)

	result, err := handler(context.Background(), makeToolRequest(map[string]interface{}{}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsError {
		t.Error("expected tool error for missing order_token")
	}
}

func TestShoppingOrderStatus_ReturnsStatus(t *testing.T) {
	storeBridge := newMockShoppingBridge(map[string]mockResponse{
		"GET /v1/guest/orders/guest_abc123": {200, testOrderStatusResponse()},
	})

	handler := makeShoppingOrderStatus(storeBridge, testShoppingConfig)
	result, err := handler(context.Background(), makeToolRequest(map[string]interface{}{
		"order_token": "guest_abc123",
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatalf("unexpected tool error")
	}
}

func TestShoppingDemoOrderStatus_NoDemoToken(t *testing.T) {
	cfgNoDemoToken := ShoppingConfig{
		DemoStorePeerID: "QmDemoStore123",
	}
	storeBridge := newMockShoppingBridge(map[string]mockResponse{})
	handler := makeShoppingDemoOrderStatus(storeBridge, cfgNoDemoToken)

	result, err := handler(context.Background(), makeToolRequest(map[string]interface{}{}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsError {
		t.Error("expected tool error when no demo token configured")
	}
}

func TestShoppingDemoOrderStatus_ReturnsDemoStatus(t *testing.T) {
	storeBridge := newMockShoppingBridge(map[string]mockResponse{
		fmt.Sprintf("GET /v1/guest/orders/%s", testShoppingConfig.DemoOrderToken): {200, testOrderStatusResponse()},
	})

	handler := makeShoppingDemoOrderStatus(storeBridge, testShoppingConfig)
	result, err := handler(context.Background(), makeToolRequest(map[string]interface{}{}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatalf("unexpected tool error")
	}

	text := extractTextContent(result)
	var parsed map[string]json.RawMessage
	if err := json.Unmarshal([]byte(text), &parsed); err != nil {
		t.Fatalf("failed to parse result: %v", err)
	}
	var isDemo bool
	json.Unmarshal(parsed["isDemo"], &isDemo)
	if !isDemo {
		t.Error("expected isDemo=true for demo order status")
	}
}

// --- Slug whitelist tests ---

func TestIsSlugAllowed_EmptyAllowList(t *testing.T) {
	cfg := ShoppingConfig{DemoStorePeerID: "Qm"}
	if !isSlugAllowed(cfg, "any-slug") {
		t.Error("empty allow list should allow all slugs")
	}
}

func TestIsSlugAllowed_WithAllowList(t *testing.T) {
	cfg := ShoppingConfig{
		DemoStorePeerID: "Qm",
		AllowedSlugs:    []string{"allowed-a", "allowed-b"},
	}
	if !isSlugAllowed(cfg, "allowed-a") {
		t.Error("allowed-a should be allowed")
	}
	if isSlugAllowed(cfg, "not-allowed") {
		t.Error("not-allowed should be rejected")
	}
}

// --- Shopping tools registration test ---

func TestShoppingToolsRegisteredWithConfig(t *testing.T) {
	bf := StaticBridgeFactory(&mockBridge{})
	opts := &ServerOptions{
		SearchURL:       "http://test-search:8080",
		StoreGatewayURL: "http://test-store:4002",
		Shopping: &ShoppingConfig{
			DemoStorePeerID: "QmTestStore",
		},
	}
	registrars := getAllToolRegistrars(bf, opts)

	names := make(map[string]bool)
	for _, reg := range registrars {
		names[reg.Name] = true
	}

	shoppingTools := []string{
		"shopping_search_demo",
		"shopping_get_detail",
		"shopping_prepare_checkout",
		"shopping_confirm_checkout",
		"shopping_order_status",
		"shopping_demo_order_status",
	}
	for _, name := range shoppingTools {
		if !names[name] {
			t.Errorf("expected shopping tool %q to be registered", name)
		}
	}
}

func TestShoppingToolsNotRegisteredWithoutConfig(t *testing.T) {
	bf := StaticBridgeFactory(&mockBridge{})
	opts := &ServerOptions{SearchURL: "http://test-search:8080"}
	registrars := getAllToolRegistrars(bf, opts)

	for _, reg := range registrars {
		if len(reg.Name) > 8 && reg.Name[:9] == "shopping_" {
			t.Errorf("shopping tool %q should not be registered without Shopping config", reg.Name)
		}
	}
}

func TestLoadShoppingConfigFromEnv(t *testing.T) {
	t.Setenv("DEMO_STORE_PEER_ID", "QmEnvStore")
	t.Setenv("DEMO_ALLOWED_SLUGS", "a, b ,c")
	t.Setenv("DEMO_ORDER_TOKEN", "guest_demo")
	t.Setenv("DEMO_MAX_ORDER_AMOUNT", "9.99")
	t.Setenv("MCP_QUOTE_TOKEN_SECRET", "env-secret")

	cfg := LoadShoppingConfigFromEnv()
	if cfg == nil {
		t.Fatal("expected shopping config")
	}
	if cfg.DemoStorePeerID != "QmEnvStore" {
		t.Fatalf("DemoStorePeerID = %q, want %q", cfg.DemoStorePeerID, "QmEnvStore")
	}
	if len(cfg.AllowedSlugs) != 3 {
		t.Fatalf("AllowedSlugs length = %d, want 3", len(cfg.AllowedSlugs))
	}
	if cfg.DemoOrderToken != "guest_demo" {
		t.Fatalf("DemoOrderToken = %q, want %q", cfg.DemoOrderToken, "guest_demo")
	}
	if got := LoadQuoteTokenSecretFromEnv(); string(got) != "env-secret" {
		t.Fatalf("quote token secret = %q, want %q", string(got), "env-secret")
	}
}

func TestLoadShoppingConfigFromEnv_EmptyPeerIDDisablesTools(t *testing.T) {
	for _, key := range []string{
		"DEMO_STORE_PEER_ID",
		"MCP_DEMO_STORE_PEER_ID",
		"DEMO_ALLOWED_SLUGS",
		"MCP_DEMO_ALLOWED_SLUGS",
		"DEMO_ORDER_TOKEN",
		"MCP_DEMO_ORDER_TOKEN",
		"DEMO_MAX_ORDER_AMOUNT",
		"MCP_DEMO_MAX_ORDER_AMOUNT",
		"QUOTE_TOKEN_SECRET",
		"MCP_QUOTE_TOKEN_SECRET",
	} {
		_ = os.Unsetenv(key)
	}
	if cfg := LoadShoppingConfigFromEnv(); cfg != nil {
		t.Fatal("expected nil shopping config when demo store peer ID is not set")
	}
	if secret := LoadQuoteTokenSecretFromEnv(); secret != nil {
		t.Fatal("expected nil quote token secret when env is not set")
	}
}

// --- helpers ---

func extractTextContent(result *gomcp.CallToolResult) string {
	for _, c := range result.Content {
		if tc, ok := c.(gomcp.TextContent); ok {
			return tc.Text
		}
	}
	return ""
}

func containsString(s, substr string) bool {
	return len(s) >= len(substr) && searchSubstring(s, substr)
}

func searchSubstring(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
