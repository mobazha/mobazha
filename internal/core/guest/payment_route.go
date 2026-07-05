// SPDX-License-Identifier: MPL-2.0

package guest

import (
	"time"

	"github.com/mobazha/mobazha/pkg/models"
	"github.com/mobazha/mobazha/pkg/payment"
)

func setGuestOrderPaymentRoute(order *models.GuestOrder, route payment.RouteIdentity, grace time.Duration) {
	if order == nil || route.IsZero() {
		return
	}
	order.RouteContributionID = route.ContributionID
	order.RouteModuleID = route.ModuleID
	order.RouteImplementationGeneration = route.ImplementationGeneration
	order.RouteRailKind = route.RailKind
	order.RouteNetworkID = route.NetworkID
	order.RouteAssetID = route.AssetID
	order.RouteProtocolVersion = route.ProtocolVersion
	order.RouteStateSchemaVersion = route.StateSchemaVersion
	if grace > 0 {
		order.PaymentGracePeriodNanos = int64(grace)
	}
}

func guestOrderPaymentRoute(order *models.GuestOrder) payment.RouteIdentity {
	if order == nil {
		return payment.RouteIdentity{}
	}
	return payment.RouteIdentity{
		ContributionID: order.RouteContributionID, ModuleID: order.RouteModuleID,
		ImplementationGeneration: order.RouteImplementationGeneration,
		RailKind:                 order.RouteRailKind, NetworkID: order.RouteNetworkID, AssetID: order.RouteAssetID,
		ProtocolVersion: order.RouteProtocolVersion, StateSchemaVersion: order.RouteStateSchemaVersion,
	}
}

func guestOrderPaymentGrace(order *models.GuestOrder, fallback time.Duration) time.Duration {
	if order != nil && order.PaymentGracePeriodNanos > 0 {
		return time.Duration(order.PaymentGracePeriodNanos)
	}
	return fallback
}
