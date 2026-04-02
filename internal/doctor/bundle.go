package doctor

import (
	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

func ExportBundle(outPath string, cfg Config, summary Summary) error {
	if outPath == "" {
		outPath = fmt.Sprintf("mobazha-diag-%s.tar.gz", time.Now().Format("20060102-150405"))
	}

	f, err := os.Create(outPath)
	if err != nil {
		return fmt.Errorf("cannot create %s: %w", outPath, err)
	}
	defer f.Close()

	gz := gzip.NewWriter(f)
	defer gz.Close()
	tw := tar.NewWriter(gz)
	defer tw.Close()

	resultsJSON, _ := json.MarshalIndent(summary, "", "  ")
	if err := addToTar(tw, "doctor-results.json", resultsJSON); err != nil {
		return err
	}

	logFiles := []string{"mobazha.log", "caddy.log"}
	for _, name := range logFiles {
		path := filepath.Join(cfg.DataDir, "logs", name)
		if data, err := readTail(path, 100*1024); err == nil {
			_ = addToTar(tw, "logs/"+name, data)
		}
	}

	if out, err := exec.Command("docker", "logs", "--tail", "500", "mobazha-node").CombinedOutput(); err == nil {
		_ = addToTar(tw, "logs/docker-mobazha-node.log", out)
	}
	if out, err := exec.Command("docker", "logs", "--tail", "500", "mobazha-caddy").CombinedOutput(); err == nil {
		_ = addToTar(tw, "logs/docker-caddy.log", out)
	}

	envPath := filepath.Join(cfg.DataDir, "..", ".env")
	if data, err := os.ReadFile(envPath); err == nil {
		sanitized := SanitizeEnv(string(data))
		_ = addToTar(tw, "config/env-sanitized.txt", []byte(sanitized))
	}

	sysInfo := fmt.Sprintf("OS: %s\nArch: %s\nCPUs: %d\nTime: %s\n",
		runtime.GOOS, runtime.GOARCH, runtime.NumCPU(), time.Now().UTC().Format(time.RFC3339))
	_ = addToTar(tw, "system-info.txt", []byte(sysInfo))

	return nil
}

func SanitizeEnv(content string) string {
	var lines []string
	sensitiveKeys := map[string]bool{
		"STANDALONE_API_KEY": true,
		"ADMIN_PASSWORD":     true,
	}
	for _, line := range strings.Split(content, "\n") {
		parts := strings.SplitN(line, "=", 2)
		if len(parts) == 2 && sensitiveKeys[strings.TrimSpace(parts[0])] {
			lines = append(lines, parts[0]+"=<REDACTED>")
		} else {
			lines = append(lines, line)
		}
	}
	return strings.Join(lines, "\n")
}

func addToTar(tw *tar.Writer, name string, data []byte) error {
	hdr := &tar.Header{
		Name:    name,
		Size:    int64(len(data)),
		Mode:    0644,
		ModTime: time.Now(),
	}
	if err := tw.WriteHeader(hdr); err != nil {
		return err
	}
	_, err := tw.Write(data)
	return err
}

func readTail(path string, maxBytes int64) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	info, err := f.Stat()
	if err != nil {
		return nil, err
	}

	if info.Size() <= maxBytes {
		return io.ReadAll(f)
	}

	if _, err := f.Seek(-maxBytes, io.SeekEnd); err != nil {
		return nil, err
	}
	return io.ReadAll(f)
}
