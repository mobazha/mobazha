// Package contracts — ContentStore abstracts content-addressed storage.
//
// Standalone mode: backed by a real IPFS node (ipfsContentStore).
// SaaS mode: backed by a shared IPFS node or HTTP gateway.
//
// Phase 2c: Cat removed — all callers migrated to NetDB / Search API.
// Phase 2b: AddFile and Unpin removed — no callers remain after Phase 2a
// replaced ComputeCID with pure in-memory CID computation.
package contracts

import (
	"context"

	"github.com/ipfs/go-cid"
)

// ContentStore abstracts content-addressed storage operations.
//
// After IPFS retirement Phases 2a-2c, only CID computation and
// pinning remain. Content retrieval is handled by NetDB / Search API.
type ContentStore interface {
	// ComputeCID returns the content ID for the given data without
	// permanently storing it. Uses pure in-memory UnixFS DAG construction.
	ComputeCID(data []byte) (cid.Cid, error)

	// Pin ensures that the content identified by c is kept in the store
	// and will not be garbage-collected.
	Pin(ctx context.Context, c cid.Cid) error
}
