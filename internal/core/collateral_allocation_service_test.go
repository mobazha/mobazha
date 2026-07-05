// SPDX-License-Identifier: MPL-2.0
// Copyright (c) 2026 fengzie and the respective contributors.

package core

import (
	"context"
	"testing"
	"time"

	"github.com/libp2p/go-libp2p/core/peer"
	corecollateral "github.com/mobazha/mobazha/internal/collateral"
	"github.com/mobazha/mobazha/internal/orderextensions"
	"github.com/mobazha/mobazha/internal/repo"
	pkgcollateral "github.com/mobazha/mobazha/pkg/collateral"
	"github.com/mobazha/mobazha/pkg/contracts"
	"github.com/mobazha/mobazha/pkg/database"
	"github.com/mobazha/mobazha/pkg/extensions"
	"github.com/mobazha/mobazha/pkg/models"
	netpb "github.com/mobazha/mobazha/pkg/net/mbzpb"
	orderpb "github.com/mobazha/mobazha/pkg/orders/mbzpb"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

func TestCollateralAllocationServiceAllocatesAndBindsInOneCoreTransaction(t *testing.T) {
	db, err := repo.MockDB()
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, db.Close()) })
	require.NoError(t, db.Update(func(tx database.Tx) error {
		for _, model := range []any{
			&models.CollateralAccountRecord{}, &models.CollateralFundingRecord{}, &models.CollateralAllocationRecord{},
			&models.CollateralActionRecord{}, &models.OrderExtensionRecord{}, &models.CollateralOrderExtensionBindingRecord{},
			&models.CollateralAllocationCredentialRecord{}, &models.IncomingMessage{},
			&models.Order{},
		} {
			if err := tx.Migrate(model); err != nil {
				return err
			}
		}
		return nil
	}))
	now := time.Now().UTC().Truncate(time.Second)
	sellerSigner := newMockSigner()
	buyerSigner := newMockSigner()
	sellerPeer, err := peer.Decode(string(sellerSigner.PeerID()))
	require.NoError(t, err)
	buyerPeer, err := peer.Decode(string(buyerSigner.PeerID()))
	require.NoError(t, err)
	assetID := "crypto:eip155:56:erc20:0x55d398326f99059fF775485246999027B3197955"
	open := pkgcollateral.OpenRequest{
		TenantID: database.StandaloneTenantID, ProviderID: "io.mobazha.collectibles", ResourceID: "srcdep-1",
		PrincipalID: string(sellerSigner.PeerID()), AssetID: assetID, RequiredAmount: "100",
		PolicyID: "io.mobazha.collectibles.source-custody", PolicyVersion: "1",
		IdempotencyKey: "open-seller-authority", ExpiresAt: now.Add(24 * time.Hour),
	}
	var account pkgcollateral.Account
	require.NoError(t, db.Update(func(tx database.Tx) error {
		account, err = corecollateral.OpenTx(tx, open, now)
		if err != nil {
			return err
		}
		account, err = corecollateral.RecordFundingTx(tx, pkgcollateral.FundingObservation{
			TenantID: open.TenantID, CollateralID: account.CollateralID, AssetID: assetID,
			FundedAmount: "100", FundingReference: "funding-seller-authority",
			ExpectedRevision: account.Revision, IdempotencyKey: "fund-seller-authority", ObservedAt: now,
		}, now)
		return err
	}))
	extension, err := extensions.NewOrderExtension(
		"order-seller-authority", open.ProviderID, "source-custody", "v1", open.ResourceID, map[string]string{"mode": "M2"},
	)
	require.NoError(t, err)
	require.NoError(t, db.Update(func(tx database.Tx) error {
		if err := orderextensions.PersistTx(tx, "order-seller-authority", extension); err != nil {
			return err
		}
		orderOpen, err := protojson.Marshal(&orderpb.OrderOpen{BuyerID: &orderpb.ID{PeerID: string(buyerSigner.PeerID())}})
		if err != nil {
			return err
		}
		return tx.Save(&models.Order{
			ID: models.OrderID("order-seller-authority"), MyRole: string(models.RoleVendor), SerializedOrderOpen: orderOpen,
		})
	}))
	request := contracts.AllocateOrderExtensionCollateralRequest{
		TenantID: open.TenantID, CollateralID: account.CollateralID, OrderID: "order-seller-authority",
		ExpectedCollateralRevision: account.Revision, IdempotencyKey: "allocate-seller-authority",
		Extension: extension,
		Requirement: extensions.CollateralRequirement{
			ProviderID: open.ProviderID, ResourceID: open.ResourceID, PrincipalID: open.PrincipalID,
			AssetID: assetID, Amount: "25", PolicyID: open.PolicyID, PolicyVersion: open.PolicyVersion,
		},
	}
	clock := now
	service := &collateralAllocationService{db: db, signer: sellerSigner, now: func() time.Time { return clock }}
	first, err := service.AllocateOrderExtensionCollateral(context.Background(), request)
	require.NoError(t, err)
	wrongPolicy := request
	wrongPolicy.IdempotencyKey = "allocate-wrong-policy"
	wrongPolicy.ExpectedCollateralRevision = first.CollateralAllocation.CollateralRevision
	wrongPolicy.Requirement.PolicyVersion = "2"
	_, err = service.AllocateOrderExtensionCollateral(context.Background(), wrongPolicy)
	require.ErrorContains(t, err, "does not match the declared requirement")
	second, err := service.AllocateOrderExtensionCollateral(context.Background(), request)
	require.NoError(t, err)
	require.Equal(t, first, second)
	require.NotNil(t, first.CollateralAllocation)
	require.Equal(t, "25", first.CollateralAllocation.Amount)

	require.NoError(t, db.View(func(tx database.Tx) error {
		stored, err := corecollateral.AccountByIDTx(tx, account.CollateralID)
		if err != nil {
			return err
		}
		require.Equal(t, "75", stored.AvailableAmount)
		return corecollateral.AdmitPersistedOrderExtensionsV2Tx(tx, request.OrderID, now)
	}))

	credential, err := service.IssueOrderExtensionCollateralCredential(context.Background(), contracts.IssueOrderExtensionCollateralCredentialRequest{
		TenantID: open.TenantID, OrderID: request.OrderID, Extension: extension, Requirement: request.Requirement,
		AudiencePeerID: string(buyerSigner.PeerID()), ExpiresAt: now.Add(5 * time.Minute),
	})
	require.NoError(t, err)
	require.NoError(t, credential.Verify(string(buyerSigner.PeerID()), now))
	differentExtension, err := extensions.NewOrderExtension(
		request.OrderID, open.ProviderID, "source-custody", "v1", open.ResourceID, map[string]string{"mode": "M3"},
	)
	require.NoError(t, err)
	require.Equal(t, extension.ExtensionID, differentExtension.ExtensionID)
	_, err = service.IssueOrderExtensionCollateralCredential(context.Background(), contracts.IssueOrderExtensionCollateralCredentialRequest{
		TenantID: open.TenantID, OrderID: request.OrderID, Extension: differentExtension, Requirement: request.Requirement,
		AudiencePeerID: string(buyerSigner.PeerID()), ExpiresAt: now.Add(5 * time.Minute),
	})
	require.ErrorContains(t, err, "revision is stale")
	clock = now.Add(time.Second)
	refreshedCredential, err := service.IssueOrderExtensionCollateralCredential(context.Background(), contracts.IssueOrderExtensionCollateralCredentialRequest{
		TenantID: open.TenantID, OrderID: request.OrderID, Extension: extension, Requirement: request.Requirement,
		AudiencePeerID: string(buyerSigner.PeerID()), ExpiresAt: clock.Add(5 * time.Minute),
	})
	require.NoError(t, err)
	require.NotEqual(t, credential.CredentialID, refreshedCredential.CredentialID)
	credential = refreshedCredential

	buyerDB, err := repo.MockDB()
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, buyerDB.Close()) })
	require.NoError(t, buyerDB.Update(func(tx database.Tx) error {
		for _, model := range []any{
			&models.OrderExtensionRecord{}, &models.CollateralOrderExtensionBindingRecord{},
			&models.CollateralAllocationCredentialRecord{}, &models.CollateralCredentialRefreshRecord{},
			&models.IncomingMessage{}, &models.Order{},
		} {
			if err := tx.Migrate(model); err != nil {
				return err
			}
		}
		if err := orderextensions.PersistTx(tx, request.OrderID, extension); err != nil {
			return err
		}
		orderOpen, err := protojson.Marshal(&orderpb.OrderOpen{
			BuyerID:  &orderpb.ID{PeerID: string(buyerSigner.PeerID())},
			Listings: []*orderpb.SignedListing{{Listing: &orderpb.Listing{VendorID: &orderpb.ID{PeerID: string(sellerSigner.PeerID())}}}},
		})
		if err != nil {
			return err
		}
		return tx.Save(&models.Order{
			ID: models.OrderID(request.OrderID), MyRole: string(models.RoleBuyer), SerializedOrderOpen: orderOpen,
		})
	}))
	buyerMessenger := &mockMessenger{}
	buyerService := &collateralAllocationService{db: buyerDB, signer: buyerSigner, messenger: buyerMessenger, now: func() time.Time { return clock }}
	require.NoError(t, buyerService.ImportOrderExtensionCollateralCredential(context.Background(), contracts.ImportOrderExtensionCollateralCredentialRequest{
		OrderID: request.OrderID, Extension: extension, Requirement: request.Requirement, Credential: credential,
	}))
	require.NoError(t, buyerService.ImportOrderExtensionCollateralCredential(context.Background(), contracts.ImportOrderExtensionCollateralCredentialRequest{
		OrderID: request.OrderID, Extension: extension, Requirement: request.Requirement, Credential: credential,
	}), "credential import is idempotent")
	require.NoError(t, buyerDB.View(func(tx database.Tx) error {
		_, err := corecollateral.AdmitExternalAllocationCredentialTx(
			tx, string(buyerSigner.PeerID()), request.OrderID, extension, request.Requirement, clock,
		)
		return err
	}))
	buyerNode := &MobazhaNode{
		storageFields: storageFields{db: buyerDB},
		cryptoFields:  cryptoFields{signer: buyerSigner},
		networkFields: networkFields{messenger: buyerMessenger},
		orderExtensionFields: orderExtensionFields{orderExtensionModules: mustRegisterOrderExtensionModules(
			t, collateralRequirementTestModule{requirement: request.Requirement},
		)},
	}
	clock = now.Add(2 * time.Second)
	transportRequest := contracts.RequestOrderExtensionCollateralCredentialRequest{
		OrderID: request.OrderID, Extension: extension, Requirement: request.Requirement, ExpiresAt: clock.Add(5 * time.Minute),
	}
	require.NoError(t, buyerService.RequestOrderExtensionCollateralCredential(context.Background(), transportRequest))
	require.NoError(t, buyerService.RequestOrderExtensionCollateralCredential(context.Background(), transportRequest), "request is throttled durably")
	require.Len(t, buyerMessenger.sent, 1)
	require.Equal(t, sellerPeer, buyerMessenger.sent[0].PeerID)
	require.Equal(t, netpb.Message_COLLATERAL_CREDENTIAL_REQUEST, buyerMessenger.sent[0].Msg.MessageType)

	sellerMessenger := &mockMessenger{}
	sellerNode := &MobazhaNode{
		storageFields: storageFields{db: db}, cryptoFields: cryptoFields{signer: sellerSigner},
		networkFields: networkFields{messenger: sellerMessenger},
	}
	require.NoError(t, sellerNode.handleCollateralCredentialRequest(buyerPeer, buyerMessenger.sent[0].Msg))
	require.NoError(t, sellerNode.handleCollateralCredentialRequest(buyerPeer, buyerMessenger.sent[0].Msg), "request retry is ACKed without issuing another response")
	require.Len(t, sellerMessenger.sent, 1)
	require.Equal(t, buyerPeer, sellerMessenger.sent[0].PeerID)
	require.Equal(t, netpb.Message_COLLATERAL_CREDENTIAL_RESPONSE, sellerMessenger.sent[0].Msg.MessageType)
	require.NoError(t, buyerNode.handleCollateralCredentialResponse(sellerPeer, sellerMessenger.sent[0].Msg))
	require.NoError(t, buyerNode.handleCollateralCredentialResponse(sellerPeer, sellerMessenger.sent[0].Msg), "response retry is idempotent")
	buyerMessenger.mu.Lock()
	buyerMessenger.sent = nil
	buyerMessenger.mu.Unlock()
	require.NoError(t, buyerNode.runCollateralCredentialRefreshOnceAt(context.Background(), clock.Add(3*time.Minute)))
	require.NoError(t, buyerNode.runCollateralCredentialRefreshOnceAt(context.Background(), clock.Add(3*time.Minute)), "refresh retry is throttled")
	require.Len(t, buyerMessenger.sent, 1)
	require.Equal(t, sellerPeer, buyerMessenger.sent[0].PeerID)

	spoofed := proto.Clone(sellerMessenger.sent[0].Msg).(*netpb.Message)
	spoofed.MessageID = "spoofed-collateral-credential-response"
	require.ErrorContains(t, buyerNode.handleCollateralCredentialResponse(buyerPeer, spoofed), "sender does not match issuer")
	require.NoError(t, buyerDB.View(func(tx database.Tx) error {
		return buyerNode.admitOrderExtensionCollateralRequirementsTx(
			context.Background(), tx, request.OrderID, nil, []extensions.OrderExtension{extension},
		)
	}))

	tampered := credential
	tampered.Allocation.Amount = "24"
	err = buyerService.ImportOrderExtensionCollateralCredential(context.Background(), contracts.ImportOrderExtensionCollateralCredentialRequest{
		OrderID: request.OrderID, Extension: extension, Requirement: request.Requirement, Credential: tampered,
	})
	require.ErrorContains(t, err, "signature is invalid")
}
