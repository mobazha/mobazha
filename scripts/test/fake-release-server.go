// fake-release-server simulates the GitHub Releases API for local auto-update testing.
//
// Usage:
//
//	go run scripts/test/fake-release-server.go -binary /path/to/new-mobazha -version 99.0.0 [-port 9999]
//
// Then start the launcher with:
//
//	MOBAZHA_UPDATE_URL=http://127.0.0.1:9999/releases mobazha-launcher
package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

type ghAsset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
}

type ghRelease struct {
	TagName string    `json:"tag_name"`
	HTMLURL string    `json:"html_url"`
	Body    string    `json:"body"`
	Assets  []ghAsset `json:"assets"`
}

func main() {
	binaryPath := flag.String("binary", "", "path to the 'new version' binary to serve (required)")
	version := flag.String("version", "99.0.0", "version string (without native- prefix)")
	port := flag.Int("port", 9999, "listen port")
	flag.Parse()

	if *binaryPath == "" {
		fmt.Fprintln(os.Stderr, "error: -binary is required")
		flag.Usage()
		os.Exit(1)
	}

	absPath, err := filepath.Abs(*binaryPath)
	if err != nil {
		log.Fatalf("resolve binary path: %v", err)
	}
	if _, err := os.Stat(absPath); err != nil {
		log.Fatalf("binary not found: %v", err)
	}

	checksum, err := fileSHA256(absPath)
	if err != nil {
		log.Fatalf("compute checksum: %v", err)
	}

	assetName := fmt.Sprintf("mobazha-%s-%s", runtime.GOOS, runtime.GOARCH)
	if runtime.GOOS == "windows" {
		assetName += ".exe"
	}

	baseURL := fmt.Sprintf("http://127.0.0.1:%d", *port)
	tag := "native-" + *version

	release := ghRelease{
		TagName: tag,
		HTMLURL: fmt.Sprintf("%s/release/%s", baseURL, tag),
		Body:    fmt.Sprintf("Test release %s for local auto-update verification.", *version),
		Assets: []ghAsset{
			{Name: assetName, BrowserDownloadURL: fmt.Sprintf("%s/download/%s", baseURL, assetName)},
			{Name: "checksums-sha256.txt", BrowserDownloadURL: fmt.Sprintf("%s/download/checksums-sha256.txt", baseURL)},
		},
	}

	checksumContent := fmt.Sprintf("%s  %s\n", checksum, assetName)

	mux := http.NewServeMux()

	mux.HandleFunc("/releases", func(w http.ResponseWriter, r *http.Request) {
		log.Printf("[API] %s %s", r.Method, r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]ghRelease{release})
	})

	mux.HandleFunc("/download/"+assetName, func(w http.ResponseWriter, r *http.Request) {
		log.Printf("[DL] %s %s", r.Method, r.URL.Path)
		f, err := os.Open(absPath)
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		defer f.Close()
		info, _ := f.Stat()
		w.Header().Set("Content-Length", fmt.Sprintf("%d", info.Size()))
		w.Header().Set("Content-Type", "application/octet-stream")
		io.Copy(w, f)
	})

	mux.HandleFunc("/download/checksums-sha256.txt", func(w http.ResponseWriter, r *http.Request) {
		log.Printf("[CS] %s %s", r.Method, r.URL.Path)
		w.Header().Set("Content-Type", "text/plain")
		fmt.Fprint(w, checksumContent)
	})

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		log.Printf("[???] %s %s (404)", r.Method, r.URL.Path)
		http.NotFound(w, r)
	})

	addr := fmt.Sprintf("127.0.0.1:%d", *port)
	log.Printf("Fake release server listening on %s", addr)
	log.Printf("  Tag: %s", tag)
	log.Printf("  Asset: %s", assetName)
	log.Printf("  SHA256: %s", checksum)
	log.Printf("  Binary: %s", absPath)
	log.Println()
	log.Printf("Set env: MOBAZHA_UPDATE_URL=%s/releases", baseURL)
	log.Println(strings.Repeat("-", 60))

	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatalf("server error: %v", err)
	}
}

func fileSHA256(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
