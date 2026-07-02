package ai

import (
	"fmt"
	"strings"
	"unicode/utf8"
)

const maxCatalogListings = 200

// ListingSummary is a lightweight product info struct for AI context injection.
// Price should be a human-readable amount (e.g. "25" not "2500" for $25 USD).
type ListingSummary struct {
	Slug        string
	Title       string
	Description string
	Price       string
	CoinType    string
	ProductType string
}

// FormatProductCatalog formats published listings into a compact text block
// suitable for injection into the AI system prompt.
// Returns empty string if listings is empty.
func FormatProductCatalog(listings []ListingSummary) string {
	if len(listings) == 0 {
		return ""
	}

	var b strings.Builder
	count := len(listings)
	shown := count
	if shown > maxCatalogListings {
		shown = maxCatalogListings
	}

	if count == 1 {
		b.WriteString("Your store's product catalog (1 item):\n")
	} else {
		b.WriteString(fmt.Sprintf("Your store's product catalog (%d items):\n", count))
	}

	for i := 0; i < shown; i++ {
		l := &listings[i]
		b.WriteString("- [")
		b.WriteString(l.Slug)
		b.WriteString("] ")
		b.WriteString(l.Title)

		if l.Price != "" && l.CoinType != "" {
			b.WriteString(" | ")
			b.WriteString(l.Price)
			b.WriteString(" ")
			b.WriteString(l.CoinType)
		}

		if l.ProductType != "" {
			b.WriteString(" | ")
			b.WriteString(l.ProductType)
		}

		desc := truncateDescription(l.Description, 80)
		if desc != "" {
			b.WriteString(" | ")
			b.WriteString(desc)
		}

		b.WriteByte('\n')
	}

	if count > maxCatalogListings {
		b.WriteString(fmt.Sprintf("... and %d more products not shown. Use listings_list_mine or listings_get tools for full details.\n", count-maxCatalogListings))
	}

	b.WriteString("\nYou already know the product catalog above. For common questions about products (what do I sell, prices, etc.), answer directly from this catalog. Use the listings_get tool only when you need full product details (complete description, variants, shipping info).")

	return b.String()
}

// FormatAmountForDisplay converts a raw amount string (in smallest currency unit)
// to a human-readable decimal string using the given divisibility.
// Example: FormatAmountForDisplay("2500", 2) => "25", FormatAmountForDisplay("10000000000000000", 18) => "0.01"
func FormatAmountForDisplay(amountStr string, divisibility uint) string {
	if divisibility == 0 || amountStr == "" || amountStr == "0" {
		return amountStr
	}
	for uint(len(amountStr)) <= divisibility {
		amountStr = "0" + amountStr
	}
	insertPos := len(amountStr) - int(divisibility)
	whole := amountStr[:insertPos]
	frac := strings.TrimRight(amountStr[insertPos:], "0")
	if frac == "" {
		return whole
	}
	return whole + "." + frac
}

// truncateDescription normalizes whitespace and truncates to maxLen runes,
// breaking at a word boundary when possible. Safe for multi-byte UTF-8.
func truncateDescription(s string, maxLen int) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	s = strings.Map(func(r rune) rune {
		if r == '\n' || r == '\r' || r == '\t' {
			return ' '
		}
		return r
	}, s)
	s = strings.Join(strings.Fields(s), " ")
	if utf8.RuneCountInString(s) <= maxLen {
		return s
	}
	runes := []rune(s)
	truncated := string(runes[:maxLen])
	if idx := strings.LastIndexByte(truncated, ' '); idx > len(string(runes[:maxLen/2])) {
		truncated = truncated[:idx]
	}
	return truncated + "..."
}
