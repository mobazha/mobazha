package core

import (
	"context"

	"github.com/mobazha/mobazha3.0/internal/core"
	"github.com/mobazha/mobazha3.0/pkg/repo"
)

type OpenBazaarNode = core.OpenBazaarNode

func NewNode(ctx context.Context, cfg *repo.Config, nodeID string) (*OpenBazaarNode, error) {
	return core.NewNode(ctx, cfg, nodeID)
}
