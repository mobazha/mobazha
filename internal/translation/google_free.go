// SPDX-License-Identifier: MPL-2.0
// Copyright (c) 2026 fengzie and the respective contributors.

package translation

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/go-resty/resty/v2"
	"github.com/mobazha/mobazha/pkg/contracts"
)

// defaultGoogleFreeEndpoint is the endpoint the Chrome dictionary extension
// calls. It needs no account and no key.
const defaultGoogleFreeEndpoint = "https://clients5.google.com/translate_a/t"

// googleFreeUserAgent is sent because the endpoint answers 429 to a client that
// does not look like a browser.
const googleFreeUserAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 " +
	"(KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36"

// GoogleFree translates through a public Google endpoint: no key, no bill, and
// the same engine a visitor would get from Google Translate itself.
//
// Two things an operator should know before choosing it. It is not a published
// API, so it can start refusing a network without warning — the cache in front
// of it keeps traffic tiny, and an operator who needs a guarantee runs their own
// proxy and points Endpoint at it. And it cannot honour a glossary: it is a
// machine translation service, so the glossary reaches it only as the cache's
// and the prompt's business elsewhere.
type GoogleFree struct {
	endpoint string
	timeout  time.Duration

	// client is injectable so a test can point the engine at its own server.
	client *resty.Client
}

func (e *GoogleFree) Name() string { return contracts.TranslationEngineGoogleFree }

// SupportsGlossary is false: the endpoint takes text and a language, and offers
// no way to say "leave this word alone".
func (e *GoogleFree) SupportsGlossary() bool { return false }

// Translate sends one text and returns the service's answer.
func (e *GoogleFree) Translate(ctx context.Context, req contracts.TranslationRequest) (string, error) {
	query := url.Values{}
	query.Set("client", "dict-chrome-ex")
	query.Set("sl", googleSourceLang(req.SourceLang))
	query.Set("tl", googleTargetLang(req.TargetLang))
	query.Set("q", req.Text)

	response, err := e.httpClient().R().
		SetContext(ctx).
		SetHeader("User-Agent", googleFreeUserAgent).
		Get(e.endpoint + "?" + query.Encode())
	if err != nil {
		return "", fmt.Errorf("translation engine %s: %w", e.Name(), err)
	}
	if response.IsError() {
		return "", fmt.Errorf("translation engine %s answered %s: %s",
			e.Name(), response.Status(), bodyExcerpt(response.String()))
	}

	out, err := parseGoogleFreeAnswer(response.Body())
	if err != nil {
		return "", fmt.Errorf("translation engine %s: %w", e.Name(), err)
	}
	if out == "" {
		return "", fmt.Errorf("translation engine %s: answered with an empty translation", e.Name())
	}
	return out, nil
}

func (e *GoogleFree) httpClient() *resty.Client {
	if e.client != nil {
		return e.client
	}
	timeout := e.timeout
	if timeout <= 0 {
		timeout = DefaultTimeout
	}
	return resty.New().SetTimeout(timeout)
}

// parseGoogleFreeAnswer reads the shapes the endpoint answers with.
//
// Three of them are known, and they are not interchangeable:
//
//	["text"]                              the caller named the source language
//	[["text", "zh-CN"]]                   the caller left it to the service, and
//	                                      the second element is the language it
//	                                      detected — not part of the answer
//	{"sentences": ["text"], "src": "…"}   an older answer shape
//
// A translation arrives split over several entries in some of them, so the
// pieces are concatenated: joining them with a separator would invent
// punctuation the seller never wrote.
func parseGoogleFreeAnswer(body []byte) (string, error) {
	var pieces []string
	if err := json.Unmarshal(body, &pieces); err == nil {
		return strings.TrimSpace(strings.Join(pieces, "")), nil
	}

	var grouped [][]string
	if err := json.Unmarshal(body, &grouped); err == nil {
		joined := make([]string, 0, len(grouped))
		for _, group := range grouped {
			joined = append(joined, strings.Join(dropTrailingLanguageTag(group), ""))
		}
		return strings.TrimSpace(strings.Join(joined, "")), nil
	}

	var detected struct {
		Sentences []string `json:"sentences"`
	}
	if err := json.Unmarshal(body, &detected); err != nil {
		return "", fmt.Errorf("unreadable answer: %w", err)
	}
	return strings.TrimSpace(strings.Join(detected.Sentences, "")), nil
}

// dropTrailingLanguageTag removes the detected language the endpoint appends to
// a translation when it had to work out the source language itself. It only
// drops the last element, and only when it reads as a language tag, so a
// translation that merely looks short is never mistaken for one.
func dropTrailingLanguageTag(group []string) []string {
	if len(group) < 2 {
		return group
	}
	last := group[len(group)-1]
	if looksLikeLanguageTag(last) {
		return group[:len(group)-1]
	}
	return group
}

// looksLikeLanguageTag reports whether a value is shaped like "en" or "zh-CN".
func looksLikeLanguageTag(value string) bool {
	value = strings.TrimSpace(value)
	if len(value) < 2 || len(value) > 5 {
		return false
	}
	parts := strings.Split(value, "-")
	if len(parts) < 1 || len(parts) > 2 {
		return false
	}
	for _, part := range parts {
		if len(part) < 2 || len(part) > 3 {
			return false
		}
		for _, r := range part {
			if (r < 'a' || r > 'z') && (r < 'A' || r > 'Z') {
				return false
			}
		}
	}
	return true
}

// googleSourceLang names the source language, or leaves it to the service.
func googleSourceLang(code string) string {
	if normalized := NormalizeLanguage(code); normalized != "" {
		return normalized
	}
	return "auto"
}

// googleTargetLang maps our language codes onto the ones the endpoint expects.
// Simplified Chinese is the "zh" of our interface, and Google spells it out.
func googleTargetLang(code string) string {
	normalized := NormalizeLanguage(code)
	if normalized == "zh" {
		return "zh-CN"
	}
	return normalized
}
