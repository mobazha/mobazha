// SPDX-License-Identifier: MPL-2.0
// Copyright (c) 2026 fengzie and the respective contributors.

package core

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"

	internaldb "github.com/mobazha/mobazha/internal/database"
	"github.com/mobazha/mobazha/pkg/contracts"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeTranslationEngine stands in for a paid engine: it records what it was
// asked to do, so a test can assert that a cached text never reaches it.
type fakeTranslationEngine struct {
	name    string
	prefix  string
	failOn  string
	mu      sync.Mutex
	seen    []contracts.TranslationRequest
}

func (f *fakeTranslationEngine) Name() string { return f.name }

// A stand-in for a model: it can be told which terms to leave alone.
func (f *fakeTranslationEngine) SupportsGlossary() bool { return true }

func (f *fakeTranslationEngine) Translate(_ context.Context, req contracts.TranslationRequest) (string, error) {
	f.mu.Lock()
	f.seen = append(f.seen, req)
	f.mu.Unlock()
	if f.failOn != "" && strings.Contains(req.Text, f.failOn) {
		return "", errors.New("engine is down")
	}
	return f.prefix + req.Text, nil
}

func (f *fakeTranslationEngine) calls() []contracts.TranslationRequest {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]contracts.TranslationRequest(nil), f.seen...)
}

func newTranslationService(t *testing.T, engine *fakeTranslationEngine, budget int64) *TranslationAppService {
	t.Helper()
	db := newTestDB(t)
	require.NoError(t, internaldb.MigrateTranslationModels(db))
	service := NewTranslationAppService(internaldb.NewGormTranslationStore(db))
	require.NoError(t, service.Configure(contracts.TranslationEngineConfig{
		Engine:            contracts.TranslationEngineOpenAICompatible,
		Endpoint:          "http://engine.invalid/v1",
		Model:             "test-model",
		MonthlyCharBudget: budget,
	}))
	// The engine is installed directly so no test reaches the network.
	service.mu.Lock()
	service.engine = engine
	service.mu.Unlock()
	return service
}

func TestTranslationTranslatesAndCaches(t *testing.T) {
	engine := &fakeTranslationEngine{name: "test:model", prefix: "zh:"}
	service := newTranslationService(t, engine, 0)
	ctx := context.Background()

	items := []contracts.TranslationItem{
		{ID: "title", Text: "Vintage turntable", SourceLang: "en"},
		{ID: "body", Text: "Serviced, new belt and stylus.", SourceLang: "en"},
	}

	first, err := service.Translate(ctx, "zh", items)
	require.NoError(t, err)
	require.Len(t, first, 2)
	assert.Equal(t, "zh:Vintage turntable", first[0].Text)
	assert.True(t, first[0].Translated)
	assert.False(t, first[0].Cached, "the first answer is not a cache hit")
	assert.Equal(t, "test:model", first[0].Engine)
	assert.Len(t, engine.calls(), 2)

	// The same text in a later request must not be paid for again.
	second, err := service.Translate(ctx, "zh", items)
	require.NoError(t, err)
	require.Len(t, second, 2)
	assert.Equal(t, "zh:Vintage turntable", second[0].Text)
	assert.True(t, second[0].Cached)
	assert.Len(t, engine.calls(), 2, "a cache hit must not reach the engine")

	// The glossary and the target language reach the engine only on a miss.
	third, err := service.Translate(ctx, "ja", items[:1])
	require.NoError(t, err)
	require.Len(t, third, 1)
	calls := engine.calls()
	require.Len(t, calls, 3)
	assert.Equal(t, "ja", calls[2].TargetLang)
	assert.Equal(t, "en", calls[2].SourceLang)
}

