package core

import (
	"context"
	"errors"
	"sync/atomic"
	"time"

	"github.com/mobazha/mobazha/internal/logger"
	"github.com/mobazha/mobazha/pkg/core/coreiface"
	"github.com/mobazha/mobazha/pkg/events"
)

// Start gets the node up and running and listens for a signal interrupt.
func (n *MobazhaNode) Start() {
	if n.sovereign {
		n.startSovereign()
		return
	}

	go func() {
		if err := n.checkRepoMigration(); err != nil {
			logger.LogErrorWithIDf(log, n.nodeID, "checkRepoMigration failed, %v", err)
		}
	}()

	go n.bootstrapDHT()

	if n.IsDefaultNode() {
		go n.SharedManager().Start()
	}

	if !n.infrastructureOnly {
		n.publishHandler()
		go n.messenger.Start()
		go n.followerTracker.Start()

		go n.orderProcessor.Start()
		go n.syncMessages()
		go func() {
			n.multiwallet.Start()
		}()

		if n.eventDispatcher != nil {
			if err := n.eventDispatcher.Start(); err != nil {
				logger.LogErrorWithIDf(log, n.nodeID, "Failed to start event dispatcher: %v", err)
			}
		}
		if err := n.profileService.UpdateSNFServers(); err != nil {
			logger.LogErrorWithIDf(log, n.nodeID, "Error updating store and forward servers in profile: %s", err)
		}

		n.startEVMChainClients()

		n.startTRONChainClients()

		if err := n.registerPaymentStrategies(); err != nil {
			logger.LogErrorWithIDf(log, n.nodeID, "Payment strategy startup failed: %v", err)
			if stopErr := n.Stop(true); stopErr != nil {
				logger.LogErrorWithIDf(log, n.nodeID, "Node cleanup after payment strategy startup failure failed: %v", stopErr)
			}
			return
		}

		n.startCancelablePaymentMonitor()

		n.startFiatPaymentMonitor()

		n.startPaymentEventMonitors()

		go n.startUTXOPaymentMonitor()

		if n.matrixChatService != nil {
			go func() {
				if err := n.matrixChatService.Start(n.nodeCtx); err != nil {
					logger.LogErrorWithIDf(log, n.nodeID, "Matrix chat service start failed: %v", err)
				}
			}()
			if n.hostService == nil {
				if gw := n.SharedManager().GetHTTPGateway(); gw != nil {
					go gw.StartMatrixChatEventBridge(n.nodeCtx, n.nodeID, n.matrixChatService)
				}
			}
		}

		if n.netDBSyncService != nil {
			n.netDBSyncService.Start()
			go n.netDBSyncService.Reconcile()
		}

		if n.hostService == nil {
			n.startStandaloneScheduler(n.nodeCtx)
		}
	}

	go func() {
		if n.peerHost == nil {
			return
		}
		conns := n.peerHost.Network().Conns()
		for _, conn := range conns {
			streams := conn.GetStreams()
			logger.LogDebugWithIDf(log, n.nodeID, "Connection to %s has %d streams",
				conn.RemotePeer(), len(streams))
		}
	}()
}

