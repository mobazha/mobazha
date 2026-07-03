package api

import (
	"context"
	"encoding/json"

	"github.com/mobazha/mobazha/pkg/contracts"
)

// StartMatrixChatEventBridge subscribes to Matrix chat events from the given
// MatrixChatService and forwards them to the WebSocket hub for the specified node.
// The bridge runs until ctx is cancelled. It should be started as a goroutine
// after the node's MatrixChatService.Start() completes.
func (g *Gateway) StartMatrixChatEventBridge(ctx context.Context, nodeID string, svc contracts.MatrixChatService) {
	if svc == nil {
		return
	}

	eventCh, err := svc.Subscribe(ctx)
	if err != nil {
		log.Errorf("Failed to subscribe to matrix chat events for node %s: %v", nodeID, err)
		return
	}

	notifyFn := g.NotifyWebsockets(nodeID)

	for {
		select {
		case <-ctx.Done():
			return
		case evt, ok := <-eventCh:
			if !ok {
				return
			}
			wrapped := wsMessage{
				Type: evt.Type,
			}
			dataBytes, err := json.Marshal(evt.Data)
			if err != nil {
				log.Errorf("Failed to marshal matrix chat event data: %v", err)
				continue
			}
			wrapped.Data = json.RawMessage(dataBytes)
			if err := notifyFn(wrapped); err != nil {
				log.Debugf("Failed to broadcast matrix chat event to WS hub (node %s): %v", nodeID, err)
			}
		}
	}
}
