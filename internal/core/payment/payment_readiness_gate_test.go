package payment

import (
	"context"
	"crypto/rand"
	"errors"
	"testing"
	"time"

	libp2pcrypto "github.com/libp2p/go-libp2p/core/crypto"
	peer "github.com/libp2p/go-libp2p/core/peer"
	"github.com/mobazha/mobazha/internal/repo"
	"github.com/mobazha/mobazha/pkg/contracts"
	"github.com/mobazha/mobazha/pkg/database"
	"github.com/mobazha/mobazha/pkg/models"
	pkpayment "github.com/mobazha/mobazha/pkg/payment"
	"github.com/stretchr/testify/require"
)

type rejectingProvisioningPolicy struct{ err error }

func (p rejectingProvisioningPolicy) AuthorizeSessionProvisioning(context.Context, SessionProvisioningPolicyInput) error {
	return p.err
}

type provisioningPolicyFunc func(context.Context, SessionProvisioningPolicyInput) error

func (f provisioningPolicyFunc) AuthorizeSessionProvisioning(ctx context.Context, input SessionProvisioningPolicyInput) error {
	return f(ctx, input)
}

func TestPaymentSessionProjector_GatesBuyerFundingTarget(t *testing.T) {
	p := NewPaymentSessionProjector(nil)
	order := &models.Order{
		ID:             "QmBuyerNotReady",
		MyRole:         string(models.RoleBuyer),
		PaymentAddress: "0xmanagedescrow",
	}
	require.NoError(t, order.SetPendingManagedEscrowInfo(&models.PendingManagedEscrowInfo{
		Coin:    "crypto:ethereum:mainnet:native",
		Address: "0xmanagedescrow",
	}))

	session, err := p.Project(&projectOrderInput{order: order})
	require.NoError(t, err)
	require.Equal(t, pkpayment.PaymentReadinessAwaitingSellerReceipt, session.PaymentReadiness.Status)
	require.Empty(t, session.FundingTarget.Address)
	require.Empty(t, session.FundingTarget.QRPayload)
}

func TestPaymentSessionProjector_BuyerReadyExposesFundingTarget(t *testing.T) {
	p := NewPaymentSessionProjector(nil)
	readyAt := time.Now()
	order := &models.Order{
		ID:             "QmBuyerReady",
		MyRole:         string(models.RoleBuyer),
		PaymentAddress: "0xmanagedescrow",
		PaymentReadyAt: &readyAt,
	}
	require.NoError(t, order.SetPendingManagedEscrowInfo(&models.PendingManagedEscrowInfo{
		Coin:    "crypto:ethereum:mainnet:native",
		Address: "0xmanagedescrow",
	}))

	session, err := p.Project(&projectOrderInput{order: order})
	require.NoError(t, err)
	require.Equal(t, pkpayment.PaymentReadinessReadyToPay, session.PaymentReadiness.Status)
	require.Equal(t, "0xmanagedescrow", session.FundingTarget.Address)
}

func TestPaymentSessionProjector_AuthorizationDraftRemainsNonActionable(t *testing.T) {
	p := NewPaymentSessionProjector(nil)
	readyAt := time.Now()
	order := &models.Order{
		ID:             "QmAuthorizationDraft",
		MyRole:         string(models.RoleBuyer),
		PaymentReadyAt: &readyAt,
	}
	attempt := &models.PaymentAttempt{
		AttemptID: "attempt-draft",
		Kind:      models.PaymentAttemptKindCryptoFundingTarget,
		State:     models.PaymentAttemptAuthorizationDraft,
	}

	session, err := p.Project(&projectOrderInput{order: order, cryptoAttempt: attempt})
	require.NoError(t, err)
	require.Equal(t, pkpayment.PaymentReadinessAwaitingSellerReceipt, session.PaymentReadiness.Status)
	require.Nil(t, session.PaymentReadiness.ReadyAt)
	require.Empty(t, session.FundingTarget.Address)
}

