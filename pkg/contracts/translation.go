// SPDX-License-Identifier: MPL-2.0
// Copyright (c) 2026 fengzie and the respective contributors.

package contracts

import (
	"context"
	"errors"
	"time"

	"github.com/mobazha/mobazha/pkg/models"
)

// The translation engines a node can be pointed at.
//
// The names are the values in mobazha.conf, so an operator picks an engine the
// same way they pick any other provider, and the node never hard-codes one:
// a hosted small model, a self-hosted OpenAI-compatible server (Ollama, vLLM,
// LM Studio) and LibreTranslate all speak through the same two implementations.
const (
	// TranslationEngineNone leaves content in its own language. It is the
	// default, because translating means sending a seller's words to somebody
	// else's server and paying for it, which is the node owner's decision.
	TranslationEngineNone = "none"
	// TranslationEngineOpenAICompatible covers every chat-completions endpoint:
	// hosted providers and self-hosted models alike.
	TranslationEngineOpenAICompatible = "openai-compatible"
	// TranslationEngineLibreTranslate is the self-hosted LibreTranslate API.
	TranslationEngineLibreTranslate = "libretranslate"
	// TranslationEngineGoogleFree uses the translation endpoint the Chrome
	// dictionary extension calls: no account, no key, no bill.
	//
	// It is a public endpoint rather than a published API, so it can change or
	// stop answering without notice. The cache in front of it keeps the traffic
	// small, and an operator who wants a promise instead of a convenience is
	// expected to run their own proxy and point Endpoint at it.
	TranslationEngineGoogleFree = "google-free"
)

var (
	// ErrTranslationEngineUnavailable means no engine is configured, so content
	// travels unchanged instead of failing.
	ErrTranslationEngineUnavailable = errors.New("no translation engine is configured")
	// ErrTranslationBudgetExceeded means this month's allowance is used up. It
	// is reported apart from a broken engine: nothing is wrong, the node has
	// simply spent what it was allowed to.
	ErrTranslationBudgetExceeded = errors.New("the monthly translation budget is used up")
)

// TranslationTerm is one term that must survive translation.
//
// Brand and model names are the reason this exists: a machine translator turns
// "DJI Air 3" into a place name, and no amount of prompt tuning fixes that in
// general. A term with an empty Target keeps whatever spelling the source used.
type TranslationTerm struct {
	Source string `json:"source"`
	Target string `json:"target,omitempty"`
}

// TranslationEngineConfig is the operator's choice of engine.
type TranslationEngineConfig struct {
	Engine   string `json:"engine"`
	Endpoint string `json:"endpoint,omitempty"`
	APIKey   string `json:"apiKey,omitempty"`
	Model    string `json:"model,omitempty"`
	// Timeout bounds one engine call. Zero means the engine's own default.
	Timeout time.Duration `json:"-"`
	// MonthlyCharBudget caps the characters this node sends to an engine in a
	// calendar month. Zero means unlimited, which is right for a self-hosted
	// engine that costs nothing per character.
	MonthlyCharBudget int64 `json:"monthlyCharBudget,omitempty"`
}

// TranslationRequest is one text on its way to an engine.
type TranslationRequest struct {
	Text string
	// SourceLang is an ISO code, or "" when the engine should detect it.
	SourceLang string
	TargetLang string
	Glossary   []TranslationTerm
}

// TranslationEngine turns text into another language. Implementations are the
// only place that knows how a particular provider is spoken to.
type TranslationEngine interface {
	// Name is what gets recorded against every cached row.
	Name() string
	// SupportsGlossary reports whether the engine can be told to leave a term
	// alone. A model can; a machine translation service cannot, and for those
	// the caller hides the terms behind placeholders instead.
	SupportsGlossary() bool
	Translate(ctx context.Context, req TranslationRequest) (string, error)
}

// TranslationStore caches finished translations and counts what has been spent.
// Implementations handle tenant scoping internally via database.Tx.
type TranslationStore interface {
	GetTranslations(ctx context.Context, cacheKeys []string) (map[string]*models.Translation, error)
	SaveTranslations(ctx context.Context, entries []*models.Translation) error
	AddUsage(ctx context.Context, month string, chars int64) error
	Usage(ctx context.Context, month string) (int64, error)
	// DeleteTranslationsOlderThan drops the entries written before a moment, so
	// a node that translates for years does not keep every wording it ever saw.
	DeleteTranslationsOlderThan(ctx context.Context, before time.Time) (int64, error)
}

// TranslationItem is one text a caller wants in another language. ID is opaque
// to the node and is only there so a caller can line the answers up with its
// own list.
type TranslationItem struct {
	ID         string `json:"id"`
	Text       string `json:"text"`
	SourceLang string `json:"sourceLang,omitempty"`
}

// TranslationResult is what came back for one item.
type TranslationResult struct {
	ID string `json:"id"`
	// Text is the translation, or the original text when nothing translated it.
	Text string `json:"text"`
	// Translated is false when Text is the input unchanged: no engine, budget
	// spent, or the text was already in the target language. A caller that
	// cares (to hide a "translated" label, say) reads this instead of comparing
	// strings.
	Translated bool `json:"translated"`
	// Cached reports that the text came from the cache, which is what makes a
	// second visit free.
	Cached bool   `json:"cached"`
	Engine string `json:"engine,omitempty"`
}

// TranslationService is the node's translation facility.
type TranslationService interface {
	// Translate returns every item in the requested language, in the same
	// order. Items nothing could be done with come back as their own text.
	Translate(ctx context.Context, targetLang string, items []TranslationItem) ([]TranslationResult, error)
	// LanguageSupported reports whether the node will attempt a language at
	// all, so a client can hide the control instead of offering a dead end.
	LanguageSupported(lang string) bool
}

// TranslationProvider exposes the node's translation facility, the way the
// wanted board exposes BuyRequests().
type TranslationProvider interface {
	Translation() TranslationService
}

// TranslationResponseDTO is the answer to POST /v1/translate.
//
// It echoes the language it was asked for, so a client can drop an answer that
// arrives after the visitor switched languages again.
type TranslationResponseDTO struct {
	TargetLang string              `json:"targetLang"`
	Results    []TranslationResult `json:"results"`
}
