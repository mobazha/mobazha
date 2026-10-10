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
	logger.LogInfoWithID(log, obNode.nodeID, "BuyRequest subsystem initialized")
}