func TestPaymentSessionProjector_FrozenAttemptTargetIsAuthoritative(t *testing.T) {
	p := NewPaymentSessionProjector(nil)
	readyAt := time.Now()
	order := &models.Order{
		ID:             "QmFrozenAttemptTarget",
		MyRole:         string(models.RoleBuyer),
		PaymentReadyAt: &readyAt,
	}
	target := &models.PaymentAttemptFundingTarget{
		Version: models.PaymentAttemptFundingTargetVersion, AttemptID: "attempt-frozen",
		Type: models.PaymentAttemptFundingTargetAddress, AssetID: "crypto:eip155:1/native",
		AmountAtomic: "1000000000000000000", Address: "0x1111111111111111111111111111111111111111",
		MemoOrTag: "attempt-memo",
	}

	session, err := p.Project(&projectOrderInput{order: order, frozenTarget: target})
	require.NoError(t, err)
	require.Equal(t, target.AssetID, session.PaymentCoin)
	require.Equal(t, target.Address, session.FundingTarget.Address)
	require.Equal(t, target.MemoOrTag, session.FundingTarget.MemoOrTag)
	require.NotEmpty(t, session.FundingTarget.Amount)
}

func TestPaymentSessionProjector_ExposesFrozenEscrowTimeout(t *testing.T) {
	base := frozenPaymentAttemptForProjectionTest(t, "QmFrozenAttemptTimeout")
	terms, err := base.GetSettlementTerms()
	require.NoError(t, err)
	require.NotNil(t, terms)
	terms.EscrowTimeoutHours = 72
	target, err := base.GetFundingTarget()
	require.NoError(t, err)
	require.NotNil(t, target)
	attempt := base
	attempt.State = models.PaymentAttemptFundingTargetReady
	attempt.SettlementTerms = nil
	attempt.SettlementTermsHash = ""
	require.NoError(t, attempt.SetSettlementTerms(*terms))

	session, err := NewPaymentSessionProjector(nil).Project(&projectOrderInput{
		order: &models.Order{ID: "QmFrozenAttemptTimeout"}, cryptoAttempt: &attempt, frozenTarget: target,
	})
	require.NoError(t, err)
	require.Equal(t, uint32(72), session.EscrowTimeoutHours)
}

func TestPaymentSessionProjector_RejectsMutableAddressConflictWithFrozenAttempt(t *testing.T) {
	p := NewPaymentSessionProjector(nil)
	order := &models.Order{
		ID:             "QmFrozenAttemptConflict",
		PaymentAddress: "0x2222222222222222222222222222222222222222",
	}
	target := &models.PaymentAttemptFundingTarget{
		Version: models.PaymentAttemptFundingTargetVersion, AttemptID: "attempt-frozen",
		Type: models.PaymentAttemptFundingTargetAddress, AssetID: "crypto:eip155:1/native",
		AmountAtomic: "1", Address: "0x1111111111111111111111111111111111111111",
	}

	_, err := p.Project(&projectOrderInput{order: order, frozenTarget: target})
	require.ErrorContains(t, err, "target conflicts with order payment address")
}

func TestPaymentSessionProjector_RejectsUnverifiableActionableAttempt(t *testing.T) {
	db, err := repo.MockDB()
	require.NoError(t, err)
	defer db.Close()

	const orderID = "QmInvalidFrozenAttempt"
	require.NoError(t, db.Update(func(tx database.Tx) error {
		if err := tx.Migrate(&models.Order{}); err != nil {
			return err
		}
		if err := tx.Migrate(&models.PaymentAttempt{}); err != nil {
			return err
		}
		if err := tx.Save(&models.Order{ID: models.OrderID(orderID), Open: true}); err != nil {
			return err
		}
		return tx.Create(&models.PaymentAttempt{
			AttemptID: "attempt-invalid", Kind: models.PaymentAttemptKindCryptoFundingTarget,
			PaymentSessionID: "ps_" + orderID, OrderID: orderID, RouteBindingID: "route-invalid",
			IdempotencyKey: "attempt-invalid", State: models.PaymentAttemptFundingTargetReady,
		})
	}))

	_, err = NewPaymentSessionProjector(db).fetchProjectInput(orderID)
	require.ErrorContains(t, err, "actionable crypto attempt has no funding target")
}

