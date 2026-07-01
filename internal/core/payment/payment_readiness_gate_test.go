package payment

import (
	"context"
	"testing"
	"time"

	"github.com/mobazha/mobazha3.0/internal/repo"
	"github.com/mobazha/mobazha3.0/pkg/contracts"
	"github.com/mobazha/mobazha3.0/pkg/database"
	"github.com/mobazha/mobazha3.0/pkg/models"
	pkpayment "github.com/mobazha/mobazha3.0/pkg/payment"
	"github.com/stretchr/testify/require"
)

func TestPaymentSessionProjector_GatesBuyerFundingTarget(t *testing.T) {
	p := NewPaymentSessionProjector(nil)
	order := &models.Order{
		ID:             "QmBuyerNotReady",
		MyRole:         string(models.RoleBuyer),
		PaymentAddress: "0xmanagedescrow",
	}
	require.NoError(t, order.SetPendingManagedEscrowPaymentInfo(&models.PendingManagedEscrowPaymentInfo{
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
	require.NoError(t, order.SetPendingManagedEscrowPaymentInfo(&models.PendingManagedEscrowPaymentInfo{
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

type panicSetupService struct {
	t *testing.T
}

func (p panicSetupService) GeneratePaymentSetup(context.Context, pkpayment.PaymentSetupParams) (*pkpayment.PaymentSetupResult, error) {
	p.t.Fatal("GeneratePaymentSetup must not run when payment is not ready")
	return nil, nil
}
