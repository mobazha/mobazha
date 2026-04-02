package cmd

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/mobazha/mobazha3.0/internal/doctor"
	"github.com/mobazha/mobazha3.0/internal/repo"
)

type Doctor struct {
	DataDir string `short:"d" long:"datadir" description:"specify the data directory to be used"`
	Testnet bool   `short:"t" long:"testnet" description:"use the test network"`
	JSON    bool   `long:"json" description:"output results as JSON"`
	Export  string `long:"export" description:"export diagnostic bundle to specified path (tar.gz)"`
}

func (x *Doctor) Execute(args []string) error {
	if x.DataDir == "" {
		x.DataDir = repo.DefaultHomeDir
		if x.Testnet {
			x.DataDir = repo.DefaultHomeDir + "-testnet"
		}
	}

	cfg := doctor.DefaultConfig()
	cfg.DataDir = x.DataDir
	cfg.Testnet = x.Testnet

	runner := doctor.NewRunner(cfg)
	summary := runner.RunAll()

	if x.Export != "" {
		if err := doctor.ExportBundle(x.Export, cfg, summary); err != nil {
			return err
		}
		fmt.Printf("Diagnostic bundle exported to: %s\n", x.Export)
		return nil
	}

	if x.JSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(summary)
	}

	for _, r := range summary.Results {
		icon := "✅"
		if r.Status == doctor.StatusWarn {
			icon = "⚠️"
		} else if r.Status == doctor.StatusFail {
			icon = "❌"
		}
		line := fmt.Sprintf("%s  %s", icon, r.Name)
		if r.Detail != "" {
			line += fmt.Sprintf(" — %s", r.Detail)
		}
		fmt.Println(line)
	}

	fmt.Println()
	fmt.Printf("Results: %d passed, %d warnings, %d failed\n", summary.Pass, summary.Warn, summary.Fail)
	if summary.Fail > 0 {
		os.Exit(1)
	}
	return nil
}
