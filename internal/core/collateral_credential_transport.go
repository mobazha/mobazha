// SPDX-License-Identifier: MPL-2.0
// Copyright (c) 2026 fengzie and the respective contributors.

package core

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/libp2p/go-libp2p/core/peer"
	corecollateral "github.com/mobazha/mobazha/internal/collateral"
	"github.com/mobazha/mobazha/internal/logger"
	"github.com/mobazha/mobazha/internal/orderextensions"
	pkgcollateral "github.com/mobazha/mobazha/pkg/collateral"
	"github.com/mobazha/mobazha/pkg/contracts"
	"github.com/mobazha/mobazha/pkg/database"
	"github.com/mobazha/mobazha/pkg/extensions"
	"github.com/mobazha/mobazha/pkg/models"
	pb "github.com/mobazha/mobazha/pkg/net/mbzpb"
	"google.golang.org/protobuf/types/known/anypb"
	"gorm.io/gorm"
)

const (
	collateralCredentialRequestMinInterval = 2 * time.Minute
	collateralCredentialRefreshWindow      = 5 * time.Minute
	collateralCredentialRequestedTTL       = 10 * time.Minute
	collateralCredentialRefreshBatchSize   = 100
)

func (s *collateralAllocationService) RequestOrderExtensionCollateralCredential(
	ctx context.Context,
	request contracts.RequestOrderExtensionCollateralCredentialRequest,
) error {
	if s == nil || s.db == nil || s.signer == nil || s.messenger == nil {
		return fmt.Errorf("collateral credential transport is unavailable")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	now := s.now().UTC().Truncate(time.Second)
	if err := request.Validate(now); err != nil {
		return err
	}
	if err := s.db.Update(func(tx database.Tx) error {
		return s.requestOrderExtensionCollateralCredentialTx(tx, request, now)
	}); err != nil {
		return fmt.Errorf("persist collateral credential request: %w", err)
	}
	return nil
}

func (s *collateralAllocationService) requestOrderExtensionCollateralCredentialTx(
	tx database.Tx,
	request contracts.RequestOrderExtensionCollateralCredentialRequest,
	now time.Time,
) error {
	if tx == nil {
		return fmt.Errorf("collateral credential request transaction is required")
	}
	if err := request.Validate(now); err != nil {
		return err
	}
	var seller peer.ID
	var order models.Order
	if err := tx.Read().Where("id = ?", request.OrderID).First(&order).Error; err != nil {
		return fmt.Errorf("load collateral credential buyer order: %w", err)
	}
	if order.Role() != models.RoleBuyer {
		return fmt.Errorf("collateral credential may only be requested from the buyer order copy")
	}
	buyer, err := order.Buyer()
	if err != nil {
		return err
	}
	if buyer.String() != string(s.signer.PeerID()) {
		return fmt.Errorf("collateral credential requester does not own the buyer order copy")
	}
	seller, err = order.Vendor()
	if err != nil {
		return err
	}
	if seller == buyer {
		return fmt.Errorf("collateral credential seller must differ from buyer")
	}
	persisted, err := orderextensions.LatestByIDTx(tx, request.OrderID, request.Extension.ExtensionID)
	if err != nil {
		return err
	}
	persistedDigest, err := corecollateral.OrderExtensionDigest(persisted)
	if err != nil {
		return err
	}
	requestDigest, err := corecollateral.OrderExtensionDigest(request.Extension)
	if err != nil {
		return err
	}
	if persisted.Revision != request.Extension.Revision || persistedDigest != requestDigest {
		return fmt.Errorf("collateral credential order extension revision is stale")
	}

	extensionJSON, err := json.Marshal(request.Extension)
	if err != nil {
		return err
	}
	requirementJSON, err := json.Marshal(request.Requirement)
	if err != nil {
		return err
	}
	payload, err := anypb.New(&pb.CollateralCredentialRequest{
		OrderID: request.OrderID, ExtensionJson: extensionJSON, RequirementJson: requirementJSON,
		ExpiresAtUnix: request.ExpiresAt.UTC().Truncate(time.Second).Unix(),
	})
	if err != nil {
		return err
	}
	message := newMessageWithID()
	message.MessageType = pb.Message_COLLATERAL_CREDENTIAL_REQUEST
	message.Payload = payload
	claimed, err := corecollateral.ClaimCredentialRequestTx(
		tx, request.OrderID, request.Extension.ExtensionID, request.Extension.Revision,
		string(s.signer.PeerID()), message.MessageID, now, collateralCredentialRequestMinInterval,
	)
	if err != nil || !claimed {
		return err
	}
	return s.messenger.ReliablySendMessage(tx, seller, message, nil)
}

func (n *MobazhaNode) RunCollateralCredentialRefreshOnce(ctx context.Context) {
	if err := n.runCollateralCredentialRefreshOnceAt(ctx, time.Now().UTC()); err != nil {
		logger.LogErrorWithIDf(log, n.nodeID, "Collateral credential refresh failed: %v", err)
	}
}

func (n *MobazhaNode) runCollateralCredentialRefreshOnceAt(ctx context.Context, now time.Time) error {
	if n == nil || n.db == nil || n.signer == nil || n.messenger == nil {
		return nil
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	now = now.UTC().Truncate(time.Second)
	audiencePeerID := string(n.signer.PeerID())
	var candidates []corecollateral.CredentialRefreshCandidate
	if err := n.db.View(func(tx database.Tx) error {
		var err error
		candidates, err = corecollateral.DueCredentialRefreshesTx(
			tx, audiencePeerID, now, now.Add(collateralCredentialRefreshWindow),
			now.Add(-collateralCredentialRequestMinInterval), collateralCredentialRefreshBatchSize,
		)
		return err
	}); err != nil {
		return err
	}

	service := &collateralAllocationService{
		db: n.db, signer: n.signer, messenger: n.messenger,
		now: func() time.Time { return now },
	}
	var refreshErr error
	for _, candidate := range candidates {
		if err := ctx.Err(); err != nil {
			return errors.Join(refreshErr, err)
		}
		var order models.Order
		var extension extensions.OrderExtension
		if err := n.db.View(func(tx database.Tx) error {
			if err := tx.Read().Where("id = ?", candidate.OrderID).First(&order).Error; err != nil {
				return err
			}
			var err error
			extension, err = orderextensions.LatestByIDTx(tx, candidate.OrderID, candidate.ExtensionID)
			return err
		}); err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				if dismissErr := n.dismissCollateralCredentialRefresh(candidate); dismissErr != nil {
					refreshErr = errors.Join(refreshErr, dismissErr)
				}
				continue
			}
			refreshErr = errors.Join(refreshErr, fmt.Errorf("load collateral credential refresh state for order %s: %w", candidate.OrderID, err))
			continue
		}
		if order.Role() != models.RoleBuyer || extension.Revision != candidate.ExtensionRevision {
			if err := n.dismissCollateralCredentialRefresh(candidate); err != nil {
				refreshErr = errors.Join(refreshErr, err)
			}
			continue
		}
		orderOpen, err := order.OrderOpenMessage()
		if err != nil {
			refreshErr = errors.Join(refreshErr, fmt.Errorf("decode collateral credential order %s: %w", candidate.OrderID, err))
			continue
		}
		requirement, required, err := n.collateralRequirementForExtension(ctx, candidate.OrderID, orderOpen, extension)
		if err != nil {
			refreshErr = errors.Join(refreshErr, err)
			continue
		}
		if !required {
			if err := n.dismissCollateralCredentialRefresh(candidate); err != nil {
				refreshErr = errors.Join(refreshErr, err)
			}
			continue
		}
		expiresAt := now.Add(collateralCredentialRequestedTTL)
		if !candidate.AccountExpiresAt.IsZero() && expiresAt.After(candidate.AccountExpiresAt) {
			expiresAt = candidate.AccountExpiresAt
		}
		if !expiresAt.After(now) {
			continue
		}
		if err := service.RequestOrderExtensionCollateralCredential(ctx, contracts.RequestOrderExtensionCollateralCredentialRequest{
			OrderID: candidate.OrderID, Extension: extension, Requirement: requirement, ExpiresAt: expiresAt,
		}); err != nil {
			refreshErr = errors.Join(refreshErr, fmt.Errorf("request collateral credential refresh for order %s: %w", candidate.OrderID, err))
		}
	}
	return refreshErr
}

