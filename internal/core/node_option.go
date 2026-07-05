package core

import (
	"fmt"

	"github.com/mobazha/mobazha/pkg/distribution"
)

// NodeOption configures node construction. Build options are resolved before
// any repository, listener, or runtime resource is created; apply options are
// injected before App Services and payment modules are wired.
type NodeOption struct {
	configure func(*nodeBuildOptions) error
	apply     func(*MobazhaNode)
}

type nodeBuildOptions struct {
	sovereign              *distribution.SovereignNodeConfig
	collateralRailInjected bool
}

func resolveNodeBuildOptions(options []NodeOption) (nodeBuildOptions, error) {
	var resolved nodeBuildOptions
	for _, option := range options {
		if option.configure == nil {
			continue
		}
		if err := option.configure(&resolved); err != nil {
			return nodeBuildOptions{}, err
		}
	}
	return resolved, nil
}

func applyNodeOptions(node *MobazhaNode, options []NodeOption) {
	for _, option := range options {
		if option.apply != nil {
			option.apply(node)
		}
	}
}

// WithSovereignNode selects the single-node, local-first runtime composition.
// The configuration is copied and validated before construction starts.
func WithSovereignNode(config distribution.SovereignNodeConfig) NodeOption {
	owned := config.Clone()
	return NodeOption{configure: func(options *nodeBuildOptions) error {
		if options.sovereign != nil {
			return fmt.Errorf("sovereign node composition configured more than once")
		}
		if err := owned.Validate(); err != nil {
			return err
		}
		options.sovereign = &owned
		return nil
	}}
}
