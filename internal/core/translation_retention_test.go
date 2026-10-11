package core

import (
	"context"
	"testing"
	"time"

	internaldb "github.com/mobazha/mobazha/internal/database"
	"github.com/mobazha/mobazha/pkg/contracts"
	pkgdb "github.com/mobazha/mobazha/pkg/database"
	"github.com/mobazha/mobazha/pkg/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestTranslationCachePrunesOldEntriesOnceADay covers both halves of the policy:
// an entry older than the window goes, one inside it stays, and the sweep does
// not run again the same day.
func TestTranslationCachePrunesOldEntriesOnceADay(t *testing.T) {
	db := newTestDB(t)
	require.NoError(t, internaldb.MigrateTranslationModels(db))
	store := internaldb.NewGormTranslationStore(db)

	service := NewTranslationAppService(store)
	fixed := time.Date(2026, 10, 11, 12, 0, 0, 0, time.UTC)
	service.now = func() time.Time { return fixed }
	service.mu.Lock()
	service.engine = &fakeTranslationEngine{name: "test:model", prefix: "zh:"}
	service.mu.Unlock()
	lastPruneDay.Store("")

	insert := func(t *testing.T, cacheKey string, written time.Time) {
		t.Helper()
		require.NoError(t, db.Update(func(tx pkgdb.Tx) error {
			if err := tx.Save(&models.Translation{
				ID:         cacheKey,
				CacheKey:   cacheKey,
				TargetLang: "zh",
				SourceHash: cacheKey,
				Text:       "译文",
				Engine:     "test:model",
				CreatedAt:  written,
				UpdatedAt:  written,
			}); err != nil {
				return err
			}
			// autoUpdateTime stamps "now" on save, so a row is aged afterwards:
			// that is the only way to make one old on purpose.
			return tx.Read().Model(&models.Translation{}).
				Where("cache_key = ?", cacheKey).
				UpdateColumn("updated_at", written).Error
		}))
	}
	insert(t, "stale", fixed.Add(-translationRetention-time.Hour))
	insert(t, "fresh", fixed.Add(-time.Hour))

	// Any translation triggers the sweep.
	_, err := service.Translate(context.Background(), "zh", []contracts.TranslationItem{
		{ID: "ask", Text: "a turntable", SourceLang: "en"},
	})
	require.NoError(t, err)

	remaining, err := store.GetTranslations(context.Background(), []string{"stale", "fresh"})
	require.NoError(t, err)
	assert.NotContains(t, remaining, "stale", "an entry past the window is dropped")
	assert.Contains(t, remaining, "fresh", "an entry inside the window is kept")

	// A second sweep the same day does nothing, even with a new stale entry.
	insert(t, "stale-again", fixed.Add(-translationRetention-time.Hour))
	_, err = service.Translate(context.Background(), "zh", []contracts.TranslationItem{
		{ID: "ask", Text: "another turntable", SourceLang: "en"},
	})
	require.NoError(t, err)

	after, err := store.GetTranslations(context.Background(), []string{"stale-again"})
	require.NoError(t, err)
	assert.Contains(t, after, "stale-again", "the sweep runs once a day, not once a request")
}
