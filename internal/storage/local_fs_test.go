package storage

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"

	cid "github.com/ipfs/go-cid"
	"github.com/mobazha/mobazha/pkg/contracts"
)

func newTestAdapter(t *testing.T) *LocalFSAdapter {
	t.Helper()
	dir := t.TempDir()
	a, err := NewLocalFSAdapter(filepath.Join(dir, "blobs"))
	if err != nil {
		t.Fatalf("NewLocalFSAdapter: %v", err)
	}
	return a
}

func TestLocalFS_PutAndGet(t *testing.T) {
	a := newTestAdapter(t)
	ctx := context.Background()

	key := "bafkreihdwdcefgh4dqkjv67uzcmw7ojee6xedzdetojuzjevtenora"
	data := []byte("hello world")
	ct := "text/plain"

	if err := a.Put(ctx, key, data, ct); err != nil {
		t.Fatalf("Put: %v", err)
	}

	rc, gotCT, err := a.Get(ctx, key)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	defer rc.Close()

	got, err := io.ReadAll(rc)
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}
	if string(got) != string(data) {
		t.Errorf("data = %q; want %q", got, data)
	}
	if gotCT != ct {
		t.Errorf("contentType = %q; want %q", gotCT, ct)
	}
}

func TestLocalFS_Put_Idempotent(t *testing.T) {
	a := newTestAdapter(t)
	ctx := context.Background()

	key := "bafkreiexampleidempotentkey1234567890abcdef"
	data := []byte("payload")

	if err := a.Put(ctx, key, data, "image/png"); err != nil {
		t.Fatalf("Put 1: %v", err)
	}
	if err := a.Put(ctx, key, data, "image/png"); err != nil {
		t.Fatalf("Put 2 (idempotent): %v", err)
	}

	rc, _, err := a.Get(ctx, key)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	defer rc.Close()
	got, _ := io.ReadAll(rc)
	if string(got) != string(data) {
		t.Errorf("data = %q; want %q", got, data)
	}
}

func TestLocalFS_Get_NotFound(t *testing.T) {
	a := newTestAdapter(t)
	ctx := context.Background()

	_, _, err := a.Get(ctx, "bafkreinonexistentkey123456789")
	if err == nil {
		t.Fatal("expected error for missing key")
	}
	if !errors.Is(err, contracts.ErrBlobNotFound) {
		t.Errorf("error = %v; want wrapping ErrBlobNotFound", err)
	}
}

func TestLocalFS_Exists(t *testing.T) {
	a := newTestAdapter(t)
	ctx := context.Background()

	key := "bafkreiexistscheck1234567890abcdef"

	exists, err := a.Exists(ctx, key)
	if err != nil {
		t.Fatalf("Exists before Put: %v", err)
	}
	if exists {
		t.Error("Exists = true before Put")
	}

	if err := a.Put(ctx, key, []byte("data"), "application/octet-stream"); err != nil {
		t.Fatalf("Put: %v", err)
	}

	exists, err = a.Exists(ctx, key)
	if err != nil {
		t.Fatalf("Exists after Put: %v", err)
	}
	if !exists {
		t.Error("Exists = false after Put")
	}
}

func TestLocalFS_PublicURL_Empty(t *testing.T) {
	a := newTestAdapter(t)
	if url := a.PublicURL("anykey"); url != "" {
		t.Errorf("PublicURL = %q; want empty", url)
	}
}

func TestLocalFS_Put_DefaultContentType(t *testing.T) {
	a := newTestAdapter(t)
	ctx := context.Background()

	key := "bafkreidefaultcttest1234567890abcdef"
	if err := a.Put(ctx, key, []byte("data"), ""); err != nil {
		t.Fatalf("Put: %v", err)
	}

	_, ct, err := a.Get(ctx, key)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if ct != "application/octet-stream" {
		t.Errorf("contentType = %q; want application/octet-stream", ct)
	}
}

func TestLocalFS_DirectorySharding(t *testing.T) {
	a := newTestAdapter(t)
	ctx := context.Background()

	key := "bafkreishardingtest1234567890abcdef"
	if err := a.Put(ctx, key, []byte("x"), "text/plain"); err != nil {
		t.Fatalf("Put: %v", err)
	}

	expected := filepath.Join(a.rootDir, "ba", key)
	if _, err := os.Stat(expected); err != nil {
		t.Errorf("blob file not at expected sharded path %s: %v", expected, err)
	}

	metaExpected := expected + ".meta"
	if _, err := os.Stat(metaExpected); err != nil {
		t.Errorf("meta file not at expected path %s: %v", metaExpected, err)
	}
}

func TestCanonicalCID_V0toV1(t *testing.T) {
	// CIDv0 starts with "Qm", CIDv1 base32 starts with "b"
	cidStr := "QmYwAPJzv5CZsnN625s3Xf2nemtYgPpHdWEz79ojWnPbdG"
	c, err := cid.Decode(cidStr)
	if err != nil {
		t.Fatalf("Decode CIDv0: %v", err)
	}
	canonical := contracts.CanonicalCID(c)
	if canonical == cidStr {
		t.Error("CanonicalCID should convert v0 to v1 base32")
	}
	if len(canonical) == 0 || canonical[0] != 'b' {
		t.Errorf("canonical = %q; should start with 'b' (base32lower)", canonical)
	}

	c2, err := cid.Decode(canonical)
	if err != nil {
		t.Fatalf("Decode canonical CID: %v", err)
	}
	if !bytes.Equal(c.Hash(), c2.Hash()) {
		t.Error("round-trip multihash mismatch")
	}
}
