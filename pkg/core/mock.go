package core

import (
	"github.com/mobazha/mobazha/internal/core"
)

type Mocknet = core.Mocknet

func NewMocknet(numNodes int) (*Mocknet, error) {
	return core.NewMocknet(numNodes)
}
