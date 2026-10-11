// SPDX-License-Identifier: MPL-2.0
// Copyright (c) 2026 fengzie and the respective contributors.

package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/mobazha/mobazha/pkg/contracts"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// translationTestNode satisfies only the translation provider; anything else on
// NodeService is never reached by this handler.
type translationTestNode struct {
	contracts.NodeService
	service contracts.TranslationService
}

func (n *translationTestNode) Translation() contracts.TranslationService { return n.service }

// IdentityInfo is called to key the rate limiter, so it must answer even when
// there is no identity: a storefront visitor has none.
func (n *translationTestNode) IdentityInfo() contracts.IdentityService { return nil }

// stubTranslationService answers every text with a marker, so a test can see
// what reached the service without an engine.
type stubTranslationService struct {
	supported   bool
	lastTarget  string
	lastItems   []contracts.TranslationItem
	translateFn func(targetLang string, items []contracts.TranslationItem) []contracts.TranslationResult
}

func (s *stubTranslationService) Translate(_ context.Context, targetLang string, items []contracts.TranslationItem) ([]contracts.TranslationResult, error) {
	s.lastTarget = targetLang
	s.lastItems = items
	if s.translateFn != nil {
		return s.translateFn(targetLang, items), nil
	}
	out := make([]contracts.TranslationResult, 0, len(items))
	for _, item := range items {
		out = append(out, contracts.TranslationResult{
			ID: item.ID, Text: targetLang + ":" + item.Text, Translated: true,
		})
	}
	return out, nil
}

func (s *stubTranslationService) LanguageSupported(string) bool { return s.supported }

func translateRequest(t *testing.T, node *translationTestNode, body string) *httptest.ResponseRecorder {
	t.Helper()
	gateway := &Gateway{}
	req := httptest.NewRequest(http.MethodPost, "/v1/translate", strings.NewReader(body))
	req = req.WithContext(context.WithValue(req.Context(), nodeContextKey, contracts.NodeService(node)))
	rr := httptest.NewRecorder()
	gateway.handleTranslate(rr, req)
	return rr
}

func decodeTranslateResponse(t *testing.T, rr *httptest.ResponseRecorder) contracts.TranslationResponseDTO {
	t.Helper()
	var envelope struct {
		Data contracts.TranslationResponseDTO `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &envelope))
	return envelope.Data
}

func TestTranslateHandlerReturnsTheRequestedLanguage(t *testing.T) {
	service := &stubTranslationService{supported: true}
	node := &translationTestNode{service: service}

	rr := translateRequest(t, node, `{"targetLang":"ja","items":[{"id":"t1","text":"Vintage turntable","sourceLang":"en"}]}`)
	require.Equal(t, http.StatusOK, rr.Code)

	answer := decodeTranslateResponse(t, rr)
	assert.Equal(t, "ja", answer.TargetLang)
	require.Len(t, answer.Results, 1)
	assert.Equal(t, "ja:Vintage turntable", answer.Results[0].Text)
	assert.True(t, answer.Results[0].Translated)
	assert.Equal(t, "t1", answer.Results[0].ID, "the caller's own id comes back")
	assert.Equal(t, "ja", service.lastTarget)
}

func TestTranslateHandlerHandsBackTheOriginalWhenNoEngineIsConfigured(t *testing.T) {
	// A node with no engine answers LanguageSupported false, which is the
	// normal state of a freshly installed node.
	service := &stubTranslationService{supported: false}
	node := &translationTestNode{service: service}

	rr := translateRequest(t, node, `{"targetLang":"ja","items":[{"id":"t1","text":"Vintage turntable"}]}`)
	require.Equal(t, http.StatusOK, rr.Code)

	answer := decodeTranslateResponse(t, rr)
	require.Len(t, answer.Results, 1)
	assert.Equal(t, "Vintage turntable", answer.Results[0].Text)
	assert.False(t, answer.Results[0].Translated)
	assert.Empty(t, service.lastItems, "nothing reaches the service when the node has no engine")
}

func TestTranslateHandlerRefusesWhatItCannotRender(t *testing.T) {
	node := &translationTestNode{service: &stubTranslationService{supported: true}}

	missingTarget := translateRequest(t, node, `{"items":[{"id":"t1","text":"hi"}]}`)
	assert.Equal(t, http.StatusBadRequest, missingTarget.Code)

	tooMany := make([]string, 0, translationMaxItems+1)
	for i := 0; i <= translationMaxItems; i++ {
		tooMany = append(tooMany, `{"id":"t`+string(rune('a'+i%26))+`","text":"x"}`)
	}
	overItemLimit := translateRequest(t, node,
		`{"targetLang":"zh","items":[`+strings.Join(tooMany, ",")+`]}`)
	assert.Equal(t, http.StatusBadRequest, overItemLimit.Code)

	long := strings.Repeat("あ", translationMaxTextRunes+1)
	overTextLimit := translateRequest(t, node,
		`{"targetLang":"zh","items":[{"id":"t1","text":"`+long+`"}]}`)
	assert.Equal(t, http.StatusBadRequest, overTextLimit.Code)

	noTarget := translateRequest(t, node, `{"targetLang":"zh","items":[]}`)
	assert.Equal(t, http.StatusOK, noTarget.Code, "an empty page is not an error")
}

func TestTranslateLimiterCountsPerCaller(t *testing.T) {
	limiter := newTranslationLimiter(2, time.Minute)
	assert.True(t, limiter.allow("addr:10.0.0.1"))
	assert.True(t, limiter.allow("addr:10.0.0.1"))
	assert.False(t, limiter.allow("addr:10.0.0.1"), "the third call in the window is refused")
	assert.True(t, limiter.allow("addr:10.0.0.2"), "a different caller has its own budget")

	// An unlimited limiter (limit <= 0) lets everything through.
	assert.True(t, newTranslationLimiter(0, time.Minute).allow("addr:10.0.0.1"))
}
