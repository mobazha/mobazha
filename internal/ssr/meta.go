package ssr

import (
	"bytes"
	"fmt"
	"net/url"
	"strings"
)

// metaBlock is the HTML fragment injected into <head> for crawler-facing pages.
type metaParams struct {
	OGType       string
	Title        string
	Description  string
	CanonicalURL string
	ImageURL     string
	SiteName     string
	TwitterCard  string
	OEmbedURL    string
	JSONLD       string
}

// injectMeta takes the cached SPA index.html and injects OG/Twitter/JSON-LD
// meta tags into <head>. It replaces the first occurrence of </head>.
func (h *SSRHandler) injectProductMeta(product *ProductData) ([]byte, error) {
	imageURL := ""
	if product.ImageHash != "" {
		imageURL = fmt.Sprintf("https://%s/v1/media/images/%s", h.domain, product.ImageHash)
	}

	desc := product.ShortDescription
	if desc == "" {
		desc = truncate(product.Description, 200)
	}

	canonicalURL := fmt.Sprintf("https://%s/product/%s", h.domain, product.Slug)
	oembedURL := fmt.Sprintf("https://%s/api/oembed?url=%s&format=json",
		h.domain, url.QueryEscape(canonicalURL))

	jsonLD := buildProductJSONLD(product, canonicalURL, imageURL)

	return h.renderMetaHTML(metaParams{
		OGType:       "product",
		Title:        product.Title,
		Description:  desc,
		CanonicalURL: canonicalURL,
		ImageURL:     imageURL,
		SiteName:     siteName(h.domain),
		TwitterCard:  "summary_large_image",
		OEmbedURL:    oembedURL,
		JSONLD:       jsonLD,
	})
}

func (h *SSRHandler) injectProfileMeta(profile *ProfileData) ([]byte, error) {
	imageURL := ""
	if profile.AvatarHash != "" {
		imageURL = fmt.Sprintf("https://%s/v1/media/images/%s", h.domain, profile.AvatarHash)
	}

	desc := truncate(profile.About, 200)
	if desc == "" {
		desc = fmt.Sprintf("Shop at %s on Mobazha", displayName(profile))
	}

	peerIDOrName := profile.PeerID
	if profile.Name != "" {
		peerIDOrName = profile.Name
	}

	canonicalURL := fmt.Sprintf("https://%s/store/%s", h.domain, profile.PeerID)
	oembedURL := fmt.Sprintf("https://%s/api/oembed?url=%s&format=json",
		h.domain, url.QueryEscape(canonicalURL))

	jsonLD := buildProfileJSONLD(profile, canonicalURL, imageURL, h.domain)

	return h.renderMetaHTML(metaParams{
		OGType:       "profile",
		Title:        peerIDOrName + " — Mobazha Store",
		Description:  desc,
		CanonicalURL: canonicalURL,
		ImageURL:     imageURL,
		SiteName:     siteName(h.domain),
		TwitterCard:  "summary",
		OEmbedURL:    oembedURL,
		JSONLD:       jsonLD,
	})
}

// buildMetaBlock constructs the meta HTML fragment using string building
// instead of html/template to avoid context-aware escaping inside <script>.
func buildMetaBlock(p metaParams) []byte {
	var b strings.Builder
	b.WriteString("\n<!-- SSR Meta Injection -->\n")
	b.WriteString(fmt.Sprintf(`<meta property="og:type" content="%s">`, attrEscape(p.OGType)))
	b.WriteByte('\n')
	b.WriteString(fmt.Sprintf(`<meta property="og:title" content="%s">`, attrEscape(p.Title)))
	b.WriteByte('\n')
	b.WriteString(fmt.Sprintf(`<meta property="og:description" content="%s">`, attrEscape(p.Description)))
	b.WriteByte('\n')
	b.WriteString(fmt.Sprintf(`<meta property="og:url" content="%s">`, attrEscape(p.CanonicalURL)))
	b.WriteByte('\n')
	if p.ImageURL != "" {
		b.WriteString(fmt.Sprintf(`<meta property="og:image" content="%s">`, attrEscape(p.ImageURL)))
		b.WriteByte('\n')
		b.WriteString(`<meta property="og:image:width" content="600">`)
		b.WriteByte('\n')
		b.WriteString(`<meta property="og:image:height" content="600">`)
		b.WriteByte('\n')
	}
	b.WriteString(fmt.Sprintf(`<meta property="og:site_name" content="%s">`, attrEscape(p.SiteName)))
	b.WriteByte('\n')
	b.WriteString(fmt.Sprintf(`<meta name="twitter:card" content="%s">`, attrEscape(p.TwitterCard)))
	b.WriteByte('\n')
	b.WriteString(fmt.Sprintf(`<meta name="twitter:title" content="%s">`, attrEscape(p.Title)))
	b.WriteByte('\n')
	b.WriteString(fmt.Sprintf(`<meta name="twitter:description" content="%s">`, attrEscape(p.Description)))
	b.WriteByte('\n')
	if p.ImageURL != "" {
		b.WriteString(fmt.Sprintf(`<meta name="twitter:image" content="%s">`, attrEscape(p.ImageURL)))
		b.WriteByte('\n')
	}
	b.WriteString(fmt.Sprintf(`<link rel="canonical" href="%s">`, attrEscape(p.CanonicalURL)))
	b.WriteByte('\n')
	b.WriteString(fmt.Sprintf(`<link rel="alternate" type="application/json+oembed" href="%s" title="%s">`,
		attrEscape(p.OEmbedURL), attrEscape(p.Title)))
	b.WriteByte('\n')
	b.WriteString(`<script type="application/ld+json">`)
	b.WriteString(p.JSONLD)
	b.WriteString("</script>\n")
	b.WriteString("<!-- /SSR Meta Injection -->\n")
	return []byte(b.String())
}

