package cmd

import (
	"context"
	"github.com/cpacia/openbazaar3.0/core"
	"github.com/cpacia/openbazaar3.0/repo"
)

// Start is the main entry point for openbazaar-go. The options to this
// command are the same as the OpenBazaar node config options.
type Start struct {
	repo.Config
}

// Execute starts the OpenBazaar node.
func (x *Start) Execute(args []string) error {
	cfg, _, err := repo.LoadConfig()
	if err != nil {
		return err
	}

	_, err = core.NewNode(context.Background(), cfg)
	if err != nil {
		return err
	}

	return nil
}
