package core

import "github.com/mobazha/mobazha/internal/core/digital"

func wireDistributionDigitalSupplyLineResolvers(
	n *MobazhaNode,
	assets *digital.DigitalAssetAppService,
) {
	if n.orderService != nil {
		n.orderService.SetDigitalSupplyLineResolver(assets)
	}
	if n.guestOrderService != nil {
		n.guestOrderService.SetDigitalSupplyLineResolver(assets)
	}
}
