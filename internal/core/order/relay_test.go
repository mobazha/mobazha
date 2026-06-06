//go:build !private_distribution

package order

import (
	"testing"

	"github.com/mobazha/mobazha3.0/internal/orders"
	"github.com/mobazha/mobazha3.0/internal/repo"
	"github.com/mobazha/mobazha3.0/pkg/contracts"
	"github.com/mobazha/mobazha3.0/pkg/database"
	"github.com/mobazha/mobazha3.0/pkg/events"
	"github.com/mobazha/mobazha3.0/pkg/models"
	pb "github.com/mobazha/mobazha3.0/pkg/orders/mbzpb"
	"github.com/mobazha/mobazha3.0/pkg/payment"
	iwallet "github.com/mobazha/mobazha3.0/pkg/wallet-interface"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockEscrowRelay struct {
	called bool
	txHash string
	err    error
}

func (m *mockEscrowRelay) GetPayoutAddress(string) (iwallet.Address, error) {
	return iwallet.NewAddress("ignored", iwallet.CtMock), nil
}

func (m *mockEscrowRelay) ReleaseCancelableFunds(*models.Order, string) (iwallet.TransactionID, string, error) {
	return "", "", nil
}

func (m *mockEscrowRelay) ReleaseFromCancelableAddressWithParams(*models.Order, contracts.ReleaseFromCancelableParams) (iwallet.Tx, *iwallet.Transaction, error) {
	return nil, nil, nil
}

func (m *mockEscrowRelay) CancelPartialPayment(string) (string, uint64, error) {
	return "", 0, nil
}

func (m *mockEscrowRelay) RelayInstructions(_ string, _ iwallet.CoinType, _ any) (string, error) {
	m.called = true
	return m.txHash, m.err
}

func TestNormalizeFiatPaymentCoin_RejectsMissingProviderSegment(t *testing.T) {
	_, err := normalizeFiatPaymentCoin(
		iwallet.CoinType("fiat:USD"),
		pb.PaymentSent_FIAT,
		"",
		"USD",
	)
	require.Error(t, err)
}

func TestNormalizeFiatPaymentCoin_RejectsLegacyAlias(t *testing.T) {
	_, err := normalizeFiatPaymentCoin(
		iwallet.CoinType("FIAT_STRIPE"),
		pb.PaymentSent_FIAT,
		"",
		"USD",
	)
	require.Error(t, err)
}

func TestNormalizeFiatPaymentCoin_RejectsEmptyCoinWithoutProviderHint(t *testing.T) {
	_, err := normalizeFiatPaymentCoin(
		iwallet.CoinType(""),
		pb.PaymentSent_FIAT,
		"",
		"USD",
	)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "fiat provider is empty")
}

func TestNormalizeFiatPaymentCoin_UsesProviderHintWhenCoinEmpty(t *testing.T) {
	coin, err := normalizeFiatPaymentCoin(
		iwallet.CoinType(""),
		pb.PaymentSent_FIAT,
		"paypal",
		"usd",
	)
	require.NoError(t, err)
	assert.Equal(t, "fiat:paypal:USD", string(coin))
}

func TestRelayOrDirect_FiatBypassesEscrowRelay(t *testing.T) {
	svc := newTestOrderAppService(t, OrderAppServiceConfig{})
	escrow := &mockEscrowRelay{}
	svc.escrow = escrow

	directCalled := false
	relayedCalled := false

	err := svc.relayOrDirect(
		models.OrderID("order-fiat-relay"),
		"decline",
		iwallet.CoinType("fiat:paypal:USD"),
		&FiatRefundInstructions{Provider: "paypal"},
		func() error {
			directCalled = true
			return nil
		},
		func(iwallet.TransactionID) error {
			relayedCalled = true
			return nil
		},
	)

	require.NoError(t, err)
	assert.True(t, directCalled)
	assert.False(t, relayedCalled)
	assert.False(t, escrow.called)
}

func TestRelayOrDirect_CryptoInstructionsUseEscrowRelay(t *testing.T) {
	svc := newTestOrderAppService(t, OrderAppServiceConfig{})
	escrow := &mockEscrowRelay{txHash: "0xrelay"}
	svc.escrow = escrow

	directCalled := false
	relayedTx := iwallet.TransactionID("")

	err := svc.relayOrDirect(
		models.OrderID("order-crypto-relay"),
		"decline",
		iwallet.CoinType("crypto:eip155:137:erc20:0xc2132D05D31c914a87C6611C10748AEb04B58e8F"),
		struct{ Payload string }{Payload: "relay-me"},
		func() error {
			directCalled = true
			return nil
		},
		func(txid iwallet.TransactionID) error {
			relayedTx = txid
			return nil
		},
	)

	require.NoError(t, err)
	assert.False(t, directCalled)
	assert.True(t, escrow.called)
	assert.Equal(t, iwallet.TransactionID("0xrelay"), relayedTx)
}

