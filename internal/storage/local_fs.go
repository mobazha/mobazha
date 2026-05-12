// Package storage provides BlobStore implementations for local and remote
// binary object storage.
package storage

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/mobazha/mobazha3.0/pkg/contracts"
)

// LocalFSAdapter implements contracts.BlobStore backed by the local filesystem.
// Objects are stored under {rootDir}/{key[0:2]}/{key} with a sidecar .meta
// file holding the Content-Type.
//
// Designed for standalone node deployments where no external CDN is available.
type LocalFSAdapter struct {
	rootDir string
}

var _ contracts.BlobStore = (*LocalFSAdapter)(nil)

// NewLocalFSAdapter creates a LocalFSAdapter rooted at dir.
// The directory is created (with parents) if it does not exist.
func NewLocalFSAdapter(dir string) (*LocalFSAdapter, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("create blob root %s: %w", dir, err)
	}
	return &LocalFSAdapter{rootDir: dir}, nil
}

func (a *LocalFSAdapter) blobPath(key string) string {
	prefix := key
	if len(prefix) > 2 {
		prefix = prefix[:2]
	}
	return filepath.Join(a.rootDir, prefix, key)
}

func (a *LocalFSAdapter) metaPath(key string) string {
	return a.blobPath(key) + ".meta"
}

// Put stores data under key. Idempotent — existing keys are skipped.
func (a *LocalFSAdapter) Put(_ context.Context, key string, data []byte, contentType string) error {
	bpath := a.blobPath(key)

	if _, err := os.Stat(bpath); err == nil {
		return nil
	}

	if err := os.MkdirAll(filepath.Dir(bpath), 0o755); err != nil {
		return fmt.Errorf("mkdir for blob %s: %w", key, err)
	}

	tmp := bpath + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return fmt.Errorf("write blob %s: %w", key, err)
	}
	if err := os.Rename(tmp, bpath); err != nil {
		os.Remove(tmp)
		return fmt.Errorf("rename blob %s: %w", key, err)
	}

	return a.writeMeta(key, contentType)
}

// PutStream streams data from r to {rootDir}/{key[0:2]}/{key} via a temp
// file and atomic rename. Idempotent — existing keys are skipped.
//
// Memory footprint is bounded by io.Copy's internal 32 KiB buffer regardless
// of file size, supporting multi-hundred-MiB digital asset uploads.
func (a *LocalFSAdapter) PutStream(_ context.Context, key string, r io.Reader, _ int64, contentType string) error {
	bpath := a.blobPath(key)
	if _, err := os.Stat(bpath); err == nil {
		// Drain reader so the upstream encryptor / pipe can finish cleanly,
		// otherwise the goroutine writing to the pipe will block forever.
		_, _ = io.Copy(io.Discard, r)
		return nil
	}

	if err := os.MkdirAll(filepath.Dir(bpath), 0o755); err != nil {
		return fmt.Errorf("mkdir for blob %s: %w", key, err)
	}

	tmp := bpath + ".tmp"
	f, err := os.OpenFile(tmp, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return fmt.Errorf("create tmp blob %s: %w", key, err)
	}
	if _, err := io.Copy(f, r); err != nil {
		_ = f.Close()
		_ = os.Remove(tmp)
		return fmt.Errorf("write blob stream %s: %w", key, err)
	}
	if err := f.Sync(); err != nil {
		_ = f.Close()
		_ = os.Remove(tmp)
		return fmt.Errorf("sync blob %s: %w", key, err)
	}
	if err := f.Close(); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("close blob %s: %w", key, err)
	}
	if err := os.Rename(tmp, bpath); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("rename blob %s: %w", key, err)
	}

	return a.writeMeta(key, contentType)
}

func (a *LocalFSAdapter) writeMeta(key, contentType string) error {
	ct := strings.TrimSpace(contentType)
	if ct == "" {
		ct = "application/octet-stream"
	}
	if err := os.WriteFile(a.metaPath(key), []byte(ct), 0o644); err != nil {
		return fmt.Errorf("write meta %s: %w", key, err)
	}
	return nil
}

// Get retrieves the blob and its Content-Type.
// Returns contracts.ErrBlobNotFound when the key is absent.
func (a *LocalFSAdapter) Get(_ context.Context, key string) (io.ReadCloser, string, error) {
	data, err := os.ReadFile(a.blobPath(key))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, "", fmt.Errorf("%w: %s", contracts.ErrBlobNotFound, key)
		}
		return nil, "", fmt.Errorf("read blob %s: %w", key, err)
	}

	ct := "application/octet-stream"
	if raw, err := os.ReadFile(a.metaPath(key)); err == nil {
		if s := strings.TrimSpace(string(raw)); s != "" {
			ct = s
		}
	}

	return io.NopCloser(bytes.NewReader(data)), ct, nil
}

// Exists checks whether a key is stored locally.
func (a *LocalFSAdapter) Exists(_ context.Context, key string) (bool, error) {
	_, err := os.Stat(a.blobPath(key))
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}

// PublicURL always returns "" because there is no external CDN for local storage.
func (a *LocalFSAdapter) PublicURL(_ string) string { return "" }
