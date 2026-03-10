package media

import (
	"crypto/rand"
	"testing"

	"github.com/ipfs/go-cid"
	mh "github.com/multiformats/go-multihash"
)

func TestComputeUnixFSCID_EmptyData(t *testing.T) {
	c, err := ComputeUnixFSCID([]byte{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	assertCIDv0Properties(t, c)

	if c.String() != goldenEmpty {
		t.Errorf("empty data CID mismatch: got %s, want %s", c.String(), goldenEmpty)
	}
}

func TestComputeUnixFSCID_SmallFile(t *testing.T) {
	data := []byte("hello world")
	c, err := ComputeUnixFSCID(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	assertCIDv0Properties(t, c)

	if c.String() != goldenHello {
		t.Errorf("small file CID mismatch: got %s, want %s", c.String(), goldenHello)
	}
}

func TestComputeUnixFSCID_Deterministic(t *testing.T) {
	data := make([]byte, 1024)
	rand.Read(data)

	c1, err := ComputeUnixFSCID(data)
	if err != nil {
		t.Fatalf("first call: %v", err)
	}

	c2, err := ComputeUnixFSCID(data)
	if err != nil {
		t.Fatalf("second call: %v", err)
	}

	if !c1.Equals(c2) {
		t.Errorf("non-deterministic: %s != %s", c1, c2)
	}
}

func TestComputeUnixFSCID_DataDependent(t *testing.T) {
	c1, _ := ComputeUnixFSCID([]byte("aaa"))
	c2, _ := ComputeUnixFSCID([]byte("bbb"))

	if c1.Equals(c2) {
		t.Error("different data produced same CID")
	}
}

func TestComputeUnixFSCID_CrossChunkBoundary(t *testing.T) {
	const chunkSize = 256 * 1024
	data := make([]byte, chunkSize+1)
	rand.Read(data)

	c, err := ComputeUnixFSCID(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	assertCIDv0Properties(t, c)

	cSingle, _ := ComputeUnixFSCID(data[:chunkSize])
	if c.Equals(cSingle) {
		t.Error("cross-chunk CID should differ from single-chunk CID")
	}
}

func TestComputeUnixFSCID_LargeFile(t *testing.T) {
	data := make([]byte, 1024*1024) // 1 MiB
	rand.Read(data)

	c, err := ComputeUnixFSCID(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	assertCIDv0Properties(t, c)
}

func TestComputeUnixFSCID_NotRawSHA256(t *testing.T) {
	data := []byte("test")
	c, _ := ComputeUnixFSCID(data)

	h, _ := mh.Sum(data, mh.SHA2_256, -1)
	rawCID := cid.NewCidV0(h)

	if c.Equals(rawCID) {
		t.Error("CID should NOT equal raw SHA2-256 hash — UnixFS wraps data in protobuf")
	}
}

func BenchmarkComputeUnixFSCID_1KB(b *testing.B) {
	data := make([]byte, 1024)
	rand.Read(data)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ComputeUnixFSCID(data)
	}
}

func BenchmarkComputeUnixFSCID_256KB(b *testing.B) {
	data := make([]byte, 256*1024)
	rand.Read(data)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ComputeUnixFSCID(data)
	}
}

func BenchmarkComputeUnixFSCID_1MB(b *testing.B) {
	data := make([]byte, 1024*1024)
	rand.Read(data)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ComputeUnixFSCID(data)
	}
}

func BenchmarkComputeUnixFSCID_5MB(b *testing.B) {
	data := make([]byte, 5*1024*1024)
	rand.Read(data)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ComputeUnixFSCID(data)
	}
}

// Golden CID values verified against IPFS daemon output (`ipfs add`).
// These serve as the integration comparison test ("准入门"): if our pure
// implementation produces different CIDs, the parameters are not aligned.
//
// Reference: https://flyingzumwalt.gitbooks.io/decentralized-web-primer/content/files-on-ipfs/
//   - empty file:   `echo -n "" | ipfs add -q` → QmbFMke1KXqnYyBBWxB74N4c5SBnJMVAiMNRcGu6x1AwQH
//   - "hello world": `echo -n "hello world" | ipfs add -q` → Qmf412jQZiuVUtdgnB36FXFX7xg5V6KEbSJ4dpQuhkLyfD
const (
	goldenEmpty = "QmbFMke1KXqnYyBBWxB74N4c5SBnJMVAiMNRcGu6x1AwQH"
	goldenHello = "Qmf412jQZiuVUtdgnB36FXFX7xg5V6KEbSJ4dpQuhkLyfD"
)

func assertCIDv0Properties(t *testing.T, c cid.Cid) {
	t.Helper()

	if c.Version() != 0 {
		t.Errorf("expected CIDv0, got v%d", c.Version())
	}
	if c.Type() != cid.DagProtobuf {
		t.Errorf("expected dag-pb codec (%d), got %d", cid.DagProtobuf, c.Type())
	}
	decoded, err := mh.Decode(c.Hash())
	if err != nil {
		t.Fatalf("multihash decode: %v", err)
	}
	if decoded.Code != mh.SHA2_256 {
		t.Errorf("expected SHA2-256, got %d", decoded.Code)
	}
	if decoded.Length != 32 {
		t.Errorf("expected 32-byte digest, got %d", decoded.Length)
	}
	s := c.String()
	if len(s) < 2 || s[:2] != "Qm" {
		t.Errorf("CIDv0 should start with Qm, got %q", s[:10])
	}
}