func TestPaymentSessionProjector_LoadsVerifiedFrozenAttemptTarget(t *testing.T) {
	db, err := repo.MockDB()
	require.NoError(t, err)
	defer db.Close()

	const orderID = "QmVerifiedFrozenAttempt"
	readyAt := time.Now()
	attempt := frozenPaymentAttemptForProjectionTest(t, orderID)
	require.NoError(t, db.Update(func(tx database.Tx) error {
		if err := tx.Migrate(&models.Order{}); err != nil {
			return err
		}
		if err := tx.Migrate(&models.PaymentAttempt{}); err != nil {
			return err
		}
		if err := tx.Save(&models.Order{
			ID: models.OrderID(orderID), MyRole: string(models.RoleBuyer), Open: true, PaymentReadyAt: &readyAt,
		}); err != nil {
			return err
		}
		return tx.Create(&attempt)
	}))

	projector := NewPaymentSessionProjector(db)
	input, err := projector.fetchProjectInput(orderID)
	require.NoError(t, err)
	require.NotNil(t, input.cryptoAttempt)
	require.NotNil(t, input.frozenTarget)
	session, err := projector.Project(input)
	require.NoError(t, err)
	require.Equal(t, attempt.Currency, session.PaymentCoin)
	require.Equal(t, "0x1111111111111111111111111111111111111111", session.FundingTarget.Address)
}

func frozenPaymentAttemptForProjectionTest(t *testing.T, orderID string) models.PaymentAttempt {
	t.Helper()
	sellerPrivateKey, sellerPublicKey, err := libp2pcrypto.GenerateEd25519Key(rand.Reader)
	require.NoError(t, err)
	sellerPeerID, err := peer.IDFromPublicKey(sellerPublicKey)
	require.NoError(t, err)
	buyerPrivateKey, buyerPublicKey, err := libp2pcrypto.GenerateEd25519Key(rand.Reader)
	require.NoError(t, err)
	buyerPeerID, err := peer.IDFromPublicKey(buyerPublicKey)
	require.NoError(t, err)
	contextID, err := models.NewSettlementAuthorizationContextID()
	require.NoError(t, err)

	attempt := models.PaymentAttempt{
		AttemptID: "attempt-verified", Kind: models.PaymentAttemptKindCryptoFundingTarget,
		PaymentSessionID: "ps_" + orderID, OrderID: orderID, AmountValue: "1000",
		Currency: "crypto:eip155:1/native", RouteBindingID: "route-verified",
		IdempotencyKey: "attempt-verified", State: models.PaymentAttemptAuthorizationDraft,
		AuthorizationContextID: contextID,
	}
	terms := models.PaymentAttemptSettlementTerms{
		Version: models.PaymentAttemptSettlementTermsVersion, OrderID: orderID, AttemptID: attempt.AttemptID,
		AssetID: attempt.Currency, FundingAmount: "1000",
		FundingTargetAddress: "0x1111111111111111111111111111111111111111",
		RouteBindingID:       attempt.RouteBindingID, BuyerPeerID: buyerPeerID.String(), SellerPeerID: sellerPeerID.String(),
		SellerAddress: "0x2222222222222222222222222222222222222222", SellerGrossBasis: "1000",
		PlatformReleaseFee:   models.PaymentAttemptSettlementFee{Amount: "0"},
		BuyerCancellationFee: models.PaymentAttemptSettlementFee{Amount: "0"},
		DisputePolicy:        models.DisputeScalingSellerAwardProRataFloor,
	}
	sellerTermsPayload, err := terms.SellerSigningPayload()
	require.NoError(t, err)
	sellerTermsSignature, err := sellerPrivateKey.Sign(sellerTermsPayload)
	require.NoError(t, err)
	target := models.PaymentAttemptFundingTarget{
		Version: models.PaymentAttemptFundingTargetVersion, AttemptID: attempt.AttemptID,
		Type: models.PaymentAttemptFundingTargetAddress, AssetID: attempt.Currency,
		AmountAtomic: terms.FundingAmount, Address: terms.FundingTargetAddress,
	}
	_, termsHash, err := terms.CanonicalBytesAndHash()
	require.NoError(t, err)
	_, targetHash, err := target.CanonicalBytesAndHash()
	require.NoError(t, err)
	signOffer := func(role models.SettlementParticipantRole, participant peer.ID, privateKey libp2pcrypto.PrivKey, publicKey []byte) models.SettlementKeyOffer {
		t.Helper()
		offer := models.SettlementKeyOffer{
			Version: models.SettlementAuthorizationVersion, AuthorizationContextID: contextID,
			OrderID: orderID, AttemptID: attempt.AttemptID, ParticipantPeerID: participant.String(),
			ParticipantRole: role, RailID: attempt.Currency,
			Purpose: "standard-order-participant:" + string(role), PublicKey: publicKey,
		}
		payload, err := offer.SigningPayload()
		require.NoError(t, err)
		offer.Signature, err = privateKey.Sign(payload)
		require.NoError(t, err)
		return offer
	}
	bundle := models.PaymentAttemptAuthorizationBundle{
		Version: models.SettlementAuthorizationVersion, AuthorizationContextID: contextID,
		OrderID: orderID, AttemptID: attempt.AttemptID, RailID: attempt.Currency,
		SettlementTermsHash: termsHash, FundingTargetHash: targetHash,
		RequiredRoles: []models.SettlementParticipantRole{models.SettlementParticipantBuyer, models.SettlementParticipantSeller},
		Offers: []models.SettlementKeyOffer{
			signOffer(models.SettlementParticipantBuyer, buyerPeerID, buyerPrivateKey, []byte("buyer-settlement-key")),
			signOffer(models.SettlementParticipantSeller, sellerPeerID, sellerPrivateKey, []byte("seller-settlement-key")),
		},
		SellerTermsSigner: sellerPeerID.String(), SellerTermsSignature: sellerTermsSignature,
	}
	require.NoError(t, attempt.SetSettlementTerms(terms))
	require.NoError(t, attempt.SetSellerTermsAuthorization(sellerPeerID.String(), sellerTermsSignature))
	require.NoError(t, attempt.SetAuthorizationBundle(bundle))
	require.NoError(t, attempt.SetFundingTarget(target))
	return attempt
}

