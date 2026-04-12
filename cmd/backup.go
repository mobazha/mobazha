package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/mobazha/mobazha3.0/internal/repo"
)

// Backup creates a compressed backup of the data directory.
type Backup struct {
	DataDir string `short:"d" long:"datadir" description:"specify the data directory to back up"`
	Testnet bool   `short:"t" long:"testnet" description:"use the test network data directory"`
	Output  string `short:"o" long:"output" description:"output file path (default: mobazha-backup-<timestamp>.tar.gz)"`
}

// Execute runs the backup command.
func (x *Backup) Execute(args []string) error {
	dataDir := x.DataDir
	if dataDir == "" {
		dataDir = repo.DefaultHomeDir
		if x.Testnet {
			dataDir = repo.DefaultHomeDir + "-testnet"
		}
	}

	if _, err := os.Stat(dataDir); os.IsNotExist(err) {
		return fmt.Errorf("data directory does not exist: %s", dataDir)
	}

	output := x.Output
	if output == "" {
		ts := time.Now().Format("20060102-150405")
		output = fmt.Sprintf("mobazha-backup-%s.tar.gz", ts)
	}

	absOutput, err := filepath.Abs(output)
	if err != nil {
		return err
	}

	absSource, _ := filepath.Abs(dataDir)
	if strings.HasPrefix(absOutput, absSource+string(os.PathSeparator)) {
		return fmt.Errorf("output path %s is inside the source directory %s; specify a path outside", absOutput, absSource)
	}

	fmt.Printf("Backing up %s → %s\n", dataDir, absOutput)

	if err := createTarGz(absOutput, dataDir); err != nil {
		return fmt.Errorf("backup failed: %w", err)
	}

	info, _ := os.Stat(absOutput)
	fmt.Printf("Backup complete: %s (%.1f MB)\n", absOutput, float64(info.Size())/1024/1024)
	return nil
}
