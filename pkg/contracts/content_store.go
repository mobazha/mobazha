// Package contracts — ContentStore abstracts content-addressed storage.
//
// After IPFS retirement Phases 2a-2e, only CID computation remains.
// Content retrieval: NetDB / Search API.
// Content persistence: BlobStore / R2 CDN.
// Content pinning: removed (CDN replaces IPFS-level replication).
package contracts

import (
	"github.com/ipfs/go-cid"
)

// ContentStore computes content-addressed identifiers.
//
// The interface has been progressively simplified through IPFS retirement:
//   - Phase 2a: ComputeCID switched to pure in-memory go-unixfs
//   - Phase 2b: AddFile/Unpin removed
//   - Phase 2c: Cat removed
//   - Phase 2e: Pin removed (CDN replaces IPFS replication)
type ContentStore interface {
	// ComputeCID returns the CIDv0 (dag-pb, SHA2-256) for the given data
	// using pure in-memory UnixFS DAG construction. No IPFS daemon needed.
	ComputeCID(data []byte) (cid.Cid, error)
}
