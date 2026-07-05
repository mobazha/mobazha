package payment

import (
	"strings"
)

var providerSecretPrefixes = []string{
	"sk_live_", "sk_test_", "rk_live_", "rk_test_", "pk_live_", "pk_test_",
}

// SanitizeProviderError removes provider credential-shaped tokens before an
// error is logged, persisted, or returned through an operational API.
func SanitizeProviderError(err error) string {
	if err == nil {
		return ""
	}
	return SanitizeProviderErrorMessage(err.Error())
}

// SanitizeProviderErrorMessage is the string form used for already-persisted
// summaries. It intentionally preserves ordinary diagnostic context.
func SanitizeProviderErrorMessage(message string) string {
	for _, prefix := range providerSecretPrefixes {
		searchFrom := 0
		for searchFrom < len(message) {
			idx := strings.Index(message[searchFrom:], prefix)
			if idx < 0 {
				break
			}
			idx += searchFrom
			end := idx + len(prefix)
			for end < len(message) && !strings.ContainsRune(" \t\r\n\"',", rune(message[end])) {
				end++
			}
			replacement := prefix + "***"
			message = message[:idx] + replacement + message[end:]
			searchFrom = idx + len(replacement)
		}
	}
	return message
}
