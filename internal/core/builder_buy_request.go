// SPDX-License-Identifier: MPL-2.0
// Copyright (c) 2026 fengzie and the respective contributors.

package core

import (
	"github.com/mobazha/mobazha/internal/database"
	"github.com/mobazha/mobazha/internal/logger"
)

// initBuyRequestSubsystem initializes the per-node wanted board: migrates the
// tables, creates the store, and wires the app service.
//
// Shared between standard and sovereign profiles (no build tags), matching the
// collection and shipping subsystems.
func initBuyRequestSubsystem(obNode *MobazhaNode) {
	if obNode == nil || obNode.db == nil {
		return
	}
	if err := database.MigrateBuyRequestModels(obNode.db); err != nil {
		logger.LogErrorWithIDf(log, obNode.nodeID, "BuyRequest: failed to migrate models: %v", err)
		return
	}
	store := database.NewGormBuyRequestStore(obNode.db)
	obNode.buyRequestService = NewBuyRequestAppService(store)
	// Board changes reach the search-service sync through the bus, so a request
	// is indexed the moment it is published and withdrawn the moment it closes.
	obNode.buyRequestService.SetEventBus(obNode.eventBus)
	logger.LogInfoWithID(log, obNode.nodeID, "BuyRequest subsystem initialized")
}

// initBuyRequestRelay connects the wanted board to the native p2p channel, so a
// seller on another node can offer on a request this node owns and this node
// can answer for a request it does not own. It runs after the messenger exists;
// without one the board keeps working, it just stays local.
func (n *MobazhaNode) initBuyRequestRelay() {
	if n == nil || n.infrastructureOnly || n.buyRequestService == nil {
		return
	}
	relay := newBuyRequestRelay(buyRequestRelayConfig{
		NodeID:    n.nodeID,
		PeerID:    n.peerID,
		DB:        n.db,
		Messenger: n.messenger,
		Board:     n.buyRequestService,
		EventBus:  n.eventBus,
	})
	if relay == nil {
		logger.LogInfoWithID(log, n.nodeID, "BuyRequest: no p2p transport, the board stays local")
		return
	}
	n.buyRequestRelay = relay
	n.buyRequestService.SetNotifier(relay)
	logger.LogInfoWithID(log, n.nodeID, "BuyRequest relay initialized")
}
