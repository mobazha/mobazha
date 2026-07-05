// SPDX-License-Identifier: MPL-2.0
// Copyright (c) 2026 fengzie and the respective contributors.

package core

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/libp2p/go-libp2p/core/peer"
	corecollateral "github.com/mobazha/mobazha/internal/collateral"
	"github.com/mobazha/mobazha/internal/orderextensions"
	pkgcollateral "github.com/mobazha/mobazha/pkg/collateral"
	"github.com/mobazha/mobazha/pkg/contracts"
	"github.com/mobazha/mobazha/pkg/database"
	"github.com/mobazha/mobazha/pkg/extensions"
	"github.com/mobazha/mobazha/pkg/models"
	pb "github.com/mobazha/mobazha/pkg/net/mbzpb"
	"google.golang.org/protobuf/types/known/anypb"
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

	var seller peer.ID
	if err := s.db.View(func(tx database.Tx) error {
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
		return nil
	}); err != nil {
		return err
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
	if err := s.db.Update(func(tx database.Tx) error {
		return s.messenger.ReliablySendMessage(tx, seller, message, nil)
	}); err != nil {
		return fmt.Errorf("persist collateral credential request: %w", err)
	}
	return nil
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
	credential, err := service.IssueOrderExtensionCollateralCredential(collateralTransportContext(n), contracts.IssueOrderExtensionCollateralCredentialRequest{
		TenantID: collateralTransportTenantID(n.db), OrderID: wire.OrderID,
		Extension: extension, Requirement: requirement, AudiencePeerID: from.String(),
		ExpiresAt: time.Unix(wire.ExpiresAtUnix, 0).UTC(),
	})
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
