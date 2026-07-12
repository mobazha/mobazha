// SPDX-License-Identifier: MPL-2.0
// Copyright (c) 2026 fengzie and the respective contributors.

package paymentintent

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"math/big"
	"strings"
	"time"

	"github.com/mobazha/mobazha/pkg/models"
	"github.com/mobazha/mobazha/pkg/payment"
	"gorm.io/gorm"
)

// CryptoPaymentAttemptDraftRequest identifies one cross-peer attempt before
// key offers or target provisioning. AttemptID must be a stable correlation
// agreed by the participants; TenantID remains local routing metadata only.
type CryptoPaymentAttemptDraftRequest struct {
	TenantID                string
	AttemptID               string
	OrderID                 string
	AmountAtomic            string
	RailID                  string
	ExpectedModeratorPeerID string
}

// PrepareCryptoPaymentAttemptDraft converts an already-authorized module route
// into Core's immutable route binding and persists the non-actionable attempt.
// It never provisions a target or requests a remote key offer.
func PrepareCryptoPaymentAttemptDraft(
	db *gorm.DB,
	request CryptoPaymentAttemptDraftRequest,
	route payment.RouteIdentity,
) (models.PaymentAttempt, models.PaymentRouteBinding, error) {
	request.TenantID = strings.TrimSpace(request.TenantID)
	request.AttemptID = strings.TrimSpace(request.AttemptID)
	request.OrderID = strings.TrimSpace(request.OrderID)
	request.AmountAtomic = strings.TrimSpace(request.AmountAtomic)
	request.RailID = strings.TrimSpace(request.RailID)
	request.ExpectedModeratorPeerID = strings.TrimSpace(request.ExpectedModeratorPeerID)
	if request.AttemptID == "" || request.OrderID == "" || request.RailID == "" ||
		len(request.AttemptID) > 64 || len("ps_"+request.OrderID) > 255 {
		return models.PaymentAttempt{}, models.PaymentRouteBinding{}, fmt.Errorf("invalid crypto payment attempt draft request")
	}
	amount, ok := new(big.Int).SetString(request.AmountAtomic, 10)
	if !ok || amount.Sign() <= 0 || amount.String() != request.AmountAtomic {
		return models.PaymentAttempt{}, models.PaymentRouteBinding{}, fmt.Errorf("crypto payment attempt amount must be canonical positive atomic units")
	}
	if err := route.Validate(); err != nil {
		return models.PaymentAttempt{}, models.PaymentRouteBinding{}, err
	}
	if route.AssetID != request.RailID {
		return models.PaymentAttempt{}, models.PaymentRouteBinding{}, fmt.Errorf("crypto payment attempt route asset does not match rail")
	}

	routeBindingID := stableCryptoPaymentRouteBindingID(request.AttemptID, route)
	attempt := models.PaymentAttempt{
		TenantID: request.TenantID, AttemptID: request.AttemptID,
		Kind:             models.PaymentAttemptKindCryptoFundingTarget,
		PaymentSessionID: "ps_" + request.OrderID, OrderID: request.OrderID,
		AmountValue: request.AmountAtomic, Currency: request.RailID,
		RouteBindingID: routeBindingID, IdempotencyKey: "crypto-attempt:" + request.AttemptID,
		ExpectedModeratorPeerID: request.ExpectedModeratorPeerID,
	}
	binding := models.PaymentRouteBinding{
		TenantID: request.TenantID, RouteBindingID: routeBindingID, AttemptID: request.AttemptID,
		ContributionID: route.ContributionID, ModuleID: route.ModuleID,
		ImplementationGeneration: route.ImplementationGeneration,
		RailKind:                 route.RailKind, NetworkID: route.NetworkID, AssetID: route.AssetID,
		ProtocolVersion: route.ProtocolVersion, StateSchemaVersion: route.StateSchemaVersion,
		CreatedAt: time.Now().UTC(),
	}
	created, err := CreateCryptoPaymentAttemptDraft(db, attempt, binding)
	if err != nil {
		return models.PaymentAttempt{}, models.PaymentRouteBinding{}, err
	}
	var persistedBinding models.PaymentRouteBinding
	if err := db.Where("tenant_id = ? AND route_binding_id = ?", request.TenantID, routeBindingID).
		First(&persistedBinding).Error; err != nil {
		return models.PaymentAttempt{}, models.PaymentRouteBinding{}, fmt.Errorf("load persisted crypto payment route binding: %w", err)
	}
	return created, persistedBinding, nil
}

func stableCryptoPaymentRouteBindingID(attemptID string, route payment.RouteIdentity) string {
	digest := sha256.Sum256([]byte(strings.Join([]string{
		attemptID, route.ContributionID, route.ModuleID, route.ImplementationGeneration,
		route.RailKind, route.NetworkID, route.AssetID, route.ProtocolVersion, route.StateSchemaVersion,
	}, "\x00")))
	return "prb_" + hex.EncodeToString(digest[:])[:60]
}
