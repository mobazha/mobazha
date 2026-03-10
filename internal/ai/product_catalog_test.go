package ai

import (
	"strings"
	"testing"
)

func TestFormatProductCatalog_Empty(t *testing.T) {
	result := FormatProductCatalog(nil)
	if result != "" {
		t.Errorf("expected empty string for nil input, got %q", result)
	}
	result = FormatProductCatalog([]ListingSummary{})
	if result != "" {
		t.Errorf("expected empty string for empty slice, got %q", result)
	}
}

func TestFormatProductCatalog_SingleItem(t *testing.T) {
	listings := []ListingSummary{
		{
			Slug:        "leather-wallet",
			Title:       "Handmade Leather Wallet",
			Description: "Premium Italian leather wallet with RFID blocking technology",
			Price:       "2500",
			CoinType:    "USD",
			Status:      "published",
			ProductType: "PHYSICAL_GOOD",
		},
	}
	result := FormatProductCatalog(listings)

	if !strings.Contains(result, "1 items") {
		t.Error("expected item count in header")
	}
	if !strings.Contains(result, "[leather-wallet]") {
		t.Error("expected slug in brackets")
	}
	if !strings.Contains(result, "Handmade Leather Wallet") {
		t.Error("expected title")
	}
	if !strings.Contains(result, "2500 USD") {
		t.Error("expected price and coin type")
	}
	if !strings.Contains(result, "PHYSICAL_GOOD") {
		t.Error("expected product type")
	}
	if !strings.Contains(result, "Premium Italian leather") {
		t.Error("expected description snippet")
	}
	if !strings.Contains(result, "listings_get tool") {
		t.Error("expected usage guidance")
	}
}

func TestFormatProductCatalog_MultipleItems(t *testing.T) {
	listings := []ListingSummary{
		{Slug: "item-1", Title: "Item One", Price: "100", CoinType: "ETH"},
		{Slug: "item-2", Title: "Item Two", Price: "200", CoinType: "BTC"},
		{Slug: "item-3", Title: "Item Three", Price: "300", CoinType: "SOL"},
	}
	result := FormatProductCatalog(listings)

	if !strings.Contains(result, "3 items") {
		t.Error("expected 3 items in header")
	}
	for _, slug := range []string{"[item-1]", "[item-2]", "[item-3]"} {
		if !strings.Contains(result, slug) {
			t.Errorf("expected %s in output", slug)
		}
	}
}

func TestFormatProductCatalog_NoPriceOrCoinType(t *testing.T) {
	listings := []ListingSummary{
		{Slug: "free-item", Title: "Free Item"},
	}
	result := FormatProductCatalog(listings)

	if strings.Contains(result, "| |") {
		t.Error("should not have empty price segment")
	}
	if !strings.Contains(result, "[free-item] Free Item") {
		t.Error("expected slug and title without trailing pipe")
	}
}

func TestFormatProductCatalog_NoProductType(t *testing.T) {
	listings := []ListingSummary{
		{Slug: "s1", Title: "T1", Price: "10", CoinType: "USD"},
	}
	result := FormatProductCatalog(listings)

	lines := strings.Split(result, "\n")
	itemLine := ""
	for _, l := range lines {
		if strings.HasPrefix(l, "- [s1]") {
			itemLine = l
			break
		}
	}
	count := strings.Count(itemLine, "|")
	if count > 1 {
		t.Errorf("expected at most 1 pipe for price, got line: %s", itemLine)
	}
}

func TestFormatProductCatalog_DescriptionTruncation(t *testing.T) {
	longDesc := strings.Repeat("word ", 30)
	listings := []ListingSummary{
		{Slug: "s1", Title: "T1", Description: longDesc, Price: "10", CoinType: "USD"},
	}
	result := FormatProductCatalog(listings)

	lines := strings.Split(result, "\n")
	for _, l := range lines {
		if strings.HasPrefix(l, "- [s1]") {
			if len(l) > 300 {
				t.Errorf("line too long (%d chars), description not truncated", len(l))
			}
			if !strings.Contains(l, "...") {
				t.Error("expected truncation ellipsis")
			}
			break
		}
	}
}

func TestFormatProductCatalog_DescriptionNewlines(t *testing.T) {
	listings := []ListingSummary{
		{Slug: "s1", Title: "T1", Description: "Line one\nLine two\rLine three\ttab", Price: "10", CoinType: "USD"},
	}
	result := FormatProductCatalog(listings)

	lines := strings.Split(result, "\n")
	for _, l := range lines {
		if strings.HasPrefix(l, "- [s1]") {
			if strings.ContainsAny(l, "\n\r\t") {
				t.Error("description should not contain newlines or tabs")
			}
			break
		}
	}
}

func TestFormatProductCatalog_ExceedsMaxListings(t *testing.T) {
	listings := make([]ListingSummary, 210)
	for i := range listings {
		listings[i] = ListingSummary{
			Slug:  "item",
			Title: "Item",
			Price: "1",
		}
	}
	result := FormatProductCatalog(listings)

	if !strings.Contains(result, "210 items") {
		t.Error("header should show total count 210")
	}
	if !strings.Contains(result, "10 more products not shown") {
		t.Error("expected overflow notice for 10 extra items")
	}

	itemCount := strings.Count(result, "- [item]")
	if itemCount != 200 {
		t.Errorf("expected 200 item lines, got %d", itemCount)
	}
}

func TestTruncateDescription(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		maxLen int
		want   string
	}{
		{"empty", "", 80, ""},
		{"short", "hello world", 80, "hello world"},
		{"exact", strings.Repeat("a", 80), 80, strings.Repeat("a", 80)},
		{"truncated at word", "The quick brown fox jumps over the lazy dog and keeps running far away into the sunset", 50, "The quick brown fox jumps over the lazy dog and..."},
		{"whitespace normalized", "  hello   world  ", 80, "hello world"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := truncateDescription(tt.input, tt.maxLen)
			if got != tt.want {
				t.Errorf("truncateDescription(%q, %d) = %q, want %q", tt.input, tt.maxLen, got, tt.want)
			}
		})
	}
}
