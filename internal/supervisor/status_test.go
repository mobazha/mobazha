package supervisor

import (
	"testing"
	"time"
)

func TestStatusWriter_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	sw := NewStatusWriter(dir)

	content := StatusFileContent{
		LauncherVersion:   "1.2.3",
		LauncherPID:       12345,
		AutoUpdateEnabled: true,
		CheckIntervalMin:  360,
		UpdateChannel:     "stable",
		LatestVersion:     "1.3.0",
		LatestReleaseURL:  "https://example.com/release",
		ReleaseNotes:      "Bug fixes",
		UpdateStatus:      "available",
		DownloadProgress:  42,
		LastError:         "",
	}

	if err := sw.WriteStatus(content); err != nil {
		t.Fatalf("WriteStatus() error: %v", err)
	}

	got, err := ReadStatusFile(dir)
	if err != nil {
		t.Fatalf("ReadStatusFile() error: %v", err)
	}

	if got.LauncherVersion != "1.2.3" {
		t.Errorf("LauncherVersion = %q", got.LauncherVersion)
	}
	if got.LauncherPID != 12345 {
		t.Errorf("LauncherPID = %d", got.LauncherPID)
	}
	if !got.AutoUpdateEnabled {
		t.Error("AutoUpdateEnabled should be true")
	}
	if got.CheckIntervalMin != 360 {
		t.Errorf("CheckIntervalMin = %d", got.CheckIntervalMin)
	}
	if got.LatestVersion != "1.3.0" {
		t.Errorf("LatestVersion = %q", got.LatestVersion)
	}
	if got.UpdateStatus != "available" {
		t.Errorf("UpdateStatus = %q", got.UpdateStatus)
	}
	if got.DownloadProgress != 42 {
		t.Errorf("DownloadProgress = %d", got.DownloadProgress)
	}
	if got.UpdatedAt.IsZero() {
		t.Error("UpdatedAt should be set automatically")
	}
	if time.Since(got.UpdatedAt) > 5*time.Second {
		t.Error("UpdatedAt should be recent")
	}
}

func TestStatusWriter_OverwritePreservesAllFields(t *testing.T) {
	dir := t.TempDir()
	sw := NewStatusWriter(dir)

	sw.WriteStatus(StatusFileContent{
		LauncherVersion: "1.0.0",
		LatestVersion:   "1.1.0",
		UpdateStatus:    "available",
		DownloadProgress: 50,
	})

	sw.WriteStatus(StatusFileContent{
		LauncherVersion:   "1.0.0",
		LatestVersion:     "1.2.0",
		UpdateStatus:      "downloading",
		DownloadProgress:  75,
		AutoUpdateEnabled: true,
	})

	got, _ := ReadStatusFile(dir)
	if got.LatestVersion != "1.2.0" {
		t.Errorf("second write should overwrite: LatestVersion = %q", got.LatestVersion)
	}
	if got.DownloadProgress != 75 {
		t.Errorf("DownloadProgress = %d, want 75", got.DownloadProgress)
	}
}

func TestReadStatusFile_NotFound(t *testing.T) {
	dir := t.TempDir()
	_, err := ReadStatusFile(dir)
	if err == nil {
		t.Error("expected error when status file doesn't exist")
	}
}
