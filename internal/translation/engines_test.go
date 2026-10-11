// SPDX-License-Identifier: MPL-2.0
// Copyright (c) 2026 fengzie and the respective contributors.

package translation

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/go-resty/resty/v2"
	"github.com/mobazha/mobazha/pkg/contracts"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestOpenAICompatibleSendsTheTextWithItsPrompt checks the wire the node uses
// against a self-hosted or hosted chat model: a request the model can act on,
// and an answer the node can read back.
func TestOpenAICompatibleSendsTheTextWithItsPrompt(t *testing.T) {
	type chatEntry struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	}
	type chatRequest struct {
		Model    string      `json:"model"`
		Messages []chatEntry `json:"messages"`
	}

	var received chatRequest
	var path string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path = r.URL.Path
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &received)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"role":"assistant","content":"  复古唱机  "}}]}`))
	}))
	defer server.Close()

	engine := &OpenAICompatible{
		endpoint: server.URL + "/v1",
		model:    "qwen3-8b",
		client:   resty.New(),
	}

	out, err := engine.Translate(context.Background(), contracts.TranslationRequest{
		Text:       "Vintage turntable",
		SourceLang: "en",
		TargetLang: "zh-CN",
		Glossary:   []contracts.TranslationTerm{{Source: "SL-1200"}},
	})
	require.NoError(t, err)
	assert.Equal(t, "复古唱机", out, "the answer is trimmed")
	assert.Equal(t, "/v1/chat/completions", path)
	assert.Equal(t, "qwen3-8b", received.Model)
	require.Len(t, received.Messages, 2)
	assert.Equal(t, "system", received.Messages[0].Role)
	assert.Contains(t, received.Messages[0].Content, "Chinese (Simplified)")
	assert.Contains(t, received.Messages[0].Content, "SL-1200")
	assert.Equal(t, "Vintage turntable", received.Messages[1].Content)
}

func TestOpenAICompatibleReportsAnEngineFailure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"error":{"message":"model not found"}}`, http.StatusNotFound)
	}))
	defer server.Close()

	engine := &OpenAICompatible{endpoint: server.URL + "/v1", client: resty.New()}
	_, err := engine.Translate(context.Background(), contracts.TranslationRequest{Text: "hi", TargetLang: "zh"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "model not found")
}

