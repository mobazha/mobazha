// SPDX-License-Identifier: MPL-2.0
// Copyright (c) 2026 fengzie and the respective contributors.

package core

import (
	"context"
	"fmt"
	"testing"
	"time"

	btcec "github.com/btcsuite/btcd/btcec/v2"
	"github.com/mobazha/mobazha/internal/core/paymentintent"
	"github.com/mobazha/mobazha/pkg/contracts"
	"github.com/mobazha/mobazha/pkg/identity"
	"github.com/mobazha/mobazha/pkg/models"
	pb "github.com/mobazha/mobazha/pkg/orders/mbzpb"
	"github.com/mobazha/mobazha/pkg/payment"
	iwallet "github.com/mobazha/mobazha/pkg/wallet-interface"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/encoding/protojson"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type settlementFinalizationWalletAccounts struct {
	destination contracts.ReservedDestination
	requests    []struct {
		railID      string
		role        contracts.WalletAccountRole
		referenceID string
	}
}

type settlementAuthorizationPaymentWatch struct {
	orderID      string
	address      string
	chain        iwallet.ChainType
	scriptPubKey []byte
	calls        int
}

func (w *settlementAuthorizationPaymentWatch) WatchPaymentAddress(
	orderID, address string,
	chain iwallet.ChainType,
	scriptPubKey []byte,
) error {
	w.orderID = orderID
	w.address = address
	w.chain = chain
	w.scriptPubKey = append([]byte(nil), scriptPubKey...)
	w.calls++
	return nil
}

func (*settlementFinalizationWalletAccounts) Capabilities(context.Context, string) (contracts.WalletCapabilities, error) {
	return contracts.WalletCapabilities{Receive: true}, nil
}

func (s *settlementFinalizationWalletAccounts) ReserveAddress(
	_ context.Context,
	railID string,
	role contracts.WalletAccountRole,
	referenceID string,
) (contracts.ReservedDestination, error) {
	s.requests = append(s.requests, struct {
		railID      string
		role        contracts.WalletAccountRole
		referenceID string
	}{railID: railID, role: role, referenceID: referenceID})
	return s.destination, nil
}

func (*settlementFinalizationWalletAccounts) Transfer(context.Context, contracts.WalletTransferRequest) (contracts.WalletTransfer, error) {
	return contracts.WalletTransfer{}, fmt.Errorf("unexpected wallet transfer")
}

func (*settlementFinalizationWalletAccounts) ReconcileTransfers(context.Context) error { return nil }

