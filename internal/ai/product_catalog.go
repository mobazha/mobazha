package ai

import (
	"fmt"
	"strings"
)

const maxCatalogListings = 200

// ListingSummary is a lightweight product info struct for AI context injection.
type ListingSummary struct {
	Slug        string
	Title       string
	Description string
	Price       string
	CoinType    string
	Status      string
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

	b.WriteString(fmt.Sprintf("Your store's product catalog (%d items):\n", count))

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
	if len(s) <= maxLen {
		return s
	}
	truncated := s[:maxLen]
	if idx := strings.LastIndexByte(truncated, ' '); idx > maxLen/2 {
		truncated = truncated[:idx]
	}
	return truncated + "..."
}
