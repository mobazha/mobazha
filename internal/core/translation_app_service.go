package core

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/google/uuid"
	"github.com/mobazha/mobazha/internal/translation"
	"github.com/mobazha/mobazha/pkg/contracts"
	"github.com/mobazha/mobazha/pkg/models"
)

// translationCacheKeyVersion is part of every cache key. Bumping it retires a
// whole generation of cached text, which is what a prompt or engine change that
// invalidates old results needs.
const translationCacheKeyVersion = "v1"

// TranslationAppService is the node's translation facility: it decides what is
// worth sending to an engine, never pays twice for the same text, and stops
// when the operator's monthly allowance is used up.
//
// It never fails a caller because an engine did. Content that cannot be
// translated is returned in its own language, because a shopper is better served
// by a listing in the seller's words than by an error page.
type TranslationAppService struct {
	store contracts.TranslationStore
	now   func() time.Time

	// mu guards the engine, its configuration and the glossary: Configure is
	// called when the node starts (and by tests), while Translate runs from
	// request handlers.
	mu       sync.RWMutex
	engine   contracts.TranslationEngine
	cfg      contracts.TranslationEngineConfig
	glossary []contracts.TranslationTerm
}

var _ contracts.TranslationService = (*TranslationAppService)(nil)

func NewTranslationAppService(store contracts.TranslationStore) *TranslationAppService {
	return &TranslationAppService{store: store, now: time.Now}
}

// Configure installs the engine and the budget from the node's configuration.
// An unknown engine name or a missing endpoint is reported here, at startup,
// rather than on the first visitor's page.
func (s *TranslationAppService) Configure(cfg contracts.TranslationEngineConfig) error {
	engine, err := translation.NewEngine(cfg)
	if err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.engine = engine
	s.cfg = cfg
	return nil
}

// SetGlossary replaces the terms that must survive translation. It is set once
// from configuration; a term list is a property of the store, not of a request.
func (s *TranslationAppService) SetGlossary(terms []contracts.TranslationTerm) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.glossary = append([]contracts.TranslationTerm(nil), terms...)
}

// Enabled reports whether an engine is configured at all.
func (s *TranslationAppService) Enabled() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.engine != nil
}

// LanguageSupported reports whether this node will attempt a language. A client
// uses it to hide a language control rather than offer a dead end.
func (s *TranslationAppService) LanguageSupported(lang string) bool {
	return s.Enabled() && translation.NormalizeLanguage(lang) != ""
}

// engineName reports what is configured, for logs and diagnostics.
func (s *TranslationAppService) engineName() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.engine == nil {
		return contracts.TranslationEngineNone
	}
	return s.engine.Name()
}

