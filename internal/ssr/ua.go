package ssr

import "strings"

// crawlerFragments are User-Agent substrings that identify known crawlers
// and social media link preview bots. Matching is case-insensitive.
var crawlerFragments = []string{
	// Search engines
	"googlebot",
	"bingbot",
	"baiduspider",
	"duckduckbot",
	"yandex",
	"sogou",
	"ia_archiver",

	// Social platforms
	"twitterbot",
	"facebookexternalhit",
	"facebookcatalog",
	"linkedinbot",
	"slackbot",
	"telegrambot",
	"whatsapp",
	"discordbot",
	"pinterestbot",

	// Tools / validators
	"lighthouse",
	"google-structured-data-testing-tool",
	"w3c_validator",
}

// IsCrawler returns true when the User-Agent string belongs to a known
// search-engine crawler or social-media link-preview bot.
func IsCrawler(userAgent string) bool {
	lower := strings.ToLower(userAgent)
	for _, frag := range crawlerFragments {
		if strings.Contains(lower, frag) {
			return true
		}
	}
	return false
}