func TestTranslationWithoutAnEngineReturnsTheOriginal(t *testing.T) {
	db := newTestDB(t)
	require.NoError(t, internaldb.MigrateTranslationModels(db))
	service := NewTranslationAppService(internaldb.NewGormTranslationStore(db))

	assert.False(t, service.Enabled())
	assert.False(t, service.LanguageSupported("zh"))

	items := []contracts.TranslationItem{{ID: "title", Text: "Vintage turntable", SourceLang: "en"}}
	results, err := service.Translate(context.Background(), "zh", items)
	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.Equal(t, "Vintage turntable", results[0].Text)
	assert.False(t, results[0].Translated)
}

func TestTranslationSkipsTextThatNeedsNoEngine(t *testing.T) {
	engine := &fakeTranslationEngine{name: "test:model", prefix: "zh:"}
	service := newTranslationService(t, engine, 0)
	ctx := context.Background()

	results, err := service.Translate(ctx, "zh", []contracts.TranslationItem{
		// Already in the target language.
		{ID: "same", Text: "复古唱机", SourceLang: "zh"},
		// Numbers and symbols read the same everywhere.
		{ID: "numbers", Text: "42 / 3.5", SourceLang: "en"},
		// Nothing to translate.
		{ID: "blank", Text: "   ", SourceLang: "en"},
	})
	require.NoError(t, err)
	assert.Empty(t, engine.calls(), "nothing here is worth paying an engine for")
	for _, result := range results {
		assert.False(t, result.Translated)
	}
	assert.Equal(t, "42 / 3.5", results[1].Text)
}

func TestTranslationStopsAtTheMonthlyBudget(t *testing.T) {
	engine := &fakeTranslationEngine{name: "test:model", prefix: "zh:"}
	// Ten characters of allowance against a twenty-character sentence.
	service := newTranslationService(t, engine, 10)

	results, err := service.Translate(context.Background(), "zh", []contracts.TranslationItem{
		{ID: "title", Text: "Vintage turntable", SourceLang: "en"},
	})
	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.Equal(t, "Vintage turntable", results[0].Text)
	assert.False(t, results[0].Translated)
	assert.Empty(t, engine.calls(), "the budget must be checked before the engine is called")
}

func TestTranslationKeepsThePageWhenOneTextFails(t *testing.T) {
	engine := &fakeTranslationEngine{name: "test:model", prefix: "zh:", failOn: "broken"}
	service := newTranslationService(t, engine, 0)

	results, err := service.Translate(context.Background(), "zh", []contracts.TranslationItem{
		{ID: "a", Text: "Vintage turntable", SourceLang: "en"},
		{ID: "b", Text: "broken text", SourceLang: "en"},
		{ID: "c", Text: "New stylus", SourceLang: "en"},
	})
	require.NoError(t, err)
	require.Len(t, results, 3)
	assert.Equal(t, "zh:Vintage turntable", results[0].Text)
	assert.Equal(t, "broken text", results[1].Text, "a failed text keeps its own words")
	assert.False(t, results[1].Translated)
	assert.Equal(t, "zh:New stylus", results[2].Text)
}

func TestTranslationCountsWhatItSpent(t *testing.T) {
	engine := &fakeTranslationEngine{name: "test:model", prefix: "zh:"}
	service := newTranslationService(t, engine, 0)

	_, err := service.Translate(context.Background(), "zh", []contracts.TranslationItem{
		{ID: "title", Text: "Vintage turntable", SourceLang: "en"},
	})
	require.NoError(t, err)

	month := service.now().UTC().Format("2006-01")
	used, err := service.store.Usage(context.Background(), month)
	require.NoError(t, err)
	assert.Equal(t, int64(len("Vintage turntable")), used)

	// A cache hit is free, so the counter must not move.
	_, err = service.Translate(context.Background(), "zh", []contracts.TranslationItem{
		{ID: "title", Text: "Vintage turntable", SourceLang: "en"},
	})
	require.NoError(t, err)
	used, err = service.store.Usage(context.Background(), month)
	require.NoError(t, err)
	assert.Equal(t, int64(len("Vintage turntable")), used)
}
