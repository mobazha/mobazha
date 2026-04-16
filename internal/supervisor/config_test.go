package supervisor

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestConfigManager_DefaultValues(t *testing.T) {
	dir := t.TempDir()
	cm := NewConfigManager(dir)
	cfg := cm.Get()

	if !cfg.AutoUpdateEnabled {
		t.Error("default AutoUpdateEnabled should be true")
	}
	if cfg.CheckIntervalMin != 360 {
		t.Errorf("default CheckIntervalMin = %d, want 360", cfg.CheckIntervalMin)
	}
	if cfg.UpdateChannel != "stable" {
		t.Errorf("default UpdateChannel = %q, want stable", cfg.UpdateChannel)
	}
}

func TestConfigManager_LoadFromDisk(t *testing.T) {
	dir := t.TempDir()
	cfg := LauncherConfig{
		AutoUpdateEnabled: false,
		CheckIntervalMin:  60,
		UpdateChannel:     "beta",
	}
	data, _ := json.MarshalIndent(cfg, "", "  ")
	os.WriteFile(filepath.Join(dir, "launcher-config.json"), data, 0644)

	cm := NewConfigManager(dir)
	got := cm.Get()

	if got.AutoUpdateEnabled {
		t.Error("AutoUpdateEnabled should be false from disk")
	}
	if got.CheckIntervalMin != 60 {
		t.Errorf("CheckIntervalMin = %d, want 60", got.CheckIntervalMin)
	}
	if got.UpdateChannel != "beta" {
		t.Errorf("UpdateChannel = %q, want beta", got.UpdateChannel)
	}
}

func TestConfigManager_ReloadUpdates(t *testing.T) {
	dir := t.TempDir()
	cm := NewConfigManager(dir)

	if cm.Get().UpdateChannel != "stable" {
		t.Fatal("precondition: default channel = stable")
	}

	cfg := LauncherConfig{
		AutoUpdateEnabled: true,
		CheckIntervalMin:  120,
		UpdateChannel:     "nightly",
	}
	data, _ := json.MarshalIndent(cfg, "", "  ")
	os.WriteFile(filepath.Join(dir, "launcher-config.json"), data, 0644)

	cm.Reload()
	got := cm.Get()
	if got.UpdateChannel != "nightly" {
		t.Errorf("after Reload, UpdateChannel = %q, want nightly", got.UpdateChannel)
	}
}

func TestConfigManager_InvalidJSON(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "launcher-config.json"), []byte("{invalid"), 0644)

	cm := NewConfigManager(dir)
	cfg := cm.Get()

	if cfg.UpdateChannel != "stable" {
		t.Error("invalid JSON should fallback to defaults")
	}
}

func TestConfigManager_ZeroIntervalDefaultsTo360(t *testing.T) {
	dir := t.TempDir()
	cfg := LauncherConfig{
		AutoUpdateEnabled: true,
		CheckIntervalMin:  0,
		UpdateChannel:     "stable",
	}
	data, _ := json.MarshalIndent(cfg, "", "  ")
	os.WriteFile(filepath.Join(dir, "launcher-config.json"), data, 0644)

	cm := NewConfigManager(dir)
	if cm.Get().CheckIntervalMin != 360 {
		t.Errorf("zero interval should default to 360, got %d", cm.Get().CheckIntervalMin)
	}
}
