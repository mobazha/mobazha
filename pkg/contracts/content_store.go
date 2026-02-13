// Package contracts — ContentStore abstracts content-addressed storage (IPFS).
//
// Standalone mode: backed by a real IPFS node (ipfsContentStore).
// SaaS mode: backed by a shared IPFS node or HTTP gateway.
package contracts

import (
	"context"

	"github.com/ipfs/go-cid"
)

// ContentStore abstracts content-addressed storage operations.
//
// All path arguments are IPFS path strings (e.g. "/ipfs/Qm.../file.json").
// Implementations convert to their native path representation internally.
type ContentStore interface {
	// Cat fetches content from the store by its path string.
	Cat(ctx context.Context, contentPath string) ([]byte, error)

	// AddFile imports a local file into the store and returns its CID.
	AddFile(ctx context.Context, filePath string) (cid.Cid, error)

	// ComputeCID returns the content ID for the given data without
	// permanently storing it.
	ComputeCID(data []byte) (cid.Cid, error)

	// Pin ensures that the content identified by c is kept in the store
	// and will not be garbage-collected.
	Pin(ctx context.Context, c cid.Cid) error

	// Unpin allows the content identified by c to be garbage-collected.
	Unpin(ctx context.Context, c cid.Cid) error
}
