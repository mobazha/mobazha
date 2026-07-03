package core

import (
	"github.com/ipfs/go-cid"
	"github.com/mobazha/mobazha/pkg/contracts"
	"github.com/mobazha/mobazha/pkg/media"
)

// Compile-time check: cidContentStore implements contracts.ContentStore.
var _ contracts.ContentStore = (*cidContentStore)(nil)

// cidContentStore implements ContentStore using pure in-memory CID computation.
// No IPFS daemon dependency — uses go-unixfs dag-pb encoding for CIDv0.
//
// After Phase 2e, ContentStore only provides ComputeCID. IPFS pinning
// has been retired in favor of CDN (BlobStore/R2) persistence.
type cidContentStore struct{}

// ComputeCID returns the CIDv0 (dag-pb, SHA2-256) for the given data
// using pure in-memory UnixFS DAG construction.
func (cs *cidContentStore) ComputeCID(data []byte) (cid.Cid, error) {
	return media.ComputeUnixFSCID(data)
}
