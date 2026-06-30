// Package cli owns the shared command surface for Mobazha distributions.
// Distribution binaries supply their own start command while reusing the
// operational commands maintained by Open Core.
package cli

import (
	"fmt"

	"github.com/jessevdk/go-flags"
	"github.com/mobazha/mobazha3.0/cmd"
)

type commandSpec struct {
	name, short, long string
	command           any
}

// Run parses and executes the Mobazha command line. Keeping the start command
// injectable lets a private distribution use its composition root without
// copying the CLI or teaching Open Core about private modules.
func Run(startCommand any) error {
	if startCommand == nil {
		return fmt.Errorf("start command is required")
	}
	parser := flags.NewParser(nil, flags.Default)
	commands := []commandSpec{
		{"status", "get the Mobazha node status", "Get the Mobazha node status.", &cmd.Status{}},
		{"start", "start the Mobazha node", "Start the Mobazha node.", startCommand},
		{"init", "initialize a Mobazha node", "Create and initialize a data directory and database.", &cmd.Init{}},
		{"doctor", "diagnose the Mobazha node environment", "Check system resources, network connectivity, DNS, Docker, and node health.", &cmd.Doctor{}},
		{"backup", "back up the Mobazha data directory", "Create a compressed backup of the Mobazha data directory.", &cmd.Backup{}},
		{"devnet", "start a local dev net", "Start a local buyer, vendor, and moderator network with mock wallets and coins.", &cmd.DevNet{}},
	}
	for _, spec := range commands {
		if _, err := parser.AddCommand(spec.name, spec.short, spec.long, spec.command); err != nil {
			return fmt.Errorf("register %s command: %w", spec.name, err)
		}
	}

	mcpCommand, err := parser.AddCommand("mcp", "manage MCP connections for AI clients", "Detect, configure, and manage MCP client connections.", &cmd.MCP{})
	if err != nil {
		return fmt.Errorf("register mcp command: %w", err)
	}
	for _, spec := range []commandSpec{
		{"connect", "configure AI clients to use this store's MCP server", "Detect AI clients and write MCP configuration.", &cmd.MCPConnect{}},
		{"list", "list detected AI clients and their MCP status", "Show installed AI clients and their MCP status.", &cmd.MCPList{}},
		{"disconnect", "remove MCP configuration from AI clients", "Remove Mobazha MCP entries from one client or all clients.", &cmd.MCPDisconnect{}},
		{"bridge", "start a stdio-to-MCP bridge", "Bridge stdin/stdout to the Mobazha MCP endpoint.", &cmd.MCPBridge{}},
	} {
		if _, err := mcpCommand.AddCommand(spec.name, spec.short, spec.long, spec.command); err != nil {
			return fmt.Errorf("register mcp %s command: %w", spec.name, err)
		}
	}

	serviceCommand, err := parser.AddCommand("service", "manage the background service", "Install, start, stop, uninstall, or inspect the Mobazha system service.", &struct{}{})
	if err != nil {
		return fmt.Errorf("register service command: %w", err)
	}
	for _, spec := range []commandSpec{
		{"install", "install and start the background service", "Register Mobazha as an automatically started system service.", &cmd.ServiceInstall{}},
		{"start", "start the background service", "Start a previously installed Mobazha service.", &cmd.ServiceStart{}},
		{"stop", "stop the background service", "Stop the Mobazha service without uninstalling it.", &cmd.ServiceStop{}},
		{"uninstall", "stop and remove the background service", "Stop Mobazha and remove it from system startup.", &cmd.ServiceUninstall{}},
		{"status", "check the service status", "Show the Mobazha background service status.", &cmd.ServiceStatus{}},
	} {
		if _, err := serviceCommand.AddCommand(spec.name, spec.short, spec.long, spec.command); err != nil {
			return fmt.Errorf("register service %s command: %w", spec.name, err)
		}
	}

	_, err = parser.Parse()
	return err
}
