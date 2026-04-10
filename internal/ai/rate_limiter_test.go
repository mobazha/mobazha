package ai

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDailyRateLimiter_Allow(t *testing.T) {
	rl := NewDailyRateLimiter()

	ok, count := rl.Allow("tenant1", 3)
	assert.True(t, ok)
	assert.Equal(t, 0, count)

	rl.Increment("tenant1")
	rl.Increment("tenant1")
	ok, count = rl.Allow("tenant1", 3)
	assert.True(t, ok)
	assert.Equal(t, 2, count)

	rl.Increment("tenant1")
	ok, count = rl.Allow("tenant1", 3)
	assert.False(t, ok)
	assert.Equal(t, 3, count)
}

func TestDailyRateLimiter_UnlimitedWhenZero(t *testing.T) {
	rl := NewDailyRateLimiter()

	for i := 0; i < 100; i++ {
		rl.Increment("tenant1")
	}

	ok, _ := rl.Allow("tenant1", 0)
	assert.True(t, ok, "limit=0 should always allow")
}

func TestDailyRateLimiter_TenantIsolation(t *testing.T) {
	rl := NewDailyRateLimiter()

	rl.Increment("tenant1")
	rl.Increment("tenant1")
	rl.Increment("tenant1")

	ok, count := rl.Allow("tenant2", 3)
	assert.True(t, ok)
	assert.Equal(t, 0, count)
}

func TestDailyRateLimiter_Usage(t *testing.T) {
	rl := NewDailyRateLimiter()

	assert.Equal(t, 0, rl.Usage("tenant1"))

	rl.Increment("tenant1")
	rl.Increment("tenant1")
	assert.Equal(t, 2, rl.Usage("tenant1"))

	assert.Equal(t, 0, rl.Usage("tenant-unknown"))
}
