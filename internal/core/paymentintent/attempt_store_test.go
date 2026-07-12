// SPDX-License-Identifier: MPL-2.0
// Copyright (c) 2026 fengzie and the respective contributors.

package paymentintent

import (
	"context"
	"crypto/rand"
	"fmt"
	"testing"
	"time"

	libp2pcrypto "github.com/libp2p/go-libp2p/core/crypto"
	peer "github.com/libp2p/go-libp2p/core/peer"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/mobazha/mobazha/pkg/contracts"
	"github.com/mobazha/mobazha/pkg/identity"
	"github.com/mobazha/mobazha/pkg/models"
)

type settlementKeyOfferSignerStub struct {
	publicKey []byte
	signCalls int
	keyRefs   []contracts.SettlementKeyRef
}

func (s *settlementKeyOfferSignerStub) PublicKey(_ context.Context, keyRef contracts.SettlementKeyRef) ([]byte, error) {
	s.keyRefs = append(s.keyRefs, keyRef)
	if s.publicKey != nil {
		return append([]byte(nil), s.publicKey...), nil
	}
	return []byte(keyRef.Purpose), nil
}

func (s *settlementKeyOfferSignerStub) Sign(context.Context, contracts.SettlementSignRequest) ([]byte, error) {
	s.signCalls++
	return nil, nil
}

func cryptoAttemptFixture(t *testing.T) (
	models.PaymentAttempt,
	models.PaymentRouteBinding,
	models.PaymentAttemptSettlementTerms,
	string,
	[]byte,
	models.PaymentAttemptAuthorizationBundle,
	models.PaymentAttemptFundingTarget,
) {
	t.Helper()
	privateKey, publicKey, err := libp2pcrypto.GenerateEd25519Key(rand.Reader)
	require.NoError(t, err)
	sellerPeerID, err := peer.IDFromPublicKey(publicKey)
	require.NoError(t, err)
	buyerPrivateKey, buyerPublicKey, err := libp2pcrypto.GenerateEd25519Key(rand.Reader)
	require.NoError(t, err)
	buyerPeerID, err := peer.IDFromPublicKey(buyerPublicKey)
	require.NoError(t, err)

	attempt := models.PaymentAttempt{
		AttemptID: "attempt-crypto-1", Kind: models.PaymentAttemptKindCryptoFundingTarget,
		PaymentSessionID: "ps_order-1", OrderID: "order-1", AmountValue: "1000",
		Currency: "crypto:eip155:1/native", RouteBindingID: "route-crypto-1",
		IdempotencyKey: "order-1:crypto:eip155:1:native", State: models.PaymentAttemptPendingExternal,
	}
	authorizationContextID, err := models.NewSettlementAuthorizationContextID()
	require.NoError(t, err)
	require.NoError(t, attempt.SetAuthorizationContextID(authorizationContextID))
	route := models.PaymentRouteBinding{
		RouteBindingID: "route-crypto-1", AttemptID: "attempt-crypto-1",
		ContributionID: "builtin-managed-evm", ModuleID: "managed-evm",
		ImplementationGeneration: "safe-v1.4.1", RailKind: "crypto",
		NetworkID: "eip155:1", AssetID: "crypto:eip155:1/native",
		ProtocolVersion: "1", StateSchemaVersion: "1", CreatedAt: time.Now().UTC(),
	}
	terms := models.PaymentAttemptSettlementTerms{
		Version: models.PaymentAttemptSettlementTermsVersion, OrderID: attempt.OrderID,
		AttemptID: attempt.AttemptID, AssetID: route.AssetID, FundingAmount: "1000",
		FundingTargetAddress: "0x3333333333333333333333333333333333333333",
		RouteBindingID:       route.RouteBindingID, BuyerPeerID: buyerPeerID.String(), SellerPeerID: sellerPeerID.String(),
		SellerAddress: "0x1111111111111111111111111111111111111111", SellerGrossBasis: "1000",
		PlatformReleaseFee:   models.PaymentAttemptSettlementFee{Amount: "0"},
		BuyerCancellationFee: models.PaymentAttemptSettlementFee{Amount: "0"},
		Affiliate: &models.PaymentAttemptAffiliateTerm{
			ReferralSessionID: "referral-1", ProgramID: "program-1",
			PromoterPeerID:    "12D3KooWSsoZBMiQjvPctdqckrAGukta3q7kAZS7cQRwfwbet7zG",
			BuyerPeerID:       buyerPeerID.String(),
			CommissionRateBPS: 500, Address: "0x2222222222222222222222222222222222222222",
			Amount: "50", SellerGrossBasis: "1000",
			Lines: []models.PaymentAttemptAffiliateLineTerm{{
				OrderLineID: "order-1:0", NetMerchandiseAtomic: "1000", CommissionAtomic: "50",
			}},
		},
		DisputePolicy: models.DisputeScalingSellerAwardProRataFloor,
	}
	payload, err := terms.SellerSigningPayload()
	require.NoError(t, err)
	signature, err := privateKey.Sign(payload)
	require.NoError(t, err)
	target := models.PaymentAttemptFundingTarget{
		Version: models.PaymentAttemptFundingTargetVersion, AttemptID: attempt.AttemptID,
		Type: models.PaymentAttemptFundingTargetAddress, AssetID: terms.AssetID,
		AmountAtomic: terms.FundingAmount, Address: "0x3333333333333333333333333333333333333333",
	}
	_, targetHash, err := target.CanonicalBytesAndHash()
	require.NoError(t, err)
	offer := models.SettlementKeyOffer{
		Version: models.SettlementAuthorizationVersion, AuthorizationContextID: authorizationContextID,
		OrderID: attempt.OrderID, AttemptID: attempt.AttemptID, ParticipantPeerID: sellerPeerID.String(),
		ParticipantRole: models.SettlementParticipantSeller, RailID: attempt.Currency,
		Purpose: "standard-order-participant:seller", PublicKey: []byte("seller-settlement-public-key"),
	}
	offerPayload, err := offer.SigningPayload()
	require.NoError(t, err)
	offer.Signature, err = privateKey.Sign(offerPayload)
	require.NoError(t, err)
	buyerOffer := models.SettlementKeyOffer{
		Version: models.SettlementAuthorizationVersion, AuthorizationContextID: authorizationContextID,
		OrderID: attempt.OrderID, AttemptID: attempt.AttemptID, ParticipantPeerID: buyerPeerID.String(),
		ParticipantRole: models.SettlementParticipantBuyer, RailID: attempt.Currency,
		Purpose: "standard-order-participant:buyer", PublicKey: []byte("buyer-settlement-public-key"),
	}
	buyerOfferPayload, err := buyerOffer.SigningPayload()
	require.NoError(t, err)
	buyerOffer.Signature, err = buyerPrivateKey.Sign(buyerOfferPayload)
	require.NoError(t, err)
	_, termsHash, err := terms.CanonicalBytesAndHash()
	require.NoError(t, err)
	bundle := models.PaymentAttemptAuthorizationBundle{
		Version: models.SettlementAuthorizationVersion, AuthorizationContextID: authorizationContextID,
		OrderID: attempt.OrderID, AttemptID: attempt.AttemptID, RailID: attempt.Currency,
		SettlementTermsHash: termsHash, FundingTargetHash: targetHash,
		RequiredRoles: []models.SettlementParticipantRole{models.SettlementParticipantBuyer, models.SettlementParticipantSeller},
		Offers:        []models.SettlementKeyOffer{buyerOffer, offer}, SellerTermsSigner: sellerPeerID.String(),
		SellerTermsSignature: signature,
	}
	return attempt, route, terms, sellerPeerID.String(), signature, bundle, target
}