func TestPaymentSessionServiceImpl_CreateSession_SkipsProvisioningWhenNotReady(t *testing.T) {
	db, err := repo.MockDB()
	require.NoError(t, err)
	defer db.Close()

	orderID := "QmCreateSessionGate"
	require.NoError(t, db.Update(func(tx database.Tx) error {
		if err := tx.Migrate(&models.Order{}); err != nil {
			return err
		}
		return tx.Save(&models.Order{
			ID:     models.OrderID(orderID),
			MyRole: string(models.RoleBuyer),
			Open:   true,
		})
	}))

	svc := NewPaymentSessionService(db)
	svc.SetCryptoFacade(&CryptoPaymentFacade{
		db:        db,
		projector: NewPaymentSessionProjector(db),
		setupSvc:  panicSetupService{t: t},
	})

	session, err := svc.CreateSession(context.Background(), contracts.CreatePaymentSessionRequest{
		OrderID:     orderID,
		PaymentCoin: "crypto:eip155:1:native",
	})
	require.NoError(t, err)
	require.Equal(t, pkpayment.PaymentReadinessAwaitingSellerReceipt, session.PaymentReadiness.Status)
	require.Empty(t, session.FundingTarget.Address)
}

func TestPaymentSessionServiceImpl_CreateSessionRunsPoliciesBeforeRailFacade(t *testing.T) {
	db, err := repo.MockDB()
	require.NoError(t, err)
	defer db.Close()

	readyAt := time.Now()
	orderID := "QmCreateSessionPolicy"
	require.NoError(t, db.Update(func(tx database.Tx) error {
		if err := tx.Migrate(&models.Order{}); err != nil {
			return err
		}
		return tx.Save(&models.Order{
			ID: models.OrderID(orderID), MyRole: string(models.RoleBuyer), Open: true, PaymentReadyAt: &readyAt,
		})
	}))

	wantErr := errors.New("policy rejected")
	svc := NewPaymentSessionService(db)
	svc.AddProvisioningPolicy(rejectingProvisioningPolicy{err: wantErr})
	svc.SetCryptoFacade(&CryptoPaymentFacade{
		db: db, projector: NewPaymentSessionProjector(db), setupSvc: panicSetupService{t: t},
	})
	_, err = svc.CreateSession(context.Background(), contracts.CreatePaymentSessionRequest{
		OrderID: orderID, PaymentCoin: "crypto:eip155:1:native",
	})
	if !errors.Is(err, wantErr) {
		t.Fatalf("CreateSession error = %v, want policy error", err)
	}
}

