package supervisor

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

// StatusFileContent is the JSON structure of update-status.json,
// written by the Launcher and read by the Node.
type StatusFileContent struct {
	LauncherVersion    string    `json:"launcherVersion"`
	LauncherPID        int       `json:"launcherPID"`
	AutoUpdateEnabled  bool      `json:"autoUpdateEnabled"`
	CheckIntervalMin   int       `json:"checkIntervalMinutes"`
	UpdateChannel      string    `json:"updateChannel"`
	LastCheckTime      string    `json:"lastCheckTime,omitempty"`
	LatestVersion      string    `json:"latestVersion,omitempty"`
	LatestReleaseURL   string    `json:"latestReleaseURL,omitempty"`
	ReleaseNotes       string    `json:"releaseNotes,omitempty"`
	UpdateStatus       string    `json:"updateStatus"`
	DownloadProgress   int       `json:"downloadProgress"`
	LastError          string    `json:"lastError,omitempty"`
	UpdatedAt          time.Time `json:"updatedAt"`
}

// StatusWriter manages writing update-status.json.
type StatusWriter struct {
	filePath string
}

func NewStatusWriter(dataDir string) *StatusWriter {
	return &StatusWriter{
		filePath: filepath.Join(dataDir, "update-status.json"),
	}
}

func (sw *StatusWriter) WriteStatus(content StatusFileContent) error {
	content.UpdatedAt = time.Now()
	data, err := json.MarshalIndent(content, "", "  ")
	if err != nil {
		return err
	}
	dir := filepath.Dir(sw.filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	return os.WriteFile(sw.filePath, data, 0644)
}

func (sw *StatusWriter) FilePath() string {
	return sw.filePath
}

// ReadStatusFile reads and parses update-status.json from disk.
// This is used by the Node side to serve update info to the frontend.
func ReadStatusFile(dataDir string) (*StatusFileContent, error) {
	path := filepath.Join(dataDir, "update-status.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var content StatusFileContent
	if err := json.Unmarshal(data, &content); err != nil {
		return nil, err
	}
	return &content, nil
}
