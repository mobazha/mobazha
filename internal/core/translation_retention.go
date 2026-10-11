package core

import (
	"context"
	"sync/atomic"
	"time"
)

// translationRetention is how long a cached translation is kept after it was
// written. It is generous on purpose: the cache is what keeps a storefront's
// wording from being translated (and paid for) twice, and an entry that is
// dropped too eagerly just comes back.
const translationRetention = 180 * 24 * time.Hour

// lastPruneDay is the day the cache was last tidied, so the sweep runs once a
// day rather than on every translation. It is process state, not stored state:
// a restarted node tidying up again costs one cheap delete.
var lastPruneDay atomic.Value

// pruneTranslations drops cached translations that have been sitting for longer
// than the retention window.
//
// It runs from the translation path rather than from a scheduler, and only on
// the first translation of each day: a node that is never asked to translate
// anything never touches the table, and one that is busy does a single delete a
// day. Failures are logged and forgotten — housekeeping must never be the reason
// a page shows the wrong language.
func (s *TranslationAppService) pruneTranslations(ctx context.Context) {
	now := s.now().UTC()
	day := now.Format("2006-01-02")
	if previous, _ := lastPruneDay.Load().(string); previous == day {
		return
	}
	lastPruneDay.Store(day)

	if _, err := s.store.DeleteTranslationsOlderThan(ctx, now.Add(-translationRetention)); err != nil {
		log.Warningf("Translation: cannot prune the cache: %s", err)
	}
}
