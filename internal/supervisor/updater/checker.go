package updater

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"runtime"
	"strings"
	"time"
)

const (
	releasesURL = "https://api.github.com/repos/mobazha/mobazha.org/releases"
	tagPrefix   = "native-"
)

type ghRelease struct {
	TagName    string    `json:"tag_name"`
	HTMLURL    string    `json:"html_url"`
	Body       string    `json:"body"`
	Assets     []ghAsset `json:"assets"`
}

type ghAsset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
}

// Checker queries GitHub Releases for newer versions.
type Checker struct {
	logger  *log.Logger
	client  *http.Client
	baseURL string
	etag    string
}

func NewChecker(logger *log.Logger) *Checker {
	base := releasesURL
	if override := os.Getenv("MOBAZHA_UPDATE_URL"); override != "" {
		base = override
		logger.Printf("Update URL overridden: %s", base)
	}
	return &Checker{
		logger:  logger,
		client:  &http.Client{Timeout: 30 * time.Second},
		baseURL: base,
	}
}

// Check compares current version against the latest GitHub release.
// Returns nil if already up-to-date.
func (c *Checker) Check(currentVersion string) (*ReleaseInfo, error) {
	req, err := http.NewRequest("GET", c.baseURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	if c.etag != "" {
		req.Header.Set("If-None-Match", c.etag)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch releases: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotModified {
		return nil, nil
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GitHub API returned %d", resp.StatusCode)
	}

	if etag := resp.Header.Get("ETag"); etag != "" {
		c.etag = etag
	}

	var releases []ghRelease
	if err := json.NewDecoder(resp.Body).Decode(&releases); err != nil {
		return nil, fmt.Errorf("decode releases: %w", err)
	}

	assetName := expectedAssetName()

	for _, r := range releases {
		if !strings.HasPrefix(r.TagName, tagPrefix) {
			continue
		}
		version := strings.TrimPrefix(r.TagName, tagPrefix)
		if version == "" || version == currentVersion {
			return nil, nil // latest release matches current
		}

		var assetURL, checksumURL string
		for _, a := range r.Assets {
			if a.Name == assetName {
				assetURL = a.BrowserDownloadURL
			}
			if a.Name == "checksums-sha256.txt" {
				checksumURL = a.BrowserDownloadURL
			}
		}
		if assetURL == "" {
			return nil, fmt.Errorf("asset %s not found in release %s", assetName, r.TagName)
		}

		notes := r.Body
		if len(notes) > 200 {
			notes = notes[:200] + "..."
		}

		return &ReleaseInfo{
			Version:     version,
			Tag:         r.TagName,
			ReleaseURL:  r.HTMLURL,
			Notes:       notes,
			AssetURL:    assetURL,
			ChecksumURL: checksumURL,
		}, nil
	}

	return nil, nil
}

func expectedAssetName() string {
	os := runtime.GOOS
	arch := runtime.GOARCH
	name := fmt.Sprintf("mobazha-%s-%s", os, arch)
	if os == "windows" {
		name += ".exe"
	}
	return name
}
