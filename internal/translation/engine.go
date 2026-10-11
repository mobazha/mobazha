// SPDX-License-Identifier: MPL-2.0
// Copyright (c) 2026 fengzie and the respective contributors.

// Package translation speaks to the machine-translation engines a node may be
// pointed at.
//
// It knows nothing about listings, wanted requests or caching: it turns one
// string into another language. Everything else — the cache, the glossary, the
// monthly budget — lives in internal/core, so an operator can change engines
// without any of it moving.
package translation

import (
	"fmt"
	"strings"
	"time"

	"github.com/mobazha/mobazha/pkg/contracts"
)

// DefaultTimeout bounds one engine call. A stalled engine must not hold a page
// open, and the content it was translating is still readable untranslated.
const DefaultTimeout = 30 * time.Second

// NewEngine builds the engine an operator configured, or nil when translation
// is switched off.
func NewEngine(cfg contracts.TranslationEngineConfig) (contracts.TranslationEngine, error) {
	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = DefaultTimeout
	}

	switch strings.ToLower(strings.TrimSpace(cfg.Engine)) {
	case "", contracts.TranslationEngineNone:
		// No engine: callers get their own text back, which is what a node that
		// has not chosen a provider should do.
		return nil, nil

	case contracts.TranslationEngineOpenAICompatible:
		endpoint := strings.TrimRight(strings.TrimSpace(cfg.Endpoint), "/")
		if endpoint == "" {
			return nil, fmt.Errorf("translation engine %q needs an endpoint", cfg.Engine)
		}
		return &OpenAICompatible{
			endpoint: endpoint,
			apiKey:   strings.TrimSpace(cfg.APIKey),
			model:    strings.TrimSpace(cfg.Model),
			timeout:  timeout,
		}, nil

	case contracts.TranslationEngineLibreTranslate:
		endpoint := strings.TrimRight(strings.TrimSpace(cfg.Endpoint), "/")
		if endpoint == "" {
			return nil, fmt.Errorf("translation engine %q needs an endpoint", cfg.Engine)
		}
		return &LibreTranslate{
			endpoint: endpoint,
			apiKey:   strings.TrimSpace(cfg.APIKey),
			timeout:  timeout,
		}, nil

	case contracts.TranslationEngineGoogleFree:
		// An empty endpoint means the public one; an operator with their own
		// proxy points this at it and gets the same wire format.
		endpoint := strings.TrimRight(strings.TrimSpace(cfg.Endpoint), "/")
		if endpoint == "" {
			endpoint = defaultGoogleFreeEndpoint
		}
		return &GoogleFree{endpoint: endpoint, timeout: timeout}, nil

	default:
		return nil, fmt.Errorf("unknown translation engine %q", cfg.Engine)
	}
}

// LanguageName is how a language is named in a prompt. A model asked for
// "zh" answers worse than one asked for "Chinese (Simplified)", and an unknown
// code is passed through rather than guessed at.
func LanguageName(code string) string {
	base := NormalizeLanguage(code)
	if name, ok := languageNames[base]; ok {
		return name
	}
	return base
}

// NormalizeLanguage folds a locale onto the language an engine is asked for:
// "zh-CN", "zh_TW" and "zh" all reach a model as "zh".
func NormalizeLanguage(code string) string {
	code = strings.ToLower(strings.TrimSpace(code))
	code = strings.ReplaceAll(code, "_", "-")
	if index := strings.Index(code, "-"); index > 0 {
		code = code[:index]
	}
	return code
}

var languageNames = map[string]string{
	"en": "English", "zh": "Chinese (Simplified)", "ja": "Japanese", "ko": "Korean",
	"es": "Spanish", "fr": "French", "de": "German", "ru": "Russian", "pt": "Portuguese",
	"it": "Italian", "nl": "Dutch", "pl": "Polish", "tr": "Turkish", "ar": "Arabic",
	"hi": "Hindi", "th": "Thai", "vi": "Vietnamese", "id": "Indonesian", "ms": "Malay",
	"uk": "Ukrainian", "cs": "Czech", "el": "Greek", "ro": "Romanian", "hu": "Hungarian",
	"sv": "Swedish", "da": "Danish", "nb": "Norwegian", "fi": "Finnish", "he": "Hebrew",
	"fa": "Persian", "bn": "Bengali", "ta": "Tamil", "et": "Estonian", "lv": "Latvian",
	"lt": "Lithuanian", "sk": "Slovak", "sl": "Slovenian", "bg": "Bulgarian", "hr": "Croatian",
	"sr": "Serbian", "ca": "Catalan", "tl": "Tagalog", "sw": "Swahili", "ur": "Urdu",
}
