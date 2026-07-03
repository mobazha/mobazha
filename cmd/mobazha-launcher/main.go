package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/mobazha/mobazha/internal/repo"
	"github.com/mobazha/mobazha/internal/supervisor"
)

const envBgKey = "MOBAZHA_LAUNCHER_BG"

func main() {
	nodeDataDir := flag.String("node-data-dir", "", "Data directory for the mobazha node (-d flag)")
	gatewayPort := flag.String("gateway-port", repo.DefaultGatewayPort, "Gateway port the node listens on")
	testnet := flag.Bool("testnet", false, "Run node in testnet mode")
	flag.Parse()

	home, _ := os.UserHomeDir()
	dataDir := filepath.Join(home, ".mobazha")
	logFile := setupLogging(dataDir)
	if logFile != nil {
		defer logFile.Close()
	}

	logger := log.New(os.Stdout, "[launcher] ", log.LstdFlags)

	var nodeArgs []string
	if *nodeDataDir != "" {
		nodeArgs = append(nodeArgs, "-d", *nodeDataDir)
	}
	if *testnet {
		nodeArgs = append(nodeArgs, "-t")
	}
	if *gatewayPort != repo.DefaultGatewayPort {
		nodeArgs = append(nodeArgs, fmt.Sprintf("--gatewayaddr=/ip4/127.0.0.1/tcp/%s", *gatewayPort))
	}

	ui := createUI(logger)

	s := supervisor.New(supervisor.Options{
		DataDir:     dataDir,
		GatewayPort: *gatewayPort,
		NodeArgs:    nodeArgs,
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
