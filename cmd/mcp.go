package cmd

// MCP is the top-level container for MCP-related subcommands.
// Subcommands are registered on it in mobazha.go.
type MCP struct{}

func (x *MCP) Execute(args []string) error {
	return nil
}
