package contracts

import (
	"context"
	"errors"
	"io"

	cid "github.com/ipfs/go-cid"
	"github.com/multiformats/go-multibase"
)

// ErrBlobNotFound is returned by BlobStore.Get when the requested key does not exist.
var ErrBlobNotFound = errors.New("blob not found")

// BlobStore abstracts binary object storage keyed by content address (CID).
//
// Implementations:
//   - R2Adapter (Cloudflare R2 via S3 API) — SaaS mode
//   - LocalFSAdapter (local filesystem) — Standalone mode
//
// All keys MUST be canonical CID strings (CIDv1 base32) produced by CanonicalCID.
type BlobStore interface {
	// Put stores data under the given key. Idempotent — re-upload of an
	// existing key is a no-op.
	Put(ctx context.Context, key string, data []byte, contentType string) error

	// Get retrieves data by key.
	// Returns ErrBlobNotFound (possibly wrapped) when the key is absent.
	Get(ctx context.Context, key string) (io.ReadCloser, string, error)

	// Exists checks whether a key is present without downloading the object.
	Exists(ctx context.Context, key string) (bool, error)

	// PublicURL returns the CDN URL for the given key, or "" if the adapter
	// has no public CDN (e.g. LocalFSAdapter).
	PublicURL(key string) string
}

// CanonicalCID converts any CID to its canonical CIDv1 base32 string,
// eliminating encoding differences (v0 vs v1) that would otherwise cause
// duplicate objects in BlobStore.
func CanonicalCID(c cid.Cid) string {
	if c.Version() == 0 {
		c = cid.NewCidV1(c.Type(), c.Hash())
	}
	s, _ := c.StringOfBase(multibase.Base32)
	return s
}
