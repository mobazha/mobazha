package core

import "github.com/mobazha/mobazha/pkg/contracts"

var (
	_ contracts.BuyRequestProvider      = (*MobazhaNode)(nil)
	_ contracts.BuyRequestRelayProvider = (*MobazhaNode)(nil)
)

// BuyRequestProvider implementation — per-node wanted board subsystem.
func (n *MobazhaNode) BuyRequests() contracts.BuyRequestService {
	if n.buyRequestService == nil {
		return nil
	}
	return n.buyRequestService
}

// BuyRequestRelayProvider implementation — the wanted board's cross-node
// transport. It is nil on a node that has no messenger, where the board simply
// stays local.
func (n *MobazhaNode) BuyRequestRelay() contracts.BuyRequestRelay {
	if n.buyRequestRelay == nil {
		// A typed nil would satisfy the interface and panic on use, so hand
		// back an untyped nil the caller can test.
		return nil
	}
	return n.buyRequestRelay
}
