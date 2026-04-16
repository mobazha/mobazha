package supervisor

import (
	"encoding/json"
	"os"
	"path/filepath"
)

type triggerFile struct {
	Action      string `json:"action"`
	RequestedAt string `json:"requestedAt"`
}

// TriggerWatcher monitors update-trigger.json written by the Node (via Web UI).
type TriggerWatcher struct {
	filePath string
}

func NewTriggerWatcher(dataDir string) *TriggerWatcher {
	return &TriggerWatcher{
		filePath: filepath.Join(dataDir, "update-trigger.json"),
	}
}

// Poll checks for a trigger file, reads the action, and deletes it.
// Returns empty string if no trigger is present.
func (tw *TriggerWatcher) Poll() string {
	data, err := os.ReadFile(tw.filePath)
	if err != nil {
		return ""
	}

	// Remove file immediately to avoid re-processing
	_ = os.Remove(tw.filePath)

	var tf triggerFile
	if json.Unmarshal(data, &tf) != nil {
		return ""
	}
	return tf.Action
}

func (tw *TriggerWatcher) FilePath() string {
	return tw.filePath
}

// WriteTrigger writes a trigger file (used by the Node side).
func WriteTrigger(dataDir, action string) error {
	path := filepath.Join(dataDir, "update-trigger.json")
	tf := triggerFile{
		Action:      action,
		RequestedAt: timeNowRFC3339(),
	}
	data, err := json.MarshalIndent(tf, "", "  ")
	if err != nil {
		return err
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}