func TestFreezeCryptoPaymentAttempt_PersistsAtomicSnapshotAndAcceptsRetry(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:crypto-attempt-%d?mode=memory&cache=shared", time.Now().UnixNano())), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.PaymentAttempt{}, &models.PaymentRouteBinding{}))
	attempt, route, terms, signer, signature, bundle, target := cryptoAttemptFixture(t)
	attempt, err = CreateCryptoPaymentAttemptDraft(db, attempt, route)
	require.NoError(t, err)

	require.NoError(t, FreezeCryptoPaymentAttempt(db, attempt, route, terms, signer, signature, bundle, target))
	route.CreatedAt = route.CreatedAt.Add(time.Minute)
	require.NoError(t, FreezeCryptoPaymentAttempt(db, attempt, route, terms, signer, signature, bundle, target))

	var stored models.PaymentAttempt
	require.NoError(t, db.Where("tenant_id = ? AND attempt_id = ?", "", attempt.AttemptID).First(&stored).Error)
	require.Equal(t, models.PaymentAttemptFundingTargetReady, stored.State)
	storedTerms, err := stored.GetSettlementTerms()
	require.NoError(t, err)
	require.Equal(t, terms, *storedTerms)
	storedTarget, err := stored.GetFundingTarget()
	require.NoError(t, err)
	require.Equal(t, target, *storedTarget)
}

