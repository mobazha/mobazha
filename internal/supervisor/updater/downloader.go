package updater

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Downloader handles fetching and verifying new binaries.
type Downloader struct {
	dataDir string
	logger  *log.Logger
	client  *http.Client
}

func NewDownloader(dataDir string, logger *log.Logger) *Downloader {
	return &Downloader{
		dataDir: dataDir,
		logger:  logger,
		client:  &http.Client{Timeout: 10 * time.Minute},
	}
}

// Download fetches the binary asset and verifies SHA256.
// Returns the path to the verified temporary file.
func (d *Downloader) Download(info *ReleaseInfo, onProgress func(pct int)) (string, error) {
	updateDir := filepath.Join(d.dataDir, "updates")
	if err := os.MkdirAll(updateDir, 0755); err != nil {
		return "", fmt.Errorf("create updates dir: %w", err)
	}

	tmpPath := filepath.Join(updateDir, fmt.Sprintf("mobazha-%s.tmp", info.Version))

	d.logger.Printf("Downloading %s ...", info.AssetURL)
	if err := d.downloadFile(info.AssetURL, tmpPath, onProgress); err != nil {
		return "", fmt.Errorf("download binary: %w", err)
	}

	if info.ChecksumURL != "" {
		d.logger.Println("Verifying SHA256 checksum...")
		if err := d.verifyChecksum(tmpPath, info); err != nil {
			_ = os.Remove(tmpPath)
			return "", fmt.Errorf("checksum verification: %w", err)
		}
		d.logger.Println("Checksum verified")
	}

	return tmpPath, nil
}

func (d *Downloader) downloadFile(url, destPath string, onProgress func(pct int)) error {
	resp, err := d.client.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	out, err := os.Create(destPath)
	if err != nil {
		return err
	}
	defer out.Close()

	total := resp.ContentLength
	var written int64
	buf := make([]byte, 32*1024)

	for {
		n, readErr := resp.Body.Read(buf)
		if n > 0 {
			if _, writeErr := out.Write(buf[:n]); writeErr != nil {
				return writeErr
			}
			written += int64(n)
			if total > 0 && onProgress != nil {
				pct := int(written * 100 / total)
				onProgress(pct)
			}
		}
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			return readErr
		}
	}

	if onProgress != nil {
		onProgress(100)
	}
	return nil
}

func (d *Downloader) verifyChecksum(filePath string, info *ReleaseInfo) error {
	expected, err := d.fetchExpectedChecksum(info)
	if err != nil {
		return err
	}

	actual, err := fileSHA256(filePath)
	if err != nil {
		return err
	}

	if actual != expected {
		return fmt.Errorf("SHA256 mismatch: expected %s, got %s", expected, actual)
	}
	return nil
}

func (d *Downloader) fetchExpectedChecksum(info *ReleaseInfo) (string, error) {
	resp, err := d.client.Get(info.ChecksumURL)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("fetch checksums: HTTP %d", resp.StatusCode)
	}

	assetName := expectedAssetName()
	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		line := scanner.Text()
		parts := strings.Fields(line)
		if len(parts) == 2 && parts[1] == assetName {
			return parts[0], nil
		}
	}
	return "", fmt.Errorf("checksum for %s not found in checksums file", assetName)
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