func (n *MobazhaNode) dismissCollateralCredentialRefresh(candidate corecollateral.CredentialRefreshCandidate) error {
	return n.db.Update(func(tx database.Tx) error {
		return corecollateral.DismissCredentialRefreshTx(tx, candidate)
	})
}

func (n *MobazhaNode) handleCollateralCredentialRequest(from peer.ID, message *pb.Message) error {
	if message == nil || message.MessageType != pb.Message_COLLATERAL_CREDENTIAL_REQUEST || strings.TrimSpace(message.MessageID) == "" {
		return fmt.Errorf("message is not a valid collateral credential request")
	}
	if n == nil || n.db == nil || n.messenger == nil {
		return fmt.Errorf("collateral credential receiver is unavailable")
	}
	if n.isDuplicate(message) {
		n.sendAckMessage(message.MessageID, from)
		return nil
	}
	wire := new(pb.CollateralCredentialRequest)
	if message.Payload == nil {
		return fmt.Errorf("collateral credential request payload is required")
	}
	if err := message.Payload.UnmarshalTo(wire); err != nil {
		return err
	}
	extension, requirement, err := decodeCollateralCredentialBinding(wire.ExtensionJson, wire.RequirementJson)
	if err != nil {
		return err
	}
	service := n.CollateralAllocation()
	if service == nil {
		return fmt.Errorf("collateral allocation service is unavailable")
	}
	issueRequest := contracts.IssueOrderExtensionCollateralCredentialRequest{
		TenantID: collateralTransportTenantID(n.db), OrderID: wire.OrderID,
		Extension: extension, Requirement: requirement, AudiencePeerID: from.String(),
		ExpiresAt: time.Unix(wire.ExpiresAtUnix, 0).UTC(),
	}
	authority, ok := service.(*collateralAllocationService)
	if !ok {
		return fmt.Errorf("collateral allocation authority implementation is unavailable")
	}
	transportCtx := collateralTransportContext(n)
	if err := authority.ensureOrderExtensionCollateralAllocation(transportCtx, issueRequest); err != nil {
		return fmt.Errorf("allocate collateral for credential request: %w", err)
	}
	credential, err := service.IssueOrderExtensionCollateralCredential(transportCtx, issueRequest)
	if err != nil {
		return err
	}
	credentialJSON, err := json.Marshal(credential)
	if err != nil {
		return err
	}
	responsePayload, err := anypb.New(&pb.CollateralCredentialResponse{
		RequestMessageID: message.MessageID, OrderID: wire.OrderID,
		ExtensionJson: wire.ExtensionJson, RequirementJson: wire.RequirementJson, CredentialJson: credentialJSON,
	})
	if err != nil {
		return err
	}
	response := newMessageWithID()
	response.MessageType = pb.Message_COLLATERAL_CREDENTIAL_RESPONSE
	response.Payload = responsePayload
	if err := n.db.Update(func(tx database.Tx) error {
		return n.messenger.ReliablySendMessage(tx, from, response, nil)
	}); err != nil {
		return fmt.Errorf("persist collateral credential response: %w", err)
	}
	n.sendAckMessage(message.MessageID, from)
	return nil
}

