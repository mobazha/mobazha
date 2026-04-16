package updater

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
)

func TestDownloader_DownloadAndVerify(t *testing.T) {
	binaryContent := []byte("fake-binary-content-v2")
	checksum := sha256.Sum256(binaryContent)
	checksumHex := hex.EncodeToString(checksum[:])
	checksumFile := fmt.Sprintf("%s  %s\n", checksumHex, expectedAssetName())

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/binary":
			w.Header().Set("Content-Length", fmt.Sprintf("%d", len(binaryContent)))
			w.Write(binaryContent)
		case "/checksums":
			w.Write([]byte(checksumFile))
		default:
			w.WriteHeader(404)
		}
	}))
	defer ts.Close()

	dataDir := t.TempDir()
	d := &Downloader{
		dataDir: dataDir,
		logger:  log.New(os.Stderr, "[test] ", 0),
		client:  ts.Client(),
	}

	var lastPct atomic.Int32
	info := &ReleaseInfo{
		Version:     "2.0.0",
		AssetURL:    ts.URL + "/binary",
		ChecksumURL: ts.URL + "/checksums",
	}

	path, err := d.Download(info, func(pct int) {
		lastPct.Store(int32(pct))
	})
	if err != nil {
		t.Fatalf("Download() error: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read downloaded file: %v", err)
	}
	if string(data) != string(binaryContent) {
		t.Error("downloaded content mismatch")
	}
	if lastPct.Load() != 100 {
		t.Errorf("expected progress 100%%, got %d%%", lastPct.Load())
	}
}

func TestDownloader_ChecksumMismatch(t *testing.T) {
	binaryContent := []byte("real-binary")
	wrongChecksum := "0000000000000000000000000000000000000000000000000000000000000000"
	checksumFile := fmt.Sprintf("%s  %s\n", wrongChecksum, expectedAssetName())

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/binary":
			w.Write(binaryContent)
		case "/checksums":
			w.Write([]byte(checksumFile))
		}
	}))
	defer ts.Close()

	dataDir := t.TempDir()
	d := &Downloader{
		dataDir: dataDir,
		logger:  log.New(os.Stderr, "[test] ", 0),
		client:  ts.Client(),
	}

	info := &ReleaseInfo{
		Version:     "2.0.0",
		AssetURL:    ts.URL + "/binary",
		ChecksumURL: ts.URL + "/checksums",
	}
	_, err := d.Download(info, nil)
	if err == nil {
		t.Fatal("expected checksum mismatch error")
	}

	tmpPath := filepath.Join(dataDir, "updates", "mobazha-2.0.0.tmp")
	if _, statErr := os.Stat(tmpPath); !os.IsNotExist(statErr) {
		t.Error("temp file should be removed on checksum failure")
	}
}

func TestDownloader_NoChecksumURL(t *testing.T) {
	binaryContent := []byte("binary-without-checksum")

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write(binaryContent)
	}))
	defer ts.Close()

	dataDir := t.TempDir()
	d := &Downloader{
		dataDir: dataDir,
		logger:  log.New(os.Stderr, "[test] ", 0),
		client:  ts.Client(),
	}

	info := &ReleaseInfo{
		Version:  "3.0.0",
		AssetURL: ts.URL + "/binary",
	}

	path, err := d.Download(info, nil)
	if err != nil {
		t.Fatalf("Download() error: %v", err)
	}

	data, _ := os.ReadFile(path)
	if string(data) != string(binaryContent) {
		t.Error("content mismatch")
	}
}

func TestDownloader_ServerError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer ts.Close()

	dataDir := t.TempDir()
	d := &Downloader{
		dataDir: dataDir,
		logger:  log.New(os.Stderr, "[test] ", 0),
		client:  ts.Client(),
	}

	info := &ReleaseInfo{Version: "1.0.0", AssetURL: ts.URL + "/binary"}
	_, err := d.Download(info, nil)
	if err == nil {
		t.Fatal("expected error on server error")
	}
}

func TestFileSHA256(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "testfile")
	content := []byte("hello world")
	os.WriteFile(path, content, 0644)

	got, err := fileSHA256(path)
	if err != nil {
		t.Fatal(err)
	}

	sum := sha256.Sum256(content)
	want := hex.EncodeToString(sum[:])
	if got != want {
		t.Errorf("fileSHA256 = %q, want %q", got, want)
	}
}
