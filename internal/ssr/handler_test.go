package ssr

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// --- UA detection ---

func TestIsCrawler(t *testing.T) {
	tests := []struct {
		ua   string
		want bool
	}{
		{"Mozilla/5.0 (compatible; Googlebot/2.1; +http://www.google.com/bot.html)", true},
		{"Twitterbot/1.0", true},
		{"facebookexternalhit/1.1", true},
		{"Mozilla/5.0 (Linux; Android 10) AppleWebKit/537.36 Chrome/91.0 Mobile Safari/537.36", false},
		{"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15) AppleWebKit/605.1 Safari/605.1", false},
		{"TelegramBot (like TwitterBot)", true},
		{"Slackbot-LinkExpanding 1.0", true},
		{"WhatsApp/2.21.7.14", true},
		{"Discordbot/2.0", true},
		{"", false},
		{"curl/7.68.0", false},
		{"LinkedInBot/1.0", true},
		{"Lighthouse", true},
	}
	for _, tt := range tests {
		if got := IsCrawler(tt.ua); got != tt.want {
			t.Errorf("IsCrawler(%q) = %v, want %v", tt.ua, got, tt.want)
		}
	}
}

// --- Listing response parsing ---

func TestParseListingResponse(t *testing.T) {
	resp := `{"data":{"listing":{"slug":"cool-shirt","vendorID":{"peerID":"QmTest123"},"metadata":{"pricingCurrency":{"code":"USD"}},"item":{"title":"Cool Shirt","description":"A very cool shirt for testing","shortDescription":"Cool shirt","price":"4900","images":[{"medium":"QmImg1","small":"QmImgSmall","large":"","original":"QmImgOrig","filename":"shirt.jpg"}]}}}}`

	pd, err := parseListingResponse([]byte(resp))
	if err != nil {
		t.Fatalf("parseListingResponse error: %v", err)
	}
	if pd.Slug != "cool-shirt" {
		t.Errorf("slug = %q, want cool-shirt", pd.Slug)
	}
	if pd.Title != "Cool Shirt" {
		t.Errorf("title = %q, want Cool Shirt", pd.Title)
	}
	if pd.Price != "4900" {
		t.Errorf("price = %q, want 4900", pd.Price)
	}
	if pd.CurrencyCode != "USD" {
		t.Errorf("currency = %q, want USD", pd.CurrencyCode)
	}
	if pd.ImageHash != "QmImg1" {
		t.Errorf("imageHash = %q, want QmImg1 (medium preferred)", pd.ImageHash)
	}
	if pd.VendorPeerID != "QmTest123" {
		t.Errorf("vendorPeerID = %q, want QmTest123", pd.VendorPeerID)
	}
}

func TestParseListingResponse_NoImages(t *testing.T) {
	resp := `{"data":{"listing":{"slug":"no-img","vendorID":{"peerID":"QmV"},"metadata":{"pricingCurrency":{"code":"BTC"}},"item":{"title":"NoImg","description":"desc","price":"100","images":[]}}}}`

	pd, err := parseListingResponse([]byte(resp))
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if pd.ImageHash != "" {
		t.Errorf("imageHash = %q, want empty", pd.ImageHash)
	}
}

// --- Profile response parsing ---

func TestParseProfileResponse(t *testing.T) {
	resp := `{"data":{"peerID":"QmStore1","name":"Test Store","handle":"@teststore","about":"Great products","location":"Tokyo","avatarHashes":{"medium":"QmAvatar","small":"","original":""},"headerHashes":{"large":"QmHeader","medium":"","original":""},"stats":{"listingCount":42,"ratingCount":10,"averageRating":4.5}}}`

	pd, err := parseProfileResponse([]byte(resp))
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if pd.Name != "Test Store" {
		t.Errorf("name = %q", pd.Name)
	}
	if pd.ListingCount != 42 {
		t.Errorf("listingCount = %d", pd.ListingCount)
	}
	if pd.AvatarHash != "QmAvatar" {
		t.Errorf("avatarHash = %q", pd.AvatarHash)
	}
	if pd.AvgRating != 4.5 {
		t.Errorf("avgRating = %f", pd.AvgRating)
	}
}

// --- Meta injection ---

func TestInjectProductMeta(t *testing.T) {
	h := &SSRHandler{
		domain:  "mystore.com",
		spaHTML: []byte(`<!DOCTYPE html><html><head><title>SPA</title></head><body></body></html>`),
	}

	product := &ProductData{
		Slug:             "cool-shirt",
		Title:            "Cool Shirt",
		Description:      "A very cool shirt",
		ShortDescription: "Cool shirt",
		Price:            "4900",
		CurrencyCode:     "USD",
		ImageHash:        "QmImg123",
	}

	result, err := h.injectProductMeta(product)
	if err != nil {
		t.Fatalf("error: %v", err)
	}

	html := string(result)
	checks := []string{
		`og:title" content="Cool Shirt"`,
		`og:description" content="Cool shirt"`,
		`og:image" content="https://mystore.com/v1/media/images/QmImg123"`,
		`og:url" content="https://mystore.com/product/cool-shirt"`,
		`twitter:card" content="summary_large_image"`,
		`application/ld+json`,
		`"@type":"Product"`,
		`application/json+oembed`,
	}
	for _, c := range checks {
		if !strings.Contains(html, c) {
			t.Errorf("missing in output: %s", c)
		}
	}
	if !strings.Contains(html, "</head>") {
		t.Error("</head> missing after injection")
	}
}