func TestCancelOrderViaRelay_UnfundedOrderBypassesPaymentSent(t *testing.T) {
	buyerSigner, buyerPeerID := testSigner(t)
	_, sellerPeerID := testSigner(t)
	bus := events.NewBus()
	db, err := repo.MockDB()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	op := orders.NewOrderProcessor(&orders.Config{
		NodeID:    "cancel-relay-test",
		Db:        db,
		Signer:    buyerSigner,
		Messenger: noopMessenger{},
		EventBus:  bus,
	})
	svc := newTestOrderAppService(t, OrderAppServiceConfig{
		DB:             db,
		Signer:         buyerSigner,
		OrderProcessor: op,
		Messenger:      noopMessenger{},
		EventBus:       bus,
		NodeID:         "cancel-relay-test",
	})

	orderID := "unfunded-cancel-via-relay"
	order := &models.Order{
		ID:                  models.OrderID(orderID),
		MyRole:              string(models.RoleBuyer),
		SerializedOrderOpen: signedOrderOpen(t, buyerPeerID, sellerPeerID),
	}
	order.SetFSMState(models.OrderState_AWAITING_PAYMENT)
	require.NoError(t, svc.db.Update(func(tx database.Tx) error {
		return tx.Save(order)
	}))

	require.NoError(t, svc.CancelOrderViaRelay(models.OrderID(orderID), nil))

	var stored models.Order
	require.NoError(t, svc.db.View(func(tx database.Tx) error {
		return tx.Read().Where("id = ?", orderID).First(&stored).Error
	}))
	cancel, err := stored.OrderCancelMessage()
	require.NoError(t, err)
	assert.Empty(t, cancel.TransactionID)
	assert.False(t, stored.Open)
}

func TestDeclineOrderViaRelay_ManagedEscrowModeratedBeforeConfirmUsesSettlementCancel(t *testing.T) {
	sellerSigner, sellerPeerID := testSigner(t)
	_, buyerPeerID := testSigner(t)
	bus := events.NewBus()
	db, err := repo.MockDB()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	op := orders.NewOrderProcessor(&orders.Config{
		NodeID:    "decline-relay-test",
		Db:        db,
		Signer:    sellerSigner,
		Messenger: noopMessenger{},
		EventBus:  bus,
	})
	reg := payment.NewRegistry()
	strategy := &fakeManagedEscrowStrategy{
		model:        payment.PaymentModelMonitored,
		signatures:   []payment.ActionOwnerSignature{{From: "0x2222222222222222222222222222222222222222", Signature: []byte{0xaa}, Index: 1}},
		actionResult: &payment.ActionResult{Mode: payment.ActionModeSubmitted, ActionID: "managed_escrow-cancel-action"},
		actionStatus: &payment.ActionStatus{TxHash: "0xdddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd"},
	}
	reg.RegisterV2(iwallet.ChainEthereum, strategy)

	svc := newTestOrderAppService(t, OrderAppServiceConfig{
		DB:              db,
		Signer:          sellerSigner,
		OrderProcessor:  op,
		Messenger:       noopMessenger{},
		EventBus:        bus,
		NodeID:          "decline-relay-test",
		PaymentRegistry: reg,
	})

	coinType := iwallet.CoinType("crypto:eip155:1:native")
	order, paymentSent := newManagedEscrowOrderForTests(t, coinType)
	order.ID = models.OrderID("managed_escrow-moderated-decline-via-relay")
	order.MyRole = string(models.RoleVendor)
	order.SerializedOrderOpen = signedOrderOpen(t, buyerPeerID, sellerPeerID)
	order.SetFSMState(models.OrderState_PENDING)
	paymentSent.SettlementSpec = payment.NewManagedEscrowSpec(true).ToPaymentSent()
	paymentSent.RefundAddress = "0x1111111111111111111111111111111111111111"
	require.NoError(t, order.SetPaymentSent(paymentSent))
	require.NoError(t, order.SetPendingManagedEscrowPaymentInfo(&models.PendingManagedEscrowPaymentInfo{
		Coin:           paymentSent.Coin,
		Address:        order.PaymentAddress,
		SettlementSpec: payment.NewManagedEscrowSpec(true).ToPending(),
	}))
	order.MarkPaymentVerified()
	require.NoError(t, svc.db.Update(func(tx database.Tx) error {
		return tx.Save(order)
	}))

	require.NoError(t, svc.DeclineOrderViaRelay(order.ID, "seller declined before confirm", nil))
	assert.Equal(t, 0, strategy.cancelCalls)
	assert.Equal(t, 1, strategy.signActionCalls)
	assert.Equal(t, "cancel", strategy.lastAction)
	assert.Equal(t, paymentSent.RefundAddress, strategy.lastParams.PayoutAddr)

	var stored models.Order
	require.NoError(t, svc.db.View(func(tx database.Tx) error {
		return tx.Read().Where("id = ?", order.ID.String()).First(&stored).Error
	}))
	decline, err := stored.OrderDeclineMessage()
	require.NoError(t, err)
	assert.Empty(t, decline.TransactionID)
	refunds, err := stored.Refunds()
	require.NoError(t, err)
	require.Len(t, refunds, 1)
	assert.Empty(t, refunds[0].GetTransactionID())
	require.NotNil(t, refunds[0].GetReleaseInfo())
	assert.Equal(t, paymentSent.RefundAddress, refunds[0].GetReleaseInfo().ToAddress)
	require.Len(t, refunds[0].GetReleaseInfo().EscrowSignatures, 1)
	assert.False(t, stored.Open)
}
