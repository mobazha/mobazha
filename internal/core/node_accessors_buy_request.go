package core

import "github.com/mobazha/mobazha/pkg/contracts"

var _ contracts.BuyRequestProvider = (*MobazhaNode)(nil)

// BuyRequestProvider implementation — per-node wanted board subsystem.
func (n *MobazhaNode) BuyRequests() contracts.BuyRequestService {
	if n.buyRequestService == nil {
		return nil
	}
	return n.buyRequestService
}