func TestFinalizeSellerSettlementAuthorization_FreezesDeterministicUTXOTarget(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:seller-authorization-finalize-%d?mode=memory&cache=shared", time.Now().UnixNano())), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(
		&models.PaymentAttempt{}, &models.PaymentRouteBinding{}, &models.PaymentAttemptSettlementOffer{},
	))
	rail, ok := iwallet.CanonicalNativeCoinType(iwallet.ChainBitcoin)
	require.True(t, ok)
	routeIdentity := payment.RouteIdentity{
		ContributionID: "utxo.btc", ModuleID: "utxo", ImplementationGeneration: "v1",
		RailKind: "escrow", NetworkID: "bip122:000000000019d6689c085ae165831e93",
		AssetID: string(rail), ProtocolVersion: "1", StateSchemaVersion: "1",
	}
	attempt, _, err := paymentintent.PrepareCryptoPaymentAttemptDraft(
		db,
		paymentintent.CryptoPaymentAttemptDraftRequest{
			TenantID: "tenant-seller", AttemptID: "attempt-finalize-utxo", OrderID: "order-finalize-utxo",
			AmountAtomic: "100000", RailID: string(rail),
		},
		routeIdentity,
	)
	require.NoError(t, err)

	buyerKeys, err := identity.GenerateKeyPair()
	require.NoError(t, err)
	buyerPeerID, err := identity.PeerIDFromPublicKey(buyerKeys.PubKey)
	require.NoError(t, err)
	sellerKeys, err := identity.GenerateKeyPair()
	require.NoError(t, err)
	sellerPeerID, err := identity.PeerIDFromPublicKey(sellerKeys.PubKey)
	require.NoError(t, err)
	buyerSettlementKey, _ := btcec.PrivKeyFromBytes([]byte("buyer-finalization-settlement-key"))
	sellerSettlementKey, _ := btcec.PrivKeyFromBytes([]byte("seller-finalization-settlement-key"))
	keyRef := contracts.SettlementKeyRef{
		TenantID: attempt.TenantID, RailID: attempt.Currency,
		Purpose: standardOrderSettlementKeyPurpose, ReferenceID: attempt.AuthorizationContextID,
	}
	buyerOffer, err := paymentintent.IssueSettlementKeyOffer(
		t.Context(), contracts.NewKeyPairSigner(buyerKeys, buyerPeerID),
		&buyerStartSettlementSigner{publicKey: buyerSettlementKey.PubKey().SerializeCompressed()},
		keyRef, attempt.OrderID, attempt.AttemptID, models.SettlementParticipantBuyer,
	)
	require.NoError(t, err)
	sellerOffer, err := paymentintent.IssueSettlementKeyOffer(
		t.Context(), contracts.NewKeyPairSigner(sellerKeys, sellerPeerID),
		&buyerStartSettlementSigner{publicKey: sellerSettlementKey.PubKey().SerializeCompressed()},
		keyRef, attempt.OrderID, attempt.AttemptID, models.SettlementParticipantSeller,
	)
	require.NoError(t, err)
	require.NoError(t, paymentintent.StoreCryptoPaymentAttemptSettlementKeyOffer(
		db, attempt.TenantID, attempt.AttemptID, buyerOffer,
	))
	require.NoError(t, paymentintent.StoreCryptoPaymentAttemptSettlementKeyOffer(
		db, attempt.TenantID, attempt.AttemptID, sellerOffer,
	))
	openBytes, err := protojson.Marshal(&pb.OrderOpen{
		Amount: attempt.AmountValue, PricingCoin: "BTC", BuyerID: &pb.ID{PeerID: buyerPeerID.String()},
		Listings: []*pb.SignedListing{{Listing: &pb.Listing{VendorID: &pb.ID{PeerID: sellerPeerID.String()}}}},
	})
	require.NoError(t, err)
	order := &models.Order{
		TenantMixin: models.TenantMixin{TenantID: attempt.TenantID}, ID: models.OrderID(attempt.OrderID),
		MyRole: string(models.RoleVendor), SerializedOrderOpen: openBytes,
	}
	payoutAddress := "bc1qsellerpayout000000000000000000000000000"
	walletAccounts := &settlementFinalizationWalletAccounts{destination: contracts.ReservedDestination{
		Destination: contracts.Destination{RailID: attempt.Currency, Address: payoutAddress, Version: 1},
	}}
	projector := standardOrderUTXOFundingTargetProjector{wallets: testMultiwallet(t, testMasterKey(t))}
	identitySigner := contracts.NewKeyPairSigner(sellerKeys, sellerPeerID)

	first, err := finalizeSellerSettlementAuthorization(
		t.Context(), db, order, identitySigner, walletAccounts, projector, attempt.AttemptID,
	)
	require.NoError(t, err)
	retry, err := finalizeSellerSettlementAuthorization(
		t.Context(), db, order, identitySigner, walletAccounts, projector, attempt.AttemptID,
	)
	require.NoError(t, err)

	require.Equal(t, models.PaymentAttemptFundingTargetReady, first.Attempt.State)
	require.NotEmpty(t, first.Target.Address)
	require.NotEmpty(t, first.Target.RedeemScriptHex)
	require.Equal(t, attempt.AmountValue, first.Target.AmountAtomic)
	require.Equal(t, payoutAddress, first.Terms.SellerAddress)
	require.Equal(t, attempt.AmountValue, first.Terms.SellerGrossBasis)
	require.Equal(t, "0", first.Terms.PlatformReleaseFee.Amount)
	require.Equal(t, "0", first.Terms.BuyerCancellationFee.Amount)
	require.Equal(t, []models.SettlementParticipantRole{
		models.SettlementParticipantBuyer, models.SettlementParticipantSeller,
	}, first.Authorization.RequiredRoles)
	require.Equal(t, first.Attempt.AuthorizationBundleHash, retry.Attempt.AuthorizationBundleHash)
	require.Equal(t, first.Target, retry.Target)
	require.Equal(t, first.Terms, retry.Terms)
	projection, err := projector.project(t.Context(), first.Attempt, first.Route, first.Authorization.Offers)
	require.NoError(t, err)
	require.Equal(t, first.Target, projection.Target)
	require.NotEmpty(t, projection.RedeemScript)
	require.Len(t, walletAccounts.requests, 1, "frozen retry must not allocate another payout address")
	require.Equal(t, contracts.AccountMain, walletAccounts.requests[0].role)
	require.Equal(t, standardOrderSellerPayoutReferencePrefix+attempt.AttemptID, walletAccounts.requests[0].referenceID)

	var retained int64
	require.NoError(t, db.Model(&models.PaymentAttemptSettlementOffer{}).
		Where("tenant_id = ? AND attempt_id = ?", attempt.TenantID, attempt.AttemptID).Count(&retained).Error)
	require.Zero(t, retained)
	storedTerms, err := first.Attempt.GetSettlementTerms()
	require.NoError(t, err)
	require.Equal(t, first.Terms, *storedTerms)
	require.NoError(t, first.Terms.VerifySellerAuthorization(sellerPeerID.String(), first.SellerSignature))

	buyerDB, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:buyer-authorization-adopt-%d?mode=memory&cache=shared", time.Now().UnixNano())), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, buyerDB.AutoMigrate(
		&models.PaymentAttempt{}, &models.PaymentRouteBinding{}, &models.PaymentAttemptSettlementOffer{},
	))
	require.NoError(t, buyerDB.Transaction(func(tx *gorm.DB) error {
		if err := paymentintent.RetainReceivedSettlementKeyOfferInTransaction(tx, attempt.TenantID, buyerOffer); err != nil {
			return err
		}
		return paymentintent.RetainReceivedSettlementKeyOfferInTransaction(tx, attempt.TenantID, sellerOffer)
	}))
	buyerAttempt, _, err := paymentintent.PrepareCryptoPaymentAttemptDraft(
		buyerDB,
		paymentintent.CryptoPaymentAttemptDraftRequest{
			TenantID: attempt.TenantID, AttemptID: attempt.AttemptID, OrderID: attempt.OrderID,
			AmountAtomic: attempt.AmountValue, RailID: attempt.Currency,
		},
		routeIdentity,
	)
	require.NoError(t, err)
	require.Equal(t, attempt.AuthorizationContextID, buyerAttempt.AuthorizationContextID)
	buyerOrder := &models.Order{
		TenantMixin: models.TenantMixin{TenantID: attempt.TenantID}, ID: models.OrderID(attempt.OrderID),
		MyRole: string(models.RoleBuyer), SerializedOrderOpen: openBytes,
	}
	adopted, err := adoptBuyerSettlementAuthorization(
		t.Context(), buyerDB, buyerOrder, contracts.NewKeyPairSigner(buyerKeys, buyerPeerID),
		projector, first.SettlementAuthorization,
	)
	require.NoError(t, err)
	adoptedRetry, err := adoptBuyerSettlementAuthorization(
		t.Context(), buyerDB, buyerOrder, contracts.NewKeyPairSigner(buyerKeys, buyerPeerID),
		projector, first.SettlementAuthorization,
	)
	require.NoError(t, err)
	require.Equal(t, models.PaymentAttemptFundingTargetReady, adopted.Attempt.State)
	require.Equal(t, first.Attempt.AuthorizationBundleHash, adopted.Attempt.AuthorizationBundleHash)
	require.Equal(t, adopted.Attempt.AuthorizationBundleHash, adoptedRetry.Attempt.AuthorizationBundleHash)
	require.Equal(t, first.Target, adopted.Target)

	watch := &settlementAuthorizationPaymentWatch{}
	require.NoError(t, watchFrozenStandardOrderUTXOAttempt(
		t.Context(), buyerDB, projector.wallets, watch, attempt.TenantID, attempt.AttemptID,
	))
	require.Equal(t, 1, watch.calls)
	require.Equal(t, attempt.OrderID, watch.orderID)
	require.Equal(t, first.Target.Address, watch.address)
	require.Equal(t, iwallet.ChainBitcoin, watch.chain)
	require.NotEmpty(t, watch.scriptPubKey)
}

