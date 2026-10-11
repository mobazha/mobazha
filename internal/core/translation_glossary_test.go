// SPDX-License-Identifier: MPL-2.0
// Copyright (c) 2026 fengzie and the respective contributors.

package core

import (
	"context"
	"strings"
	"sync"
	"testing"

	internaldb "github.com/mobazha/mobazha/internal/database"
	"github.com/mobazha/mobazha/pkg/contracts"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestProtectGlossaryReplacesOnlyWhatIsThere(t *testing.T) {
	text, protected := protectGlossary("Vintage Technics SL-1200 turntable", []contracts.TranslationTerm{
		{Source: "SL-1200"},
		{Source: "Nikon F3"},
		{Source: "Technics"},
	})

	assert.Equal(t, "Vintage [[1]] [[2]] turntable", text)
	require.Len(t, protected, 2, "a term the text does not contain is not protected")
	assert.Equal(t, "Technics", protected[0].Source, "numbering follows the sentence")
	assert.Equal(t, "SL-1200", protected[1].Source)
}

func TestRestoreGlossaryUsesThePreferredSpelling(t *testing.T) {
	protected := []contracts.TranslationTerm{{Source: "turntable"}, {Source: "beanie", Target: "毛线帽"}}

	assert.Equal(t, "转盘 turntable", restoreGlossary("转盘 [[1]]", protected))
	assert.Equal(t, "毛线帽", restoreGlossary("[[2]]", protected), "a preferred spelling wins")
	assert.Equal(t, "[[ 2 ]]", restoreGlossary("[[ 2 ]]", []contracts.TranslationTerm{{Source: "beanie", Target: "毛线帽"}}))
	assert.Equal(t, "[[9]] stays", restoreGlossary("[[9]] stays", protected), "a placeholder we did not make is left alone")
	assert.Equal(t, "no placeholders", restoreGlossary("no placeholders", nil))
}

// nonGlossaryEngine stands in for a machine translation service: it cannot be
// told anything, so it receives whatever it is given.
type nonGlossaryEngine struct {
	mu   sync.Mutex
	seen []string
}

func (e *nonGlossaryEngine) Name() string { return "test:service" }

func (e *nonGlossaryEngine) SupportsGlossary() bool { return false }

func (e *nonGlossaryEngine) Translate(_ context.Context, req contracts.TranslationRequest) (string, error) {
	e.mu.Lock()
	e.seen = append(e.seen, req.Text)
	e.mu.Unlock()
	// A service that translates word by word, and has no idea what a bracket is.
	return "【译】" + strings.ReplaceAll(req.Text, "Vintage", "复古"), nil
}

func (e *nonGlossaryEngine) requests() []string {
	e.mu.Lock()
	defer e.mu.Unlock()
	return append([]string(nil), e.seen...)
}

func TestTranslationHidesTermsFromAnEngineThatCannotKeepThem(t *testing.T) {
	db := newTestDB(t)
	require.NoError(t, internaldb.MigrateTranslationModels(db))
	service := NewTranslationAppService(internaldb.NewGormTranslationStore(db))
	require.NoError(t, service.Configure(contracts.TranslationEngineConfig{
		Engine:   contracts.TranslationEngineOpenAICompatible,
		Endpoint: "http://engine.invalid/v1",
	}))

	engine := &nonGlossaryEngine{}
	service.mu.Lock()
	service.engine = engine
	service.mu.Unlock()
	service.SetGlossary([]contracts.TranslationTerm{
		{Source: "SL-1200"},
		{Source: "Technics", Target: "铁三角"},
	})

	results, err := service.Translate(context.Background(), "zh", []contracts.TranslationItem{
		{ID: "title", Text: "Vintage Technics SL-1200 turntable", SourceLang: "en"},
	})
	require.NoError(t, err)
	require.Len(t, results, 1)

	// The engine never saw the brand or the model number.
	require.Len(t, engine.requests(), 1)
	assert.Equal(t, "Vintage [[1]] [[2]] turntable", engine.requests()[0])

	// The caller gets them back — with the preferred spelling where there is one.
	assert.Equal(t, "【译】复古 铁三角 SL-1200 turntable", results[0].Text)
	assert.NotContains(t, results[0].Text, "[[", "no placeholder survives to the caller")
	assert.Contains(t, results[0].Text, "SL-1200")
	assert.Contains(t, results[0].Text, "铁三角")
}