func TestLibreTranslateSendsTheText(t *testing.T) {
	var body map[string]any
	var path string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path = r.URL.Path
		raw, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(raw, &body)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"translatedText":"复古唱机"}`))
	}))
	defer server.Close()

	engine := &LibreTranslate{endpoint: server.URL, client: resty.New()}
	out, err := engine.Translate(context.Background(), contracts.TranslationRequest{
		Text:       "Vintage turntable",
		TargetLang: "zh-TW",
	})
	require.NoError(t, err)
	assert.Equal(t, "复古唱机", out)
	assert.Equal(t, "/translate", path)
	assert.Equal(t, "Vintage turntable", body["q"])
	assert.Equal(t, "auto", body["source"], "an unstated source language is left to the engine")
	assert.Equal(t, "zh", body["target"], "a region is folded onto its language")
	assert.Equal(t, "text", body["format"])
}

func TestNewEngine(t *testing.T) {
	engine, err := NewEngine(contracts.TranslationEngineConfig{Engine: contracts.TranslationEngineNone})
	require.NoError(t, err)
	assert.Nil(t, engine, "translation off means no engine at all")

	_, err = NewEngine(contracts.TranslationEngineConfig{Engine: contracts.TranslationEngineOpenAICompatible})
	require.Error(t, err, "an engine without an endpoint is a configuration mistake")

	_, err = NewEngine(contracts.TranslationEngineConfig{Engine: "nope"})
	require.Error(t, err)

	engine, err = NewEngine(contracts.TranslationEngineConfig{
		Engine:   contracts.TranslationEngineOpenAICompatible,
		Endpoint: "http://127.0.0.1:11434/v1/",
		Model:    "qwen3:8b",
	})
	require.NoError(t, err)
	assert.Equal(t, "openai-compatible:qwen3:8b", engine.Name())
}

func TestChatCompletionsURLAcceptsBothForms(t *testing.T) {
	assert.Equal(t, "http://host/v1/chat/completions", chatCompletionsURL("http://host/v1"))
	assert.Equal(t, "http://host/v1/chat/completions", chatCompletionsURL("http://host/v1/chat/completions"))
}

// TestGoogleFreeSendsTheText checks the wire of the keyless endpoint: the query
// the service expects, and both of the shapes it answers with.
func TestGoogleFreeSendsTheText(t *testing.T) {
	var received url.Values
	var body string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		received = r.URL.Query()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(body))
	}))
	defer server.Close()

	engine := &GoogleFree{endpoint: server.URL, client: resty.New()}

	// The shape used when the caller named the source language: a list of pieces.
	body = `["Vintage turn","table"]`
	out, err := engine.Translate(context.Background(), contracts.TranslationRequest{
		Text:       "Vintage turntable",
		SourceLang: "en",
		TargetLang: "zh-CN",
	})
	require.NoError(t, err)
	assert.Equal(t, "Vintage turntable", out, "the pieces are joined without a separator")
	assert.Equal(t, "dict-chrome-ex", received.Get("client"))
	assert.Equal(t, "en", received.Get("sl"))
	assert.Equal(t, "zh-CN", received.Get("tl"), "our zh is Simplified Chinese")
	assert.Equal(t, "Vintage turntable", received.Get("q"))

	// The shape used when the source language was left to the service — which is
	// every call the storefront makes. These are bodies the live endpoint really
	// answered with: the translation, then the language it detected.
	body = `[["复古唱机","zh-CN"]]`
	out, err = engine.Translate(context.Background(), contracts.TranslationRequest{
		Text:       "Vintage turntable",
		TargetLang: "zh",
	})
	require.NoError(t, err)
	assert.Equal(t, "复古唱机", out)
	assert.Equal(t, "auto", received.Get("sl"))

	body = `[["Vintage turntable. Has been fully maintained.","zh-CN"]]`
	out, err = engine.Translate(context.Background(), contracts.TranslationRequest{
		Text:       "复古唱盘。已全面保养。",
		TargetLang: "en",
	})
	require.NoError(t, err)
	assert.Equal(t, "Vintage turntable. Has been fully maintained.", out,
		"the detected language is not part of the answer")

	// An older shape, still answered by some deployments.
	body = `{"sentences":["复古唱机"],"src":"zh-CN"}`
	out, err = engine.Translate(context.Background(), contracts.TranslationRequest{
		Text:       "Vintage turntable",
		TargetLang: "zh",
	})
	require.NoError(t, err)
	assert.Equal(t, "复古唱机", out)
}

func TestGoogleFreeKeepsTranslationsThatLookShort(t *testing.T) {
	// A one-word translation must survive: only a trailing language tag is
	// dropped, never a short piece of the answer itself.
	out, err := parseGoogleFreeAnswer([]byte(`[["Ok"]]`))
	require.NoError(t, err)
	assert.Equal(t, "Ok", out)

	out, err = parseGoogleFreeAnswer([]byte(`[["复古唱机","zh-CN"]]`))
	require.NoError(t, err)
	assert.Equal(t, "复古唱机", out)

	out, err = parseGoogleFreeAnswer([]byte(`[["part one ","part two","en"]]`))
	require.NoError(t, err)
	assert.Equal(t, "part one part two", out)

	_, err = parseGoogleFreeAnswer([]byte(`not json`))
	require.Error(t, err)
}

func TestGoogleFreeReportsAnEngineFailure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "Too Many Requests", http.StatusTooManyRequests)
	}))
	defer server.Close()

	engine := &GoogleFree{endpoint: server.URL, client: resty.New()}
	_, err := engine.Translate(context.Background(), contracts.TranslationRequest{Text: "hi", TargetLang: "zh"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "429")
}

func TestLanguageNameAndNormalize(t *testing.T) {
	assert.Equal(t, "zh", NormalizeLanguage("zh-CN"))
	assert.Equal(t, "zh", NormalizeLanguage("zh_TW"))
	assert.Equal(t, "Chinese (Simplified)", LanguageName("zh-Hans"))
	assert.Equal(t, "Japanese", LanguageName("ja"))
	assert.Equal(t, "xx", LanguageName("xx"), "an unknown code is passed through")
}

func TestTranslationPromptCarriesTheGlossary(t *testing.T) {
	prompt := TranslationPrompt("de", []contracts.TranslationTerm{
		{Source: "DJI Air 3"},
		{Source: "turntable", Target: "Plattenspieler"},
	})
	assert.Contains(t, prompt, "German")
	assert.Contains(t, prompt, "DJI Air 3")
	assert.Contains(t, prompt, `"turntable" as "Plattenspieler"`)
	assert.Contains(t, prompt, "Reply with the translation only")
}
