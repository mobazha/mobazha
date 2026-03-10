package ai

import (
	"strings"
	"testing"
	"unicode/utf8"
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
			Price:       "25",
			CoinType:    "USD",
			ProductType: "PHYSICAL_GOOD",
		},
	}
	result := FormatProductCatalog(listings)

	if !strings.Contains(result, "(1 item)") {
		t.Error("expected singular '1 item' in header")
	}
	if strings.Contains(result, "1 items") {
		t.Error("should not have '1 items' (bad grammar)")
	}
	if !strings.Contains(result, "[leather-wallet]") {
		t.Error("expected slug in brackets")
	}
	if !strings.Contains(result, "Handmade Leather Wallet") {
		t.Error("expected title")
	}
	if !strings.Contains(result, "25 USD") {
		t.Error("expected human-readable price")
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
		{Slug: "item-1", Title: "Item One", Price: "0.01", CoinType: "ETH"},
		{Slug: "item-2", Title: "Item Two", Price: "0.005", CoinType: "BTC"},
		{Slug: "item-3", Title: "Item Three", Price: "300", CoinType: "SOL"},
	}
	result := FormatProductCatalog(listings)

	if !strings.Contains(result, "(3 items)") {
		t.Error("expected '3 items' in header")
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
			if strings.ContainsAny(l, "\r\t") {
				t.Error("description should not contain carriage returns or tabs")
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

func TestTruncateDescription_ChineseUTF8(t *testing.T) {
	chinese := "这是一个很长的中文商品描述用来测试截断逻辑是否会在多字节字符中间切断导致乱码问题需要更多文字"
	result := truncateDescription(chinese, 20)

	if !utf8.ValidString(result) {
		t.Errorf("result is not valid UTF-8: %q", result)
	}
	runes := []rune(result)
	if len(runes) > 20+3 { // +3 for "..."
		t.Errorf("result too long: %d runes", len(runes))
	}
	if !strings.HasSuffix(result, "...") {
		t.Error("expected truncation ellipsis")
	}
}

func TestTruncateDescription_MixedCJKAndLatin(t *testing.T) {
	mixed := "Premium 手工皮革钱包 with RFID blocking 这是一段很长的混合中英文描述需要被截断"
	result := truncateDescription(mixed, 30)

	if !utf8.ValidString(result) {
		t.Errorf("result is not valid UTF-8: %q", result)
	}
	if !strings.HasSuffix(result, "...") {
		t.Error("expected truncation ellipsis")
	}
}

func TestTruncateDescription_Emoji(t *testing.T) {
	emoji := "🎒 Backpack special offer 🔥 Limited time only! Get yours before they're all gone! 🚀"
	result := truncateDescription(emoji, 30)

	if !utf8.ValidString(result) {
		t.Errorf("result is not valid UTF-8: %q", result)
	}
	if !strings.HasSuffix(result, "...") {
		t.Error("expected truncation ellipsis")
	}
}

func TestTruncateDescription_ShortChinese(t *testing.T) {
	short := "手工钱包"
	result := truncateDescription(short, 80)
	if result != short {
		t.Errorf("expected unchanged short Chinese text, got %q", result)
	}
}

func TestFormatAmountForDisplay(t *testing.T) {
	tests := []struct {
		name         string
		amountStr    string
		divisibility uint
		want         string
	}{
		{"zero divisibility", "100", 0, "100"},
		{"empty amount", "", 2, ""},
		{"zero amount", "0", 8, "0"},
		{"USD 25.00", "2500", 2, "25"},
		{"USD 25.50", "2550", 2, "25.5"},
		{"USD 0.99", "99", 2, "0.99"},
		{"USD 0.01", "1", 2, "0.01"},
		{"BTC 1.0", "100000000", 8, "1"},
		{"BTC 0.001", "100000", 8, "0.001"},
		{"BTC 0.00000001", "1", 8, "0.00000001"},
		{"ETH 0.01", "10000000000000000", 18, "0.01"},
		{"ETH 1.0", "1000000000000000000", 18, "1"},
		{"USDT 100.0", "100000000", 6, "100"},
		{"USDT 99.99", "99990000", 6, "99.99"},
		{"large amount", "1234567890", 2, "12345678.9"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FormatAmountForDisplay(tt.amountStr, tt.divisibility)
			if got != tt.want {
				t.Errorf("FormatAmountForDisplay(%q, %d) = %q, want %q", tt.amountStr, tt.divisibility, got, tt.want)
			}
		})
	}
}
