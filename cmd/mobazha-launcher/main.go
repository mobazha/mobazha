package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/mobazha/mobazha3.0/internal/repo"
	"github.com/mobazha/mobazha3.0/internal/supervisor"
)

const envBgKey = "MOBAZHA_LAUNCHER_BG"

func main() {
	home, _ := os.UserHomeDir()
	dataDir := filepath.Join(home, ".mobazha")
	logFile := setupLogging(dataDir)
	if logFile != nil {
		defer logFile.Close()
	}

	logger := log.New(os.Stdout, "[launcher] ", log.LstdFlags)

	ui := createUI(logger)

	s := supervisor.New(supervisor.Options{
		DataDir:     dataDir,
		GatewayPort: repo.DefaultGatewayPort,
		UI:          ui,
		Logger:      logger,
	})

	s.Run()
}

func setupLogging(dataDir string) *os.File {
	logDir := filepath.Join(dataDir, "logs")
	if err := os.MkdirAll(logDir, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "failed to create log dir: %v\n", err)
		return nil
	}
	logPath := filepath.Join(logDir, "launcher.log")
	f, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to open log file: %v\n", err)
		return nil
	}
	// Log to both stdout and file
	log.SetOutput(f)
	return f
}
