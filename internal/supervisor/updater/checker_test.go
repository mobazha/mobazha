package updater

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"runtime"
	"testing"
)

func newTestChecker(url string) *Checker {
	return &Checker{
		logger:  log.New(os.Stderr, "[test] ", 0),
		client:  http.DefaultClient,
		baseURL: url,
	}
}

func assetName() string {
	name := fmt.Sprintf("mobazha-%s-%s", runtime.GOOS, runtime.GOARCH)
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	return name
}

func TestChecker_NewVersionAvailable(t *testing.T) {
	releases := []ghRelease{{
		TagName: "native-1.2.0",
		HTMLURL: "https://github.com/mobazha/mobazha.org/releases/tag/native-1.2.0",
		Body:    "Release notes for 1.2.0",
		Assets: []ghAsset{
			{Name: assetName(), BrowserDownloadURL: "https://example.com/binary"},
			{Name: "checksums-sha256.txt", BrowserDownloadURL: "https://example.com/checksums"},
		},
	}}

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("ETag", `"test-etag"`)
		json.NewEncoder(w).Encode(releases)
	}))
	defer ts.Close()

	c := newTestChecker(ts.URL)
	info, err := c.Check("1.0.0")
	if err != nil {
		t.Fatalf("Check() error: %v", err)
	}
	if info == nil {
		t.Fatal("expected non-nil ReleaseInfo")
	}
	if info.Version != "1.2.0" {
		t.Errorf("version = %q, want 1.2.0", info.Version)
	}
	if info.AssetURL != "https://example.com/binary" {
		t.Errorf("AssetURL = %q", info.AssetURL)
	}
	if info.ChecksumURL != "https://example.com/checksums" {
		t.Errorf("ChecksumURL = %q", info.ChecksumURL)
	}
	if c.etag != `"test-etag"` {
		t.Errorf("etag not cached: %q", c.etag)
	}
}

func TestChecker_AlreadyUpToDate(t *testing.T) {
	releases := []ghRelease{{
		TagName: "native-1.0.0",
		Assets:  []ghAsset{{Name: assetName(), BrowserDownloadURL: "https://example.com/bin"}},
	}}

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(releases)
	}))
	defer ts.Close()

	c := newTestChecker(ts.URL)
	info, err := c.Check("1.0.0")
	if err != nil {
		t.Fatalf("Check() error: %v", err)
	}
	if info != nil {
		t.Errorf("expected nil (up-to-date), got version %q", info.Version)
	}
}

func TestChecker_ETagNotModified(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("If-None-Match") == `"cached"` {
			w.WriteHeader(http.StatusNotModified)
			return
		}
		json.NewEncoder(w).Encode([]ghRelease{})
	}))
	defer ts.Close()

	c := newTestChecker(ts.URL)
	c.etag = `"cached"`
	info, err := c.Check("1.0.0")
	if err != nil {
		t.Fatalf("Check() error: %v", err)
	}
	if info != nil {
		t.Error("expected nil for 304 Not Modified")
	}
}

func TestChecker_NoMatchingAsset(t *testing.T) {
	releases := []ghRelease{{
		TagName: "native-2.0.0",
		Assets:  []ghAsset{{Name: "mobazha-freebsd-arm64", BrowserDownloadURL: "https://example.com/x"}},
	}}

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(releases)
	}))
	defer ts.Close()

	c := newTestChecker(ts.URL)
	_, err := c.Check("1.0.0")
	if err == nil {
		t.Error("expected error for missing platform asset")
	}
}

func TestChecker_APIError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer ts.Close()

	c := newTestChecker(ts.URL)
	_, err := c.Check("1.0.0")
	if err == nil {
		t.Error("expected error for 403 status")
	}
}

func TestChecker_SkipsNonNativeTags(t *testing.T) {
	releases := []ghRelease{
		{TagName: "docker-1.5.0", Assets: []ghAsset{{Name: assetName()}}},
		{TagName: "native-1.3.0", Assets: []ghAsset{{Name: assetName(), BrowserDownloadURL: "https://example.com/bin"}}},
	}

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(releases)
	}))
	defer ts.Close()

	c := newTestChecker(ts.URL)
	info, err := c.Check("1.0.0")
	if err != nil {
		t.Fatalf("Check() error: %v", err)
	}
	if info == nil || info.Version != "1.3.0" {
		t.Errorf("expected version 1.3.0 (skipping docker tag), got %v", info)
	}
}

func TestChecker_TruncatesLongNotes(t *testing.T) {
	longNotes := ""
	for i := 0; i < 300; i++ {
		longNotes += "x"
	}

	releases := []ghRelease{{
		TagName: "native-2.0.0",
		Body:    longNotes,
		Assets:  []ghAsset{{Name: assetName(), BrowserDownloadURL: "https://example.com/bin"}},
	}}

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(releases)
	}))
	defer ts.Close()

	c := newTestChecker(ts.URL)
	info, err := c.Check("1.0.0")
	if err != nil {
		t.Fatalf("Check() error: %v", err)
	}
	if len(info.Notes) > 204 { // 200 + "..."
		t.Errorf("notes too long: %d chars", len(info.Notes))
	}
}