// Stop cleanly shuts down the MobazhaNode and signals to any
// listening goroutines that it's time to stop.
func (n *MobazhaNode) Stop(force bool) error {
	if n.sovereign {
		return n.stopSovereign()
	}

	if atomic.LoadInt32(&n.publishActive) > 0 && !force {
		return coreiface.ErrPublishingActive
	}
	if !atomic.CompareAndSwapInt32(&n.stopped, 0, 1) {
		return nil
	}
	var stopErr error
	if n.paymentModuleManager != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		stopErr = n.paymentModuleManager.Stop(ctx)
		cancel()
		if stopErr != nil {
			logger.LogErrorWithIDf(log, n.nodeID, "stop trusted payment modules: %v", stopErr)
			atomic.StoreInt32(&n.stopped, 0)
			return stopErr
		}
	}

	if n.IsDefaultNode() {
		n.SharedManager().Stop()
	}

	if !n.infrastructureOnly {
		if n.messenger != nil {
			n.messenger.Stop()
		}
		if n.networkService != nil {
			n.networkService.Close()
		}
		if n.orderProcessor != nil {
			n.orderProcessor.Stop()
		}
		if n.orderLockManager != nil {
			n.orderLockManager.Stop()
		}
		if n.followerTracker != nil {
			n.followerTracker.Close()
		}
		if n.multiwallet != nil {
			n.multiwallet.Close()
		}
	}
	if n.eventDispatcher != nil {
		n.eventDispatcher.Stop()
	}
	if n.shutdownTorFunc != nil {
		n.shutdownTorFunc()
	}
	n.StopUTXOPaymentMonitor()
	if n.shutdown != nil {
		close(n.shutdown)
	}
	if n.matrixChatService != nil {
		if err := n.matrixChatService.Stop(); err != nil {
			log.Errorf("Matrix chat service stop error: %v", err)
		}
	}
	if n.netDBSyncService != nil {
		n.netDBSyncService.Stop()
	}

	if n.repo != nil {
		n.repo.Close()
	}

	if n.nodeCancel != nil {
		n.nodeCancel()
	}

	if n.p2pInfra != nil {
		stop := make(chan struct{})
		go func() {
			n.p2pInfra.Close()
			close(stop)
		}()
		select {
		case <-time.After(time.Second * 2):
			log.Warning("P2P infrastructure close timed out after 2s, proceeding with shutdown")
			if n.eventBus != nil {
				n.eventBus.Emit(&events.P2PShutdown{})
			}
			return errors.Join(stopErr, coreiface.ErrP2PDelayedShutdown)
		case <-stop:
			if n.eventBus != nil {
				n.eventBus.Emit(&events.P2PShutdown{})
			}
		}
	} else {
		if n.peerHost != nil {
			n.peerHost.Close()
		}
		if n.eventBus != nil {
			n.eventBus.Emit(&events.P2PShutdown{})
		}
	}
	return stopErr
}

// startSovereign starts the deliberately small local-first runtime. The
// sovereign distribution shares the same node type and lifecycle state
// machine as every other deployment, but opts out of network-facing workers.
func (n *MobazhaNode) startSovereign() {
	go func() {
		if err := n.checkRepoMigration(); err != nil {
			logger.LogErrorWithIDf(log, n.nodeID, "checkRepoMigration failed, %v", err)
		}
	}()

	if n.sharedManager != nil {
		n.sharedManager.Start()
	}
	if n.eventDispatcher != nil {
		if err := n.eventDispatcher.Start(); err != nil {
			logger.LogErrorWithIDf(log, n.nodeID, "failed to start event dispatcher: %v", err)
		}
	}
	if err := n.runDistributionPaymentModules(n.nodeCtx); err != nil {
		logger.LogErrorWithIDf(log, n.nodeID, "trusted payment module startup failed: %v", err)
		if stopErr := n.Stop(true); stopErr != nil {
			logger.LogErrorWithIDf(log, n.nodeID, "node cleanup after payment module failure failed: %v", stopErr)
		}
		return
	}

	n.startStandaloneScheduler(n.nodeCtx)
	n.restoreGuestPaymentWatches()
	logger.LogInfoWithIDf(log, n.nodeID, "sovereign local-first node started")
}

func (n *MobazhaNode) restoreGuestPaymentWatches() {
	if n.guestOrderService == nil || n.guestPaymentMonitor == nil {
		return
	}
	orders, err := n.guestOrderService.ListActiveOrders(n.nodeCtx)
	if err != nil {
		logger.LogErrorWithIDf(log, n.nodeID, "restore guest payment watches: list active orders failed: %v", err)
		return
	}
	for _, order := range orders {
		n.guestPaymentMonitor.WatchOrder(order)
	}
	if len(orders) > 0 {
		logger.LogInfoWithIDf(log, n.nodeID, "restored %d guest payment watches", len(orders))
	}
}

func (n *MobazhaNode) stopSovereign() error {
	if !atomic.CompareAndSwapInt32(&n.stopped, 0, 1) {
		return nil
	}
	if n.shutdown != nil {
		close(n.shutdown)
	}
	if n.guestPaymentMonitor != nil {
		n.guestPaymentMonitor.StopAll()
	}
	var closeErr error
	if n.paymentModuleManager != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		if err := n.paymentModuleManager.Stop(ctx); err != nil {
			logger.LogErrorWithIDf(log, n.nodeID, "stop trusted payment modules: %v", err)
			closeErr = err
		}
		cancel()
	}
	if n.eventDispatcher != nil {
		n.eventDispatcher.Stop()
	}
	if n.repo != nil {
		n.repo.Close()
	}
	if n.nodeCancel != nil {
		n.nodeCancel()
	}
	if n.sharedManager != nil {
		n.sharedManager.Stop()
	}
	return closeErr
}