func TestFreezeCryptoPaymentAttempt_RejectsFrozenMutation(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:crypto-attempt-conflict-%d?mode=memory&cache=shared", time.Now().UnixNano())), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.PaymentAttempt{}, &models.PaymentRouteBinding{}))
	attempt, route, terms, signer, signature, bundle, target := cryptoAttemptFixture(t)
	attempt, err = CreateCryptoPaymentAttemptDraft(db, attempt, route)
	require.NoError(t, err)
	require.NoError(t, FreezeCryptoPaymentAttempt(db, attempt, route, terms, signer, signature, bundle, target))

	target.Address = "0x4444444444444444444444444444444444444444"
	require.ErrorIs(
		t,
		FreezeCryptoPaymentAttempt(db, attempt, route, terms, signer, signature, bundle, target),
		models.ErrPaymentAttemptSettlementTermsConflict,
	)
}

func TestFreezeCryptoPaymentAttempt_DoesNotOverwriteConcurrentWinner(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:crypto-attempt-cas-%d?mode=memory&cache=shared", time.Now().UnixNano())), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.PaymentAttempt{}, &models.PaymentRouteBinding{}))
	attempt, route, terms, signer, signature, bundle, target := cryptoAttemptFixture(t)
	attempt, err = CreateCryptoPaymentAttemptDraft(db, attempt, route)
	require.NoError(t, err)

	fired := false
	require.NoError(t, db.Callback().Update().Before("gorm:update").Register("test:freeze-concurrent-winner", func(tx *gorm.DB) {
		if fired || tx.Statement.Table != (models.PaymentAttempt{}).TableName() {
			return
		}
		fired = true
		require.NoError(t, tx.Exec(
			"UPDATE payment_attempts SET state = ?, last_error = ? WHERE tenant_id = ? AND attempt_id = ?",
			models.PaymentAttemptFundingTargetReady, "concurrent winner", attempt.TenantID, attempt.AttemptID,
		).Error)
	}))
	t.Cleanup(func() { db.Callback().Update().Remove("test:freeze-concurrent-winner") })

	err = FreezeCryptoPaymentAttempt(db, attempt, route, terms, signer, signature, bundle, target)
	require.ErrorIs(t, err, models.ErrPaymentAttemptSettlementTermsConflict)
	require.True(t, fired)

	var stored models.PaymentAttempt
	require.NoError(t, db.Where("tenant_id = ? AND attempt_id = ?", attempt.TenantID, attempt.AttemptID).First(&stored).Error)
	// The test hook runs in the same transaction, so the intentional conflict
	// rolls its simulated competing state claim back. The important property is
	// that the conditional update did not write the frozen snapshot.
	require.Equal(t, models.PaymentAttemptAuthorizationDraft, stored.State)
	require.Empty(t, stored.LastError)
	require.Empty(t, stored.FundingTarget)
}

func TestFreezeCryptoPaymentAttempt_RejectsTargetBeforeValidSellerAuthorization(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:crypto-attempt-auth-%d?mode=memory&cache=shared", time.Now().UnixNano())), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.PaymentAttempt{}, &models.PaymentRouteBinding{}))
	attempt, route, terms, signer, _, bundle, target := cryptoAttemptFixture(t)
	attempt, err = CreateCryptoPaymentAttemptDraft(db, attempt, route)
	require.NoError(t, err)

	require.Error(t, FreezeCryptoPaymentAttempt(db, attempt, route, terms, signer, []byte("invalid"), bundle, target))
	var count int64
	require.NoError(t, db.Model(&models.PaymentAttempt{}).Count(&count).Error)
	require.Equal(t, int64(1), count)
	var stored models.PaymentAttempt
	require.NoError(t, db.Where("tenant_id = ? AND attempt_id = ?", attempt.TenantID, attempt.AttemptID).First(&stored).Error)
	require.Equal(t, models.PaymentAttemptAuthorizationDraft, stored.State)
}

func TestFreezeCryptoPaymentAttempt_RequiresPersistedDraft(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:crypto-attempt-draft-%d?mode=memory&cache=shared", time.Now().UnixNano())), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.PaymentAttempt{}, &models.PaymentRouteBinding{}))
	attempt, route, terms, signer, signature, bundle, target := cryptoAttemptFixture(t)

	require.ErrorContains(
		t,
		FreezeCryptoPaymentAttempt(db, attempt, route, terms, signer, signature, bundle, target),
		"authorization draft is required",
	)
}

func TestCreateCryptoPaymentAttemptDraft_ReusesDurableContextOnRetry(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:crypto-attempt-context-%d?mode=memory&cache=shared", time.Now().UnixNano())), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.PaymentAttempt{}, &models.PaymentRouteBinding{}))
	attempt, route, _, _, _, _, _ := cryptoAttemptFixture(t)
	attempt.AuthorizationContextID = ""

	first, err := CreateCryptoPaymentAttemptDraft(db, attempt, route)
	require.NoError(t, err)
	require.NotEmpty(t, first.AuthorizationContextID)
	retry, err := CreateCryptoPaymentAttemptDraft(db, attempt, route)
	require.NoError(t, err)
	require.Equal(t, first.AuthorizationContextID, retry.AuthorizationContextID)
	require.Equal(t, models.PaymentAttemptAuthorizationDraft, retry.State)
}