func TestInjectProfileMeta(t *testing.T) {
	h := &SSRHandler{
		domain:  "mystore.com",
		spaHTML: []byte(`<!DOCTYPE html><html><head></head><body></body></html>`),
	}

	profile := &ProfileData{
		PeerID:     "QmStore1",
		Name:       "Test Store",
		About:      "Best products ever",
		AvatarHash: "QmAv1",
	}

	result, err := h.injectProfileMeta(profile)
	if err != nil {
		t.Fatalf("error: %v", err)
	}

	html := string(result)
	checks := []string{
		`og:title" content="Test Store — Mobazha Store"`,
		`og:url" content="https://mystore.com/store/QmStore1"`,
		`twitter:card" content="summary"`,
		`"@type":"Organization"`,
	}
	for _, c := range checks {
		if !strings.Contains(html, c) {
			t.Errorf("missing: %s", c)
		}
	}
}

// --- oEmbed handler (with mock API) ---

func TestHandleOEmbed_Product(t *testing.T) {
	mockAPI := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"data":{"listing":{"slug":"shirt","vendorID":{"peerID":"QmV"},"metadata":{"pricingCurrency":{"code":"USD"}},"item":{"title":"Shirt","description":"Nice","price":"1000","images":[]}}}}`)
	}))
	defer mockAPI.Close()

	// Extract port from mockAPI.URL
	parts := strings.Split(mockAPI.URL, ":")
	port := 0
	fmt.Sscanf(parts[len(parts)-1], "%d", &port)

	h := &SSRHandler{
		nodePort:    port,
		domain:      "localhost",
		localPeerID: "QmV",
		httpClient:  mockAPI.Client(),
	}

	req := httptest.NewRequest("GET", "/api/oembed?url=https://localhost/product/shirt&format=json", nil)
	w := httptest.NewRecorder()

	h.handleOEmbed(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", w.Code, w.Body.String())
	}

	var resp oembedResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp.Type != "rich" {
		t.Errorf("type = %q", resp.Type)
	}
	if resp.Title != "Shirt" {
		t.Errorf("title = %q", resp.Title)
	}
	if !strings.Contains(resp.HTML, "iframe") {
		t.Error("HTML should contain iframe")
	}
	if w.Header().Get("Access-Control-Allow-Origin") != "*" {
		t.Error("missing CORS header")
	}
}

func TestHandleOEmbed_BadURL(t *testing.T) {
	h := &SSRHandler{domain: "example.com"}

	req := httptest.NewRequest("GET", "/api/oembed", nil)
	w := httptest.NewRecorder()
	h.handleOEmbed(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("no url param: status = %d", w.Code)
	}

	req = httptest.NewRequest("GET", "/api/oembed?url=https://evil.com/product/x", nil)
	w = httptest.NewRecorder()
	h.handleOEmbed(w, req)
	if w.Code != http.StatusForbidden {
		t.Errorf("bad host: status = %d", w.Code)
	}
}

// --- Helpers ---

func TestTruncate(t *testing.T) {
	if got := truncate("hello", 10); got != "hello" {
		t.Errorf("short string: %q", got)
	}
	long := strings.Repeat("a", 300)
	got := truncate(long, 200)
	if len([]rune(got)) != 200 {
		t.Errorf("truncated len = %d", len([]rune(got)))
	}
	if !strings.HasSuffix(got, "…") {
		t.Error("should end with ellipsis")
	}
}

func TestJsonString(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{`hello`, `"hello"`},
		{`he"llo`, `"he\"llo"`},
		{"line\nbreak", `"line\nbreak"`},
		{`<script>`, `"\u003cscript\u003e"`},
		{`a&b`, `"a\u0026b"`},
	}
	for _, tt := range tests {
		if got := jsonString(tt.in); got != tt.want {
			t.Errorf("jsonString(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestDisplayName(t *testing.T) {
	if got := displayName(&ProfileData{Name: "Store"}); got != "Store" {
		t.Errorf("with name: %q", got)
	}
	if got := displayName(&ProfileData{Handle: "@shop"}); got != "@shop" {
		t.Errorf("with handle: %q", got)
	}
	pid := "QmYJgDm5aB6FZ7dPHRPhbLcaK7RtzFPvdN3f123456789abc"
	got := displayName(&ProfileData{PeerID: pid})
	if !strings.HasPrefix(got, "QmYJgDm5") || !strings.HasSuffix(got, "9abc") {
		t.Errorf("truncated peerID: %q", got)
	}
}
