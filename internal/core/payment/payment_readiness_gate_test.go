package payment

import (
	"context"
	"errors"
	"testing"
	"time"

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