func TestStandardOrderUTXOFundingTargetProjector_ModeratedUsesTwoOfThreeWithTimeout(t *testing.T) {
	rail, ok := iwallet.CanonicalNativeCoinType(iwallet.ChainBitcoin)
	require.True(t, ok)
	contextID, err := models.NewSettlementAuthorizationContextID()
	require.NoError(t, err)
	attempt := models.PaymentAttempt{
		TenantID: "tenant-moderated", AttemptID: "attempt-moderated",
		OrderID: "order-moderated", Currency: string(rail), AmountValue: "100000",
		AuthorizationContextID: contextID, State: models.PaymentAttemptAuthorizationDraft,
	}
	route := models.PaymentRouteBinding{AssetID: string(rail)}
	roles := []models.SettlementParticipantRole{
		models.SettlementParticipantBuyer, models.SettlementParticipantSeller, models.SettlementParticipantModerator,
	}
	identities := make([]contracts.Signer, 0, len(roles))
	settlementKeys := make([]*btcec.PrivateKey, 0, len(roles))
	for _, role := range roles {
		keys, err := identity.GenerateKeyPair()
		require.NoError(t, err)
		peerID, err := identity.PeerIDFromPublicKey(keys.PubKey)
		require.NoError(t, err)
		identities = append(identities, contracts.NewKeyPairSigner(keys, peerID))
		key, _ := btcec.PrivKeyFromBytes([]byte("moderated-" + string(role) + "-settlement-key"))
		settlementKeys = append(settlementKeys, key)
	}
	moderatorPeerID := identities[2].PeerID().String()
	attempt.ExpectedModeratorPeerID = moderatorPeerID
	keyRef := contracts.SettlementKeyRef{
		TenantID: attempt.TenantID, RailID: attempt.Currency,
		Purpose: standardOrderSettlementKeyPurpose, ReferenceID: contextID,
	}
	offers := make([]models.SettlementKeyOffer, 0, len(roles))
	for i, role := range roles {
		payout, fee := "", ""
		if role == models.SettlementParticipantModerator {
			payout, fee = "bc1qmoderatorpayout00000000000000000000000", "100"
		}
		offer, err := paymentintent.IssueSettlementKeyOfferWithScope(
			t.Context(), identities[i],
			&buyerStartSettlementSigner{publicKey: settlementKeys[i].PubKey().SerializeCompressed()},
			keyRef, attempt.OrderID, attempt.AttemptID, role,
			moderatorPeerID, attempt.AmountValue, payout, fee, 72,
		)
		require.NoError(t, err)
		offers = append(offers, offer)
	}
	wallets := testMultiwallet(t, testMasterKey(t))
	projector := standardOrderUTXOFundingTargetProjector{wallets: wallets}
	projection, err := projector.project(t.Context(), attempt, route, offers)
	require.NoError(t, err)
	require.NotEmpty(t, projection.Target.Address)
	require.NotEmpty(t, projection.RedeemScript)

	wallet, err := wallets.WalletForCurrencyCode(string(rail))
	require.NoError(t, err)
	timeoutWallet, ok := wallet.(iwallet.UTXOEscrowWithTimeout)
	require.True(t, ok)
	expectedAddress, expectedScript, err := timeoutWallet.CreateMultisigWithTimeout(
		[]btcec.PublicKey{*settlementKeys[0].PubKey(), *settlementKeys[1].PubKey(), *settlementKeys[2].PubKey()},
		nil, 2, 72*time.Hour, *settlementKeys[1].PubKey(),
	)
	require.NoError(t, err)
	require.Equal(t, expectedAddress.String(), projection.Target.Address)
	require.Equal(t, expectedScript, projection.RedeemScript)

	mutated := append([]models.SettlementKeyOffer(nil), offers...)
	mutated[2].EscrowTimeoutHours = 48
	require.Error(t, mutated[2].Verify(), "timeout mutation must invalidate the moderator identity signature")
}

func TestStandardOrderUTXOFundingTargetProjector_RejectsInvalidOffers(t *testing.T) {
	rail, ok := iwallet.CanonicalNativeCoinType(iwallet.ChainBitcoin)
	require.True(t, ok)
	attempt := models.PaymentAttempt{
		AttemptID: "attempt-invalid-key", OrderID: "order-invalid-key",
		Kind: models.PaymentAttemptKindCryptoFundingTarget, State: models.PaymentAttemptAuthorizationDraft,
		AmountValue: "1000", Currency: string(rail), AuthorizationContextID: stringsOfHex('a', 64),
	}
	projector := standardOrderUTXOFundingTargetProjector{wallets: testMultiwallet(t, testMasterKey(t))}
	_, err := projector.ProjectStandardOrderFundingTarget(
		t.Context(), attempt, models.PaymentRouteBinding{AssetID: attempt.Currency},
		[]models.SettlementKeyOffer{{}, {}},
	)
	require.Error(t, err)
}

func stringsOfHex(value byte, count int) string {
	result := make([]byte, count)
	for i := range result {
		result[i] = value
	}
	return string(result)
}
