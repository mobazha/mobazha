package supervisor

import (
	"os"
	"path/filepath"
	"testing"
)

func TestTrigger_WriteThenPoll(t *testing.T) {
	dir := t.TempDir()
	tw := NewTriggerWatcher(dir)

	if err := WriteTrigger(dir, "check"); err != nil {
		t.Fatalf("WriteTrigger() error: %v", err)
	}

	if _, err := os.Stat(filepath.Join(dir, "update-trigger.json")); err != nil {
		t.Fatal("trigger file should exist after write")
	}

	action := tw.Poll()
	if action != "check" {
		t.Errorf("Poll() = %q, want check", action)
	}

	if _, err := os.Stat(filepath.Join(dir, "update-trigger.json")); !os.IsNotExist(err) {
		t.Error("trigger file should be deleted after Poll()")
	}
}

func TestTrigger_PollWhenEmpty(t *testing.T) {
	dir := t.TempDir()
	tw := NewTriggerWatcher(dir)

	action := tw.Poll()
	if action != "" {
		t.Errorf("Poll() on missing file = %q, want empty", action)
	}
}

func TestTrigger_ApplyAction(t *testing.T) {
	dir := t.TempDir()
	tw := NewTriggerWatcher(dir)

	WriteTrigger(dir, "apply")
	action := tw.Poll()
	if action != "apply" {
		t.Errorf("Poll() = %q, want apply", action)
	}
}

func TestTrigger_MultiplePollsConsumeOnce(t *testing.T) {
	dir := t.TempDir()
	tw := NewTriggerWatcher(dir)

	WriteTrigger(dir, "check")

	first := tw.Poll()
	second := tw.Poll()

	if first != "check" {
		t.Errorf("first Poll() = %q", first)
	}
	if second != "" {
		t.Errorf("second Poll() should be empty, got %q", second)
	}
}

func TestTrigger_InvalidJSON(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "update-trigger.json"), []byte("{bad"), 0644)

	tw := NewTriggerWatcher(dir)
	action := tw.Poll()
	if action != "" {
		t.Errorf("invalid JSON should return empty, got %q", action)
	}

	if _, err := os.Stat(filepath.Join(dir, "update-trigger.json")); !os.IsNotExist(err) {
		t.Error("invalid trigger file should still be deleted")
	}
}
