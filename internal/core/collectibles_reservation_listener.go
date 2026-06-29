//go:build !private_distribution

package core

import (
	"context"
	"time"

	"github.com/mobazha/mobazha3.0/internal/collectiblesdelivery"
	"github.com/mobazha/mobazha3.0/internal/logger"
	"github.com/mobazha/mobazha3.0/pkg/events"
)

func (n *MobazhaNode) startCollectibleReservationReleaseListener() {
	if n == nil || n.eventBus == nil || n.shutdown == nil || n.collectibleFirstSaleReservationReleaseHook == nil {
		return
	}
	sub, err := n.eventBus.Subscribe([]interface{}{
		&events.OrderCancel{},
		&events.OrderDeclined{},
		&events.OrderExpired{},
		&events.OrderAutoCancelled{},
	})
	if err != nil {
		logger.LogErrorWithIDf(log, n.nodeID, "Collectibles: reservation release subscription failed: %v", err)
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
				orderID, _, ok := collectiblesdelivery.TerminalEvent(event)
				if !ok || orderID == "" {
					continue
				}
				ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
				n.runCollectibleLifecycleDeliveries(ctx)
				cancel()
			case <-n.shutdown:
				return
			}
		}
	}()
}

func collectibleReservationReleaseEvent(event interface{}) (string, string) {
	orderID, reason, _ := collectiblesdelivery.TerminalEvent(event)
	return orderID, reason
}