func TestPaymentSessionServiceImpl_AuthorizationDraftBlocksLegacyProvisioning(t *testing.T) {
	db, err := repo.MockDB()
	require.NoError(t, err)
	defer db.Close()

	readyAt := time.Now()
	const orderID = "QmAuthorizationDraftGate"
	require.NoError(t, db.Update(func(tx database.Tx) error {
		if err := tx.Migrate(&models.Order{}); err != nil {
			return err
		}
		if err := tx.Migrate(&models.PaymentAttempt{}); err != nil {
			return err
		}
		if err := tx.Save(&models.Order{
			ID: models.OrderID(orderID), MyRole: string(models.RoleBuyer), Open: true, PaymentReadyAt: &readyAt,
		}); err != nil {
			return err
		}
		return tx.Create(&models.PaymentAttempt{
			AttemptID: "attempt-draft", Kind: models.PaymentAttemptKindCryptoFundingTarget,
			PaymentSessionID: "ps_" + orderID, OrderID: orderID, AmountValue: "1000",
			Currency: "crypto:eip155:1:native", RouteBindingID: "route-draft",
			IdempotencyKey: "attempt-draft", State: models.PaymentAttemptAuthorizationDraft,
		})
	}))

	svc := NewPaymentSessionService(db)
	svc.SetCryptoFacade(&CryptoPaymentFacade{
		db: db, projector: NewPaymentSessionProjector(db), setupSvc: panicSetupService{t: t},
	})
	session, err := svc.CreateSession(context.Background(), contracts.CreatePaymentSessionRequest{
		OrderID: orderID, PaymentCoin: "crypto:eip155:1:native",
	})
	require.NoError(t, err)
	require.Empty(t, session.FundingTarget.Address)

	session, err = svc.CreateSession(context.Background(), contracts.CreatePaymentSessionRequest{
		OrderID: orderID, PaymentCoin: "crypto:eip155:56:native",
	})
	require.NoError(t, err)
	// A draft has not frozen an actionable target yet; in particular, the
	// caller's replacement coin must not be projected into the session.
	require.Empty(t, session.PaymentCoin)
	require.Equal(t, pkpayment.PaymentReadinessAwaitingSellerReceipt, session.PaymentReadiness.Status)
	require.Empty(t, session.FundingTarget.Address)
}

func TestPaymentSessionServiceImpl_CoinSwitchAuthorizesBeforeClearingExistingTarget(t *testing.T) {
	db, err := repo.MockDB()
	require.NoError(t, err)
	defer db.Close()

	readyAt := time.Now()
	orderID := "QmCoinSwitchPolicyOrder"
	oldAddress := "0x111122223333444455556666777788889999aaaa"
	order := &models.Order{
		ID: models.OrderID(orderID), MyRole: string(models.RoleBuyer), Open: true,
		PaymentReadyAt: &readyAt, PaymentAddress: oldAddress,
	}
	require.NoError(t, order.SetPendingManagedEscrowInfo(&models.PendingManagedEscrowInfo{
		Coin: "crypto:eip155:1:native", Address: oldAddress,
	}))
	require.NoError(t, db.Update(func(tx database.Tx) error {
		if err := tx.Migrate(&models.Order{}); err != nil {
			return err
		}
		return tx.Save(order)
	}))

	wantErr := errors.New("new rail reservation rejected")
	svc := NewPaymentSessionService(db)
	svc.AddProvisioningPolicy(provisioningPolicyFunc(func(_ context.Context, _ SessionProvisioningPolicyInput) error {
		var current models.Order
		require.NoError(t, db.View(func(tx database.Tx) error {
			return tx.Read().Where("id = ?", orderID).First(&current).Error
		}))
		require.Equal(t, oldAddress, current.PaymentAddress, "authorization must run before destructive session clearing")
		return wantErr
	}))

	_, err = svc.CreateSession(context.Background(), contracts.CreatePaymentSessionRequest{
		OrderID: orderID, PaymentCoin: "crypto:eip155:56:native",
	})
	require.ErrorIs(t, err, wantErr)
	var persisted models.Order
	require.NoError(t, db.View(func(tx database.Tx) error {
		return tx.Read().Where("id = ?", orderID).First(&persisted).Error
	}))
	require.Equal(t, oldAddress, persisted.PaymentAddress)
	require.NotEmpty(t, persisted.PendingPaymentInfo)
}

type panicSetupService struct {
	t *testing.T
}

func (p panicSetupService) GeneratePaymentSetup(context.Context, pkpayment.PaymentSetupParams) (*pkpayment.PaymentSetupResult, error) {
	p.t.Fatal("GeneratePaymentSetup must not run when payment is not ready")
	return nil, nil
}