func (n *MobazhaNode) handleCollateralCredentialResponse(from peer.ID, message *pb.Message) error {
	if message == nil || message.MessageType != pb.Message_COLLATERAL_CREDENTIAL_RESPONSE || strings.TrimSpace(message.MessageID) == "" {
		return fmt.Errorf("message is not a valid collateral credential response")
	}
	if n == nil || n.db == nil || n.messenger == nil {
		return fmt.Errorf("collateral credential receiver is unavailable")
	}
	if n.isDuplicate(message) {
		n.sendAckMessage(message.MessageID, from)
		return nil
	}
	wire := new(pb.CollateralCredentialResponse)
	if message.Payload == nil {
		return fmt.Errorf("collateral credential response payload is required")
	}
	if err := message.Payload.UnmarshalTo(wire); err != nil {
		return err
	}
	if strings.TrimSpace(wire.RequestMessageID) == "" {
		return fmt.Errorf("collateral credential response request ID is required")
	}
	extension, requirement, err := decodeCollateralCredentialBinding(wire.ExtensionJson, wire.RequirementJson)
	if err != nil {
		return err
	}
	var credential pkgcollateral.AllocationCredential
	if err := json.Unmarshal(wire.CredentialJson, &credential); err != nil {
		return fmt.Errorf("decode collateral allocation credential: %w", err)
	}
	if credential.IssuerPeerID != from.String() {
		return fmt.Errorf("collateral credential response sender does not match issuer")
	}
	service := n.CollateralAllocation()
	if service == nil {
		return fmt.Errorf("collateral allocation service is unavailable")
	}
	if err := service.ImportOrderExtensionCollateralCredential(collateralTransportContext(n), contracts.ImportOrderExtensionCollateralCredentialRequest{
		OrderID: wire.OrderID, Extension: extension, Requirement: requirement, Credential: credential,
	}); err != nil {
		return err
	}
	n.sendAckMessage(message.MessageID, from)
	return nil
}

func decodeCollateralCredentialBinding(extensionJSON, requirementJSON []byte) (extensions.OrderExtension, extensions.CollateralRequirement, error) {
	var extension extensions.OrderExtension
	if err := json.Unmarshal(extensionJSON, &extension); err != nil {
		return extensions.OrderExtension{}, extensions.CollateralRequirement{}, fmt.Errorf("decode collateral credential extension: %w", err)
	}
	var requirement extensions.CollateralRequirement
	if err := json.Unmarshal(requirementJSON, &requirement); err != nil {
		return extensions.OrderExtension{}, extensions.CollateralRequirement{}, fmt.Errorf("decode collateral credential requirement: %w", err)
	}
	return extension, requirement, nil
}

func collateralTransportTenantID(db database.Database) string {
	type tenantIDGetter interface {
		TenantID() string
	}
	if scoped, ok := db.(tenantIDGetter); ok && strings.TrimSpace(scoped.TenantID()) != "" {
		return scoped.TenantID()
	}
	return database.StandaloneTenantID
}

func collateralTransportContext(n *MobazhaNode) context.Context {
	if n != nil && n.nodeCtx != nil {
		return n.nodeCtx
	}
	return context.Background()
}
