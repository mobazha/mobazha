// SPDX-License-Identifier: MPL-2.0
// Copyright (c) 2026 fengzie and the respective contributors.

package api

import (
	"encoding/json"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/mobazha/mobazha/pkg/contracts"
	"github.com/mobazha/mobazha/pkg/response"
)

// The limits a caller of POST /v1/translate has to fit inside.
//
// Translation is a read-only endpoint that sends work to somebody else's
// server, so it is the one place a stranger could cost the operator money or
// get the node's engine rate-limited for everybody. The interface asks for what
// it can render — one page of cards, or one product's text — and says so when
// it is asked for more.
const (
	translationMaxItems      = 40
	translationMaxTextRunes  = 2000
	translationMaxTotalRunes = 20000
	// translationRequestsPerMinute is per caller, not per node.
	translationRequestsPerMinute = 60
)

// translationLimiter counts requests per caller in a fixed window.
//
// A page load makes one call for every text on it, so the limit is generous for
// a person and useless for a script. It is deliberately in memory: a node
// restarting loses its counters, which is a smaller problem than a database
// write on every translated page.
type translationLimiter struct {
	mu       sync.Mutex
	limit    int
	window   time.Duration
	counters map[string]*translationWindow
	lastSweep time.Time
}

type translationWindow struct {
	start time.Time
	count int
}

func newTranslationLimiter(limit int, window time.Duration) *translationLimiter {
	return &translationLimiter{
		limit:    limit,
		window:   window,
		counters: make(map[string]*translationWindow),
	}
}

func (l *translationLimiter) allow(key string) bool {
	if l == nil || l.limit <= 0 {
		return true
	}
	now := time.Now()

	l.mu.Lock()
	defer l.mu.Unlock()

	l.sweep(now)
	current, ok := l.counters[key]
	if !ok || now.Sub(current.start) >= l.window {
		l.counters[key] = &translationWindow{start: now, count: 1}
		return true
	}
	if current.count >= l.limit {
		return false
	}
	current.count++
	return true
}

// sweep drops windows that have expired, so a busy node does not accumulate one
// entry per visitor for the lifetime of the process.
func (l *translationLimiter) sweep(now time.Time) {
	if now.Sub(l.lastSweep) < l.window {
		return
	}
	l.lastSweep = now
	for key, entry := range l.counters {
		if entry == nil || now.Sub(entry.start) >= l.window {
			delete(l.counters, key)
		}
	}
}

// translationLimiterForCalls is the process-wide limiter behind /v1/translate.
var translationLimiterForCalls = newTranslationLimiter(translationRequestsPerMinute, time.Minute)

// translationCallerKey identifies who is asking: the authenticated peer when
// there is one, the network address otherwise. Translation is public — a
// visitor browsing a store has no identity to give — so the address is the only
// handle available for most callers.
func translationCallerKey(r *http.Request) string {
	if identity := getNodeService(r).IdentityInfo(); identity != nil {
		if peerID := strings.TrimSpace(identity.Identity().String()); peerID != "" {
			return "peer:" + peerID
		}
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil || host == "" {
		return "addr:" + r.RemoteAddr
	}
	return "addr:" + host
}

// translateRequestItem is one text the caller wants in another language. The id
// is the caller's own and comes back untouched, so it can line the answers up
// with the fields it is about to render.
type translateRequestItem struct {
	ID         string `json:"id"`
	Text       string `json:"text"`
	SourceLang string `json:"sourceLang"`
}

type translateRequestBody struct {
	TargetLang string                `json:"targetLang"`
	Items      []translateRequestItem `json:"items"`
}

// handleTranslate answers POST /v1/translate: the node's content translation,
// used by the storefront to show a product in the visitor's language.
//
// It is a read: nothing is written to the node's books, and a text nothing can
// translate (no engine configured, budget spent, an engine that refused) comes
// back as the caller sent it, flagged so the caller can render it as it is.
func (g *Gateway) handleTranslate(w http.ResponseWriter, r *http.Request) {
	provider, ok := getNodeService(r).(contracts.TranslationProvider)
	if !ok || provider.Translation() == nil {
		response.Error(w, http.StatusNotImplemented, response.CodeNotImplemented, "Translation not available")
		return
	}
	service := provider.Translation()

	if !translationLimiterForCalls.allow(translationCallerKey(r)) {
		response.Error(w, http.StatusTooManyRequests, response.CodeConflict,
			"Too many translation requests; please slow down")
		return
	}

	var body translateRequestBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		response.Error(w, http.StatusBadRequest, response.CodeBadRequest, "Invalid request body")
		return
	}

	targetLang := strings.TrimSpace(body.TargetLang)
	if targetLang == "" {
		response.Error(w, http.StatusBadRequest, response.CodeValidation, "targetLang is required")
		return
	}
	if !service.LanguageSupported(targetLang) {
		// No engine, or a language the node will not attempt. The caller gets
		// its own text back rather than an error: a page with untranslated
		// content is better than a page with nothing on it.
		response.Success(w, contracts.TranslationResponseDTO{
			TargetLang: targetLang,
			Results:    untranslatedResults(body.Items),
		})
		return
	}
	if len(body.Items) == 0 {
		response.Success(w, contracts.TranslationResponseDTO{TargetLang: targetLang, Results: []contracts.TranslationResult{}})
		return
	}
	if len(body.Items) > translationMaxItems {
		response.Error(w, http.StatusBadRequest, response.CodeValidation,
			"Too many texts in one request; send at most 40")
		return
	}

	total := 0
	items := make([]contracts.TranslationItem, 0, len(body.Items))
	for _, item := range body.Items {
		runes := utf8.RuneCountInString(item.Text)
		if runes > translationMaxTextRunes {
			response.Error(w, http.StatusBadRequest, response.CodeValidation,
				"One of the texts is longer than 2000 characters; send a shorter part of it")
			return
		}
		total += runes
		items = append(items, contracts.TranslationItem{
			ID:         item.ID,
			Text:       item.Text,
			SourceLang: item.SourceLang,
		})
	}
	if total > translationMaxTotalRunes {
		response.Error(w, http.StatusBadRequest, response.CodeValidation,
			"Too much text in one request; send at most 20000 characters")
		return
	}

	results, err := service.Translate(r.Context(), targetLang, items)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, response.CodeInternalError, "Internal server error")
		return
	}

	response.Success(w, contracts.TranslationResponseDTO{TargetLang: targetLang, Results: results})
}

// untranslatedResults echoes the caller's texts back, flagged as untouched.
func untranslatedResults(items []translateRequestItem) []contracts.TranslationResult {
	out := make([]contracts.TranslationResult, 0, len(items))
	for _, item := range items {
		out = append(out, contracts.TranslationResult{ID: item.ID, Text: item.Text})
	}
	return out
}
