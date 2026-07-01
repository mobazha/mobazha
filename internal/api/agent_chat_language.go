package api

import (
	"strings"
	"unicode"

	aipkg "github.com/mobazha/mobazha3.0/internal/ai"
)

func agentChatToolLanguage(chatCtx *aipkg.ChatContext, userMessage string) string {
	locale := ""
	if chatCtx != nil {
		locale = supportedAgentLanguageCode(chatCtx.Locale)
	}
	message := strings.TrimSpace(userMessage)
	if message != "" {
		hasHan := false
		hasLatin := false
		for _, r := range message {
			switch {
			case unicode.In(r, unicode.Hiragana, unicode.Katakana):
				return "ja"
			case unicode.In(r, unicode.Hangul):
				return "ko"
			case unicode.In(r, unicode.Han):
				hasHan = true
			case unicode.In(r, unicode.Cyrillic):
				return "ru"
			case unicode.In(r, unicode.Latin) && unicode.IsLetter(r):
				hasLatin = true
			}
		}
		if hasHan {
			return "zh"
		}
		if hasLatin {
			switch locale {
			case "en", "es", "fr", "de", "pt":
				return locale
			default:
				return "en"
			}
		}
	}
	if locale != "" {
		return locale
	}
	return "en"
}

func supportedAgentLanguageCode(locale string) string {
	normalized := strings.ToLower(strings.TrimSpace(locale))
	if idx := strings.IndexAny(normalized, "-_"); idx >= 0 {
		normalized = normalized[:idx]
	}
	switch normalized {
	case "en", "zh", "ja", "ko", "es", "fr", "de", "ru", "pt":
		return normalized
	default:
		return ""
	}
}
