package core

import (
	"context"
	"time"

	"github.com/mobazha/mobazha/internal/extensiondelivery"
	"github.com/mobazha/mobazha/internal/logger"
	"github.com/mobazha/mobazha/pkg/events"
)

func (n *MobazhaNode) startOrderExtensionEventListener() {
	if n == nil || n.eventBus == nil || n.shutdown == nil || len(n.orderExtensionModules) == 0 {
		return
	}
	sub, err := n.eventBus.Subscribe([]interface{}{
		&events.OrderCancel{},
		&events.OrderDeclined{},
		&events.OrderExpired{},
		&events.OrderAutoCancelled{},
	})
	if err != nil {
		logger.LogErrorWithIDf(log, n.nodeID, "Extensions: order event subscription failed: %v", err)
		return
	}
	go func() {
		defer sub.Close()
		for {
			select {
			case event, ok := <-sub.Out():
				if !ok {
					return
				}
				orderID, _, ok := extensiondelivery.TerminalEvent(event)
				if !ok || orderID == "" {
					continue
				}
				ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
				n.runExtensionDeliveries(ctx)
				cancel()
			case <-n.shutdown:
				return
			}
		}
	}()
}
