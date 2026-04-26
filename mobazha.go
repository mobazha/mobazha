package main

import (
	"log"
	"os"

	"github.com/jessevdk/go-flags"
	"github.com/mobazha/mobazha3.0/cmd"
)

func main() {
	parser := flags.NewParser(nil, flags.Default)

	_, err := parser.AddCommand("status",
		"get the Mobazha node status",
		"The status command gets the Mobazha node status",
		&cmd.Status{})
	if err != nil {
		log.Fatal(err)
	}

	_, err = parser.AddCommand("start",
		"start the Mobazha node",
		"The start command starts the Mobazha node",
		&cmd.Start{})
	if err != nil {
		log.Fatal(err)
	}
	_, err = parser.AddCommand("init",
		"initialize an Mobazha node",
		"The init command creates and initializes a new data directory and database.",
		&cmd.Init{})
	if err != nil {
		log.Fatal(err)
	}
	_, err = parser.AddCommand("doctor",
		"diagnose the Mobazha node environment",
		"The doctor command checks system resources, network connectivity, DNS resolution, Docker status, and node health.",
		&cmd.Doctor{})
	if err != nil {
		log.Fatal(err)
	}
	_, err = parser.AddCommand("backup",
		"back up the Mobazha data directory",
		"The backup command creates a compressed archive of the data directory for safekeeping.",
		&cmd.Backup{})
	if err != nil {
		log.Fatal(err)
	}
	_, err = parser.AddCommand("devnet",
		"start a local dev net",
		"The devnet command spins up a local network of three nodes (buyer, vendor, moderator)"+
			"that connects all three nodes together and uses a mock wallet and mock coins. Each node is pre-populated"+
			"with data for ease of use.",
		&cmd.DevNet{})
	if err != nil {
		log.Fatal(err)
	}

	mcpCmd, err := parser.AddCommand("mcp",
		"manage MCP connections for AI clients",
		"Detect, configure, and manage MCP (Model Context Protocol) connections between this node and AI clients like Cursor, Claude, and VS Code.",
		&cmd.MCP{})
	if err != nil {
		log.Fatal(err)
	}
	if _, err := mcpCmd.AddCommand("connect",
		"configure AI clients to use this store's MCP server",
		"Auto-detect installed AI clients and write MCP configuration. Optionally specify a client name as argument.",
		&cmd.MCPConnect{}); err != nil {
		log.Fatal(err)
	}
	if _, err := mcpCmd.AddCommand("list",
		"list detected AI clients and their MCP status",
		"Scan the system for installed AI clients and show which ones are configured.",
		&cmd.MCPList{}); err != nil {
		log.Fatal(err)
	}
	if _, err := mcpCmd.AddCommand("disconnect",
		"remove MCP configuration from AI clients",
		"Remove Mobazha MCP server entries from client configuration files. Pass a client name or 'all'.",
		&cmd.MCPDisconnect{}); err != nil {
		log.Fatal(err)
	}
	if _, err := mcpCmd.AddCommand("bridge",
		"start a stdio-to-MCP bridge",
		"Start a bidirectional bridge between stdin/stdout and the MCP endpoint. "+
			"This is typically launched automatically by AI clients (e.g. Claude Desktop) "+
			"via their JSON config, not run manually.",
		&cmd.MCPBridge{}); err != nil {
		log.Fatal(err)
	}

	serviceCmd, err := parser.AddCommand("service",
		"manage the background service",
		"Install, uninstall, or check the status of the Mobazha background service (systemd on Linux, launchd on macOS).",
		&struct{}{})
	if err != nil {
		log.Fatal(err)
	}
	if _, err := serviceCmd.AddCommand("install",
		"install and start the background service",
		"Register Mobazha as a system service that starts automatically on boot.",
		&cmd.ServiceInstall{}); err != nil {
		log.Fatal(err)
	}
	if _, err := serviceCmd.AddCommand("start",
		"start the background service",
		"Start a previously stopped Mobazha service.",
		&cmd.ServiceStart{}); err != nil {
		log.Fatal(err)
	}
	if _, err := serviceCmd.AddCommand("stop",
		"stop the background service",
		"Stop the Mobazha background service. Use 'mobazha service install' to start it again.",
		&cmd.ServiceStop{}); err != nil {
		log.Fatal(err)
	}
	if _, err := serviceCmd.AddCommand("uninstall",
		"stop and remove the background service",
		"Stop the Mobazha service and remove it from system startup.",
		&cmd.ServiceUninstall{}); err != nil {
		log.Fatal(err)
	}
	if _, err := serviceCmd.AddCommand("status",
		"check the service status",
		"Show the current status of the Mobazha background service.",
		&cmd.ServiceStatus{}); err != nil {
		log.Fatal(err)
	}

	if _, err := parser.Parse(); err != nil {
		os.Exit(1)
	}
}