func (h *SSRHandler) renderMetaHTML(p metaParams) ([]byte, error) {
	injection := buildMetaBlock(p)
	headClose := []byte("</head>")
	idx := bytes.Index(h.spaHTML, headClose)
	if idx < 0 {
		return h.spaHTML, nil
	}

	result := make([]byte, 0, len(h.spaHTML)+len(injection))
	result = append(result, h.spaHTML[:idx]...)
	result = append(result, injection...)
	result = append(result, h.spaHTML[idx:]...)
	return result, nil
}

// attrEscape escapes a string for safe use in an HTML attribute value.
func attrEscape(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, `"`, "&quot;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	return s
}

func buildProductJSONLD(p *ProductData, url, imageURL string) string {
	var sb strings.Builder
	sb.WriteString(`{"@context":"https://schema.org","@type":"Product"`)
	sb.WriteString(`,"name":` + jsonString(p.Title))
	if p.Description != "" {
		sb.WriteString(`,"description":` + jsonString(truncate(p.Description, 500)))
	}
	if imageURL != "" {
		sb.WriteString(`,"image":` + jsonString(imageURL))
	}
	sb.WriteString(`,"url":` + jsonString(url))
	if p.Price != "" && p.CurrencyCode != "" {
		sb.WriteString(`,"offers":{"@type":"Offer","price":` + jsonString(p.Price))
		sb.WriteString(`,"priceCurrency":` + jsonString(p.CurrencyCode))
		sb.WriteString(`,"availability":"https://schema.org/InStock"}`)
	}
	if p.VendorName != "" {
		sb.WriteString(`,"brand":{"@type":"Brand","name":` + jsonString(p.VendorName) + `}`)
	}
	sb.WriteString(`}`)
	return sb.String()
}

func buildProfileJSONLD(p *ProfileData, url, imageURL, domain string) string {
	var sb strings.Builder
	sb.WriteString(`{"@context":"https://schema.org","@type":"Organization"`)
	sb.WriteString(`,"name":` + jsonString(displayName(p)))
	if p.About != "" {
		sb.WriteString(`,"description":` + jsonString(truncate(p.About, 500)))
	}
	sb.WriteString(`,"url":` + jsonString(url))
	if imageURL != "" {
		sb.WriteString(`,"logo":` + jsonString(imageURL))
	}
	sb.WriteString(`}`)
	return sb.String()
}

func displayName(p *ProfileData) string {
	if p.Name != "" {
		return p.Name
	}
	if p.Handle != "" {
		return p.Handle
	}
	if len(p.PeerID) > 12 {
		return p.PeerID[:8] + "…" + p.PeerID[len(p.PeerID)-4:]
	}
	return p.PeerID
}

func siteName(domain string) string {
	if domain != "" {
		return domain
	}
	return "Mobazha"
}

func truncate(s string, maxLen int) string {
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	return string(runes[:maxLen-1]) + "…"
}

// jsonString returns a JSON-encoded string value (with quotes and escaping).
func jsonString(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `"`, `\"`)
	s = strings.ReplaceAll(s, "\n", `\n`)
	s = strings.ReplaceAll(s, "\r", `\r`)
	s = strings.ReplaceAll(s, "\t", `\t`)
	s = strings.ReplaceAll(s, "<", `\u003c`)
	s = strings.ReplaceAll(s, ">", `\u003e`)
	s = strings.ReplaceAll(s, "&", `\u0026`)
	return `"` + s + `"`
}
