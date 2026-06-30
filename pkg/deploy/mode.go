package deploy

import "sync/atomic"

// Mode represents the deployment mode of the current process.
type Mode int32

const (
	// Standalone is the default: a single-tenant node running on its own.
	Standalone Mode = iota
	// SaaS indicates a multi-tenant hosting process managing many nodes.
	SaaS
	// PrivateDistribution indicates the private privacy-focused distribution.
	PrivateDistribution
)

var processMode atomic.Int32

// SetProcessMode sets the process-wide deployment mode.
// Must be called exactly once from the process entry point, before any
// goroutine reads it.
func SetProcessMode(m Mode) { processMode.Store(int32(m)) }

// GetProcessMode returns the current process-wide deployment mode.
func GetProcessMode() Mode { return Mode(processMode.Load()) }

// IsSaaS is a convenience for GetProcessMode() == SaaS.
func IsSaaS() bool { return GetProcessMode() == SaaS }

// IsPrivateDistribution is a convenience for GetProcessMode() == PrivateDistribution.
func IsPrivateDistribution() bool { return GetProcessMode() == PrivateDistribution }