func TestNewSettlementSignRequest_UsesOnlyFrozenAttemptBindings(t *testing.T) {
	attempt, _, terms, signer, signature, bundle, target := cryptoAttemptFixture(t)
	attempt.TenantID = "tenant-a"
	require.NoError(t, attempt.SetSettlementTerms(terms))
	require.NoError(t, attempt.SetSellerTermsAuthorization(signer, signature))
	require.NoError(t, attempt.SetAuthorizationBundle(bundle))
	require.NoError(t, attempt.SetFundingTarget(target))

	request, err := NewSettlementSignRequest(
		attempt,
		contracts.SettlementKeyRef{TenantID: attempt.TenantID, RailID: attempt.Currency, Purpose: "standard-order-participant:seller", ReferenceID: attempt.AuthorizationContextID},
		models.SettlementParticipantSeller,
		"mobazha:settlement:eip155:1:v1", "release", 7, []byte("canonical transaction plan"),
	)
	require.NoError(t, err)
	require.NoError(t, request.Validate())
	require.Equal(t, attempt.OrderID, request.OrderID)
	require.Equal(t, attempt.AttemptID, request.AttemptID)
	require.Equal(t, attempt.SettlementTermsHash, request.TermsHash)

	_, err = NewSettlementSignRequest(
		attempt,
		contracts.SettlementKeyRef{TenantID: attempt.TenantID, RailID: attempt.Currency, Purpose: "standard-order-participant:buyer", ReferenceID: attempt.AuthorizationContextID},
		models.SettlementParticipantSeller,
		"mobazha:settlement:eip155:1:v1", "release", 7, []byte("canonical transaction plan"),
	)
	require.ErrorContains(t, err, "purpose")
}

func TestIssueSettlementKeyOffer_UsesOpaqueSettlementPublicKey(t *testing.T) {
	keyPair, err := identity.GenerateKeyPair()
	require.NoError(t, err)
	peerID, err := identity.PeerIDFromPublicKey(keyPair.PubKey)
	require.NoError(t, err)
	identitySigner := contracts.NewKeyPairSigner(keyPair, peerID)
	contextID, err := models.NewSettlementAuthorizationContextID()
	require.NoError(t, err)
	settlementSigner := &settlementKeyOfferSignerStub{publicKey: []byte("attempt-settlement-public-key")}

	offer, err := IssueSettlementKeyOffer(
		t.Context(), identitySigner, settlementSigner,
		contracts.SettlementKeyRef{
			TenantID: "tenant-a", RailID: "crypto:eip155:1:native",
			Purpose: "standard-order-participant", ReferenceID: contextID,
		},
		"order-1", "attempt-1", models.SettlementParticipantSeller,
	)
	require.NoError(t, err)
	require.NoError(t, offer.Verify())
	require.Equal(t, settlementSigner.publicKey, offer.PublicKey)
	require.Zero(t, settlementSigner.signCalls)
	require.Len(t, settlementSigner.keyRefs, 1)
	require.Equal(t, "standard-order-participant:seller", settlementSigner.keyRefs[0].Purpose)
}

func TestIssueSettlementKeyOffer_RoleSeparatesSignerKeyReference(t *testing.T) {
	keyPair, err := identity.GenerateKeyPair()
	require.NoError(t, err)
	peerID, err := identity.PeerIDFromPublicKey(keyPair.PubKey)
	require.NoError(t, err)
	identitySigner := contracts.NewKeyPairSigner(keyPair, peerID)
	contextID, err := models.NewSettlementAuthorizationContextID()
	require.NoError(t, err)
	settlementSigner := &settlementKeyOfferSignerStub{}
	keyRef := contracts.SettlementKeyRef{
		TenantID: "tenant-a", RailID: "crypto:eip155:1:native",
		Purpose: "standard-order-participant", ReferenceID: contextID,
	}

	buyer, err := IssueSettlementKeyOffer(t.Context(), identitySigner, settlementSigner, keyRef, "order-1", "attempt-1", models.SettlementParticipantBuyer)
	require.NoError(t, err)
	seller, err := IssueSettlementKeyOffer(t.Context(), identitySigner, settlementSigner, keyRef, "order-1", "attempt-1", models.SettlementParticipantSeller)
	require.NoError(t, err)
	require.Equal(t, "standard-order-participant:buyer", buyer.Purpose)
	require.Equal(t, "standard-order-participant:seller", seller.Purpose)
	require.NotEqual(t, buyer.PublicKey, seller.PublicKey)
}
