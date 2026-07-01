package core

import (
	"testing"

	"github.com/mobazha/mobazha3.0/internal/repo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestRatingsAppService(t *testing.T) *RatingsAppService {
	t.Helper()
	db, err := repo.MockDB()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	return NewRatingsAppService(RatingsAppServiceConfig{DB: db})
}

// ── GetMyRatings ────────────────────────────────────────────────

func TestRatingsAppService_GetMyRatings_Empty(t *testing.T) {
	svc := newTestRatingsAppService(t)
	_, err := svc.GetMyRatings()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}
