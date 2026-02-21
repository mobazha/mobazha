package webhook

import "time"

// Config holds webhook system tuning and quota parameters.
// Zero values for limit fields mean unlimited.
type Config struct {
	MaxEndpoints  int           // max active endpoints per node (0 = unlimited)
	MaxRatePerMin int           // max deliveries per minute (0 = unlimited, reserved for future use)
	RetentionDays int           // delivery record retention in days
	MaxRetries    int           // max delivery attempts (including the first)
	PollInterval  time.Duration // delivery polling interval
	HTTPTimeout   time.Duration // per-delivery HTTP timeout
}

// DefaultConfig returns safe defaults for standalone nodes (no limits).
func DefaultConfig() Config {
	return Config{
		MaxEndpoints:  0,
		MaxRatePerMin: 0,
		RetentionDays: 7,
		MaxRetries:    5,
		PollInterval:  5 * time.Second,
		HTTPTimeout:   10 * time.Second,
	}
}

func (c Config) retentionAge() time.Duration {
	if c.RetentionDays <= 0 {
		return 7 * 24 * time.Hour
	}
	return time.Duration(c.RetentionDays) * 24 * time.Hour
}

func (c Config) maxRetries() int {
	if c.MaxRetries <= 0 {
		return 5
	}
	return c.MaxRetries
}

func (c Config) pollInterval() time.Duration {
	if c.PollInterval <= 0 {
		return 5 * time.Second
	}
	return c.PollInterval
}

func (c Config) httpTimeout() time.Duration {
	if c.HTTPTimeout <= 0 {
		return 10 * time.Second
	}
	return c.HTTPTimeout
}
