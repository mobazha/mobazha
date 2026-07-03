package core

import (
	"context"
	"sort"
	"testing"

	"github.com/libp2p/go-libp2p/core/peer"
	pb "github.com/mobazha/mobazha/pkg/net/mbzpb"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type spyNetworkService struct {
	registered map[pb.Message_MessageType]bool
}

func newSpyNetworkService() *spyNetworkService {
	return &spyNetworkService{registered: make(map[pb.Message_MessageType]bool)}
}

func (s *spyNetworkService) RegisterHandler(mt pb.Message_MessageType, _ func(peer.ID, *pb.Message) error) {
	s.registered[mt] = true
}
func (s *spyNetworkService) SendMessage(_ context.Context, _ peer.ID, _ *pb.Message) error {
	return nil
}
func (s *spyNetworkService) DeliverLocalMessage(_ peer.ID, _ *pb.Message) error { return nil }
func (s *spyNetworkService) Close()                                             {}

func TestRegisterHandlers_CoversAllMessageTypes(t *testing.T) {
	spy := newSpyNetworkService()
	node := &MobazhaNode{networkFields: networkFields{networkService: spy}}
	node.registerHandlers()

	allTypes := pb.Message_MessageType_name
	require.NotEmpty(t, allTypes, "protobuf enum map should not be empty")

	activeCount := 0
	var missing []string
	for num, name := range allTypes {
		mt := pb.Message_MessageType(num)
		activeCount++
		if !spy.registered[mt] {
			missing = append(missing, name)
		}
	}
	sort.Strings(missing)
	assert.Empty(t, missing,
		"these message types have no handler in registerHandlers(): %v", missing)

	assert.Len(t, spy.registered, activeCount,
		"handler count should match active proto enum count")
}