// Translate returns every item in the requested language, in the order it was
// given. An item nothing could be done with comes back as its own text, so a
// caller can render the answer without checking for gaps.
func (s *TranslationAppService) Translate(ctx context.Context, targetLang string, items []contracts.TranslationItem) ([]contracts.TranslationResult, error) {
	results := make([]contracts.TranslationResult, len(items))
	for index, item := range items {
		results[index] = contracts.TranslationResult{ID: item.ID, Text: item.Text}
	}

	target := translation.NormalizeLanguage(targetLang)
	if target == "" || len(items) == 0 {
		return results, nil
	}

	engine, cfg, glossary := s.state()
	if engine == nil {
		// No engine: the caller gets its own content back. Nothing is logged
		// because this is the configured state of the node, not a failure.
		return results, nil
	}

	type pending struct {
		index    int
		cacheKey string
		item     contracts.TranslationItem
	}
	queue := make([]pending, 0, len(items))
	cacheKeys := make([]string, 0, len(items))
	for index, item := range items {
		if !worthTranslating(item.Text, item.SourceLang, target) {
			continue
		}
		key := translationCacheKey(item.SourceLang, target, item.Text)
		queue = append(queue, pending{index: index, cacheKey: key, item: item})
		cacheKeys = append(cacheKeys, key)
	}
	if len(queue) == 0 {
		return results, nil
	}

	cached, err := s.store.GetTranslations(ctx, cacheKeys)
	if err != nil {
		return nil, fmt.Errorf("read the translation cache: %w", err)
	}

	misses := make([]pending, 0, len(queue))
	for _, entry := range queue {
		if hit, ok := cached[entry.cacheKey]; ok && hit != nil {
			results[entry.index] = contracts.TranslationResult{
				ID:         entry.item.ID,
				Text:       hit.Text,
				Translated: true,
				Cached:     true,
				Engine:     hit.Engine,
			}
			continue
		}
		misses = append(misses, entry)
	}
	if len(misses) == 0 {
		return results, nil
	}

	month := s.now().UTC().Format("2006-01")
	needed := int64(0)
	for _, entry := range misses {
		needed += int64(len([]rune(entry.item.Text)))
	}
	if cfg.MonthlyCharBudget > 0 {
		used, err := s.store.Usage(ctx, month)
		if err != nil {
			return nil, fmt.Errorf("read the translation budget: %w", err)
		}
		if used+needed > cfg.MonthlyCharBudget {
			// The allowance is a promise to the operator, so the rest of this
			// page stays in its own language instead of spending past it.
			log.Warningf("Translation: monthly budget of %d characters is used up (%d used, %d needed); leaving %d text(s) untranslated",
				cfg.MonthlyCharBudget, used, needed, len(misses))
			return results, nil
		}
	}

	spent := int64(0)
	saved := make([]*models.Translation, 0, len(misses))
	// Housekeeping, on the first translation of each day: see
	// translation_retention.go for why it lives on the request path.
	s.pruneTranslations(ctx)
	for _, entry := range misses {
		request := contracts.TranslationRequest{
			Text:       entry.item.Text,
			SourceLang: translation.NormalizeLanguage(entry.item.SourceLang),
			TargetLang: target,
			Glossary:   glossary,
		}
		var protected []contracts.TranslationTerm
		if !engine.SupportsGlossary() {
			// The engine cannot be told to leave a term alone, so the terms are
			// hidden from it and put back afterwards. The cache still keys on
			// what the seller wrote, which is what a later request will ask for.
			request.Text, protected = protectGlossary(request.Text, glossary)
			request.Glossary = nil
		}

		translated, err := engine.Translate(ctx, request)
		if len(protected) > 0 {
			translated = restoreGlossary(translated, protected)
		}
		if err != nil {
			// One text failing does not fail the page; the item keeps its own
			// words and the next request tries again.
			log.Warningf("Translation: %s could not translate %q into %s: %s",
				engine.Name(), excerpt(entry.item.Text), target, err)
			continue
		}
		spent += int64(len([]rune(entry.item.Text)))
		results[entry.index] = contracts.TranslationResult{
			ID:         entry.item.ID,
			Text:       translated,
			Translated: true,
			Engine:     engine.Name(),
		}
		saved = append(saved, &models.Translation{
			ID:         uuid.New().String(),
			CacheKey:   entry.cacheKey,
			SourceLang: translation.NormalizeLanguage(entry.item.SourceLang),
			TargetLang: target,
			SourceHash: sourceHash(entry.item.Text),
			Text:       translated,
			Engine:     engine.Name(),
		})
	}

	// The answers are what the caller asked for; failing to cache them is worth
	// a line but not an error, and the next request simply translates again.
	if len(saved) > 0 {
		if err := s.store.SaveTranslations(ctx, saved); err != nil {
			log.Warningf("Translation: cannot cache %d translation(s): %s", len(saved), err)
		}
	}
	if spent > 0 {
		if err := s.store.AddUsage(ctx, month, spent); err != nil {
			log.Warningf("Translation: cannot record %d translated characters: %s", spent, err)
		}
	}
	return results, nil
}

// state reads the engine, its configuration and the glossary in one go, so a
// concurrent Configure cannot hand Translate a half-updated set.
func (s *TranslationAppService) state() (contracts.TranslationEngine, contracts.TranslationEngineConfig, []contracts.TranslationTerm) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.engine, s.cfg, s.glossary
}

// worthTranslating reports whether a text is worth sending to an engine.
//
// Three things are not: nothing at all, text already in the target language
// (the caller told us the source language), and text with no letters in it —
// a part number, a price or a size reads the same in every language and paying
// an engine to say so is waste.
func worthTranslating(text, sourceLang, target string) bool {
	if strings.TrimSpace(text) == "" {
		return false
	}
	if source := translation.NormalizeLanguage(sourceLang); source != "" && source == target {
		return false
	}
	return hasLetters(text)
}

func hasLetters(text string) bool {
	for _, r := range text {
		if unicode.IsLetter(r) {
			return true
		}
	}
	return false
}

// translationCacheKey is content-addressed: the same sentence asked for by a
// hundred visitors is translated once, and an edited sentence is a new key.
func translationCacheKey(sourceLang, targetLang, text string) string {
	sum := sha256.Sum256([]byte(strings.Join([]string{
		translationCacheKeyVersion,
		translation.NormalizeLanguage(sourceLang),
		targetLang,
		text,
	}, "\n")))
	return hex.EncodeToString(sum[:])
}

func sourceHash(text string) string {
	sum := sha256.Sum256([]byte(text))
	return hex.EncodeToString(sum[:])
}

// excerpt keeps a text readable in a log line.
func excerpt(text string) string {
	text = strings.Join(strings.Fields(text), " ")
	runes := []rune(text)
	if len(runes) > 60 {
		return string(runes[:60]) + "..."
	}
	return text
}
