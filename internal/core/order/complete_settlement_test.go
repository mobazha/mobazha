package order

import (
	"errors"
	"testing"

	"github.com/libp2p/go-libp2p/core/peer"
	intdb "github.com/mobazha/mobazha3.0/internal/database"
	"github.com/mobazha/mobazha3.0/internal/orders"
	utils "github.com/mobazha/mobazha3.0/internal/orders/testutil"
	"github.com/mobazha/mobazha3.0/internal/repo"
	"github.com/mobazha/mobazha3.0/pkg/contracts"
	"github.com/mobazha/mobazha3.0/pkg/core/coreiface"
	"github.com/mobazha/mobazha3.0/pkg/database"
	"github.com/mobazha/mobazha3.0/pkg/events"
	"github.com/mobazha/mobazha3.0/pkg/models"
	pb "github.com/mobazha/mobazha3.0/pkg/orders/mbzpb"
	"github.com/mobazha/mobazha3.0/pkg/payment"
	iwallet "github.com/mobazha/mobazha3.0/pkg/wallet-interface"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func seedManagedEscrowModeratedShippedOrderForComplete(
	t *testing.T,
	svc *OrderAppService,
	orderID string,
	buyerPeerID, sellerPeerID peer.ID,
) {
	t.Helper()

	coinType := iwallet.CoinType("crypto:eip155:11155111:native")
	order := &models.Order{
		ID:             models.OrderID(orderID),
		MyRole:         string(models.RoleBuyer),
		PaymentAddress: "0x9999999999999999999999999999999999999999",
		Open:           true,
	}
	order.SetFSMState(models.OrderState_SHIPPED)

	require.NoError(t, order.PutMessage(utils.MustWrapOrderMessage(&pb.OrderOpen{
		BuyerID:   &pb.ID{PeerID: buyerPeerID.String()},
		Chaincode: "01020304",
		Listings: []*pb.SignedListing{{
			Listing: &pb.Listing{
				VendorID: &pb.ID{PeerID: sellerPeerID.String()},
				Slug:     "managed_escrow-complete-test",
				Metadata: &pb.Listing_Metadata{ContractType: pb.Listing_Metadata_PHYSICAL_GOOD},
				Item: &pb.Listing_Item{
					Title: "ManagedEscrow complete item",
					Images: []*pb.Image{{
						Tiny:  "ipfs://tiny",
						Small: "ipfs://small",
					}},
				},
			},
		}},
		Items: []*pb.OrderOpen_Item{{ListingHash: "listing-1", Quantity: "1"}},
	})))
	require.NoError(t, order.PutMessage(utils.MustWrapOrderMessage(&pb.OrderShipment{
		Shipments: []*pb.OrderShipment_ShippedItem{{
			ItemIndex: 0,
		}},
		ReleaseInfo: &pb.EscrowRelease{
			ToAddress: "0x1111111111111111111111111111111111111111",
		},
	})))

	paymentSent := &pb.PaymentSent{
		Coin:           coinType.String(),
		Amount:         "1000000000000000000",
		ToAddress:      order.PaymentAddress,
		Moderator:      "12D3KooWManagedEscrowModerator",
		Chaincode:      "abcd",
		Script:         "beef",
		PlatformAddr:   "0x7777777777777777777777777777777777777777",
		SettlementSpec: payment.NewManagedEscrowSpec(true).ToPaymentSent(),
	}
	require.NoError(t, order.SetPaymentSent(paymentSent))
	require.NoError(t, order.SetPendingManagedEscrowPaymentInfo(&models.PendingManagedEscrowPaymentInfo{
		Coin:           paymentSent.Coin,
		Address:        order.PaymentAddress,
		SettlementSpec: payment.NewManagedEscrowSpec(true).ToPending(),
	}))

	require.NoError(t, svc.db.Update(func(tx database.Tx) error {
		return tx.Save(order)
	}))
}

func seedSettlementCompleteAction(
	t *testing.T,
	svc *OrderAppService,
	orderID, actionID, state, txHash string,
) {
	t.Helper()

	row := &models.SettlementAction{
		ActionID:   actionID,
		OrderID:    orderID,
		ActionKind: "complete",
		State:      state,
		TxHash:     txHash,
	}
	require.NoError(t, svc.db.Update(func(tx database.Tx) error {
		return tx.Save(row)
	}))
}

func newManagedEscrowCompleteTestService(
	t *testing.T,
	strategy *fakeManagedEscrowStrategy,
	buyerSigner contracts.Signer,
	buyerPeerID peer.ID,
) *OrderAppService {
	t.Helper()

	reg := payment.NewRegistry()
	reg.RegisterV2(iwallet.ChainEthereum, strategy)

	db, err := repo.MockDB()
	require.NoError(t, err)
	require.NoError(t, intdb.MigrateSettlementActionModels(db))
	require.NoError(t, db.Update(func(tx database.Tx) error {
		return tx.SetProfile(&models.Profile{Name: "Test Buyer"})
	}))
	t.Cleanup(func() { _ = db.Close() })

	bus := events.NewBus()
	op := orders.NewOrderProcessor(&orders.Config{
		NodeID:    "managed_escrow-complete-test",
		Db:        db,
		Signer:    buyerSigner,
		Messenger: noopMessenger{},
		EventBus:  bus,
	})

	return NewOrderAppService(OrderAppServiceConfig{
		DB:              db,
		PaymentRegistry: reg,
		Signer:          buyerSigner,
		OrderProcessor:  op,
		Messenger:       noopMessenger{},
		EventBus:        bus,
		NodeID:          "managed_escrow-complete-test",
		PeerID:          func() peer.ID { return buyerPeerID },
	})
}

func TestCompleteOrder_ManagedEscrowMonitored_RejectsPendingSettlementRelease(t *testing.T) {
	t.Parallel()

	buyerSigner, buyerPeerID := testSigner(t)
	_, sellerPeerID := testSigner(t)
	strategy := &fakeManagedEscrowStrategy{model: payment.PaymentModelMonitored}
	svc := newManagedEscrowCompleteTestService(t, strategy, buyerSigner, buyerPeerID)

	const orderID = "managed_escrow-complete-pending"
	seedManagedEscrowModeratedShippedOrderForComplete(t, svc, orderID, buyerPeerID, sellerPeerID)
	seedSettlementCompleteAction(t, svc, orderID, "complete-act-pending", "submitted", "")

	err := svc.CompleteOrder(models.OrderID(orderID), "", nil, false, nil)
	require.Error(t, err)
	assert.True(t, errors.Is(err, coreiface.ErrBadRequest))
	assert.Contains(t, err.Error(), "still pending")
	assert.Equal(t, 0, strategy.completeCalls)
}

func TestCompleteOrder_ManagedEscrowMonitored_SkipsInlineReleaseWhenSettlementTxHashReady(t *testing.T) {
	t.Parallel()

	buyerSigner, buyerPeerID := testSigner(t)
	_, sellerPeerID := testSigner(t)
	strategy := &fakeManagedEscrowStrategy{model: payment.PaymentModelMonitored}
	svc := newManagedEscrowCompleteTestService(t, strategy, buyerSigner, buyerPeerID)

	const (
		orderID = "managed_escrow-complete-ready"
		txHash  = "0xcompleteeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee"
	)
	seedManagedEscrowModeratedShippedOrderForComplete(t, svc, orderID, buyerPeerID, sellerPeerID)
	seedSettlementCompleteAction(t, svc, orderID, "complete-act-ready", "submitted", txHash)

	err := svc.CompleteOrder(models.OrderID(orderID), iwallet.TransactionID(txHash), nil, false, nil)
	require.NoError(t, err)
	assert.Equal(t, 0, strategy.completeCalls)

	var updated models.Order
	require.NoError(t, svc.db.View(func(tx database.Tx) error {
		return tx.Read().Where("id = ?", orderID).First(&updated).Error
	}))
	assert.False(t, updated.Open)
	assert.NotNil(t, updated.CompletedAt)
	_, err = updated.OrderCompleteMessage()
	require.NoError(t, err)
}

func seedUTXOModeratedShippedOrderForComplete(
	t *testing.T,
	svc *OrderAppService,
	orderID string,
	buyerPeerID, sellerPeerID peer.ID,
) {
	t.Helper()

	coinType := iwallet.CoinType("BCH")
	order := &models.Order{
		ID:             models.OrderID(orderID),
		MyRole:         string(models.RoleBuyer),
		PaymentAddress: "bitcoincash:qpayment",
		Open:           true,
	}
	order.SetFSMState(models.OrderState_SHIPPED)

	require.NoError(t, order.PutMessage(utils.MustWrapOrderMessage(&pb.OrderOpen{
		BuyerID:   &pb.ID{PeerID: buyerPeerID.String()},
		Chaincode: "01020304",
		Listings: []*pb.SignedListing{{
			Listing: &pb.Listing{
				VendorID: &pb.ID{PeerID: sellerPeerID.String()},
				Slug:     "utxo-complete-test",
				Metadata: &pb.Listing_Metadata{ContractType: pb.Listing_Metadata_PHYSICAL_GOOD},
				Item: &pb.Listing_Item{
					Title: "UTXO complete item",
					Images: []*pb.Image{{
						Tiny:  "ipfs://tiny",
						Small: "ipfs://small",
					}},
				},
			},
		}},
		Items: []*pb.OrderOpen_Item{{ListingHash: "listing-1", Quantity: "1"}},
	})))
	require.NoError(t, order.PutMessage(utils.MustWrapOrderMessage(&pb.OrderShipment{
		Shipments: []*pb.OrderShipment_ShippedItem{{ItemIndex: 0}},
		ReleaseInfo: &pb.EscrowRelease{
			ToAddress: "bitcoincash:qvendor",
		},
	})))

	paymentSent := &pb.PaymentSent{
		Coin:           string(coinType),
		Amount:         "100000",
		ToAddress:      order.PaymentAddress,
		Moderator:      "12D3KooWModerator",
		Chaincode:      "abcd",
		Script:         "beef",
		SettlementSpec: payment.NewUTXOSpec(true).ToPaymentSent(),
	}
	require.NoError(t, order.SetPaymentSent(paymentSent))

	require.NoError(t, svc.db.Update(func(tx database.Tx) error {
		return tx.Save(order)
	}))
}

func newUTXOCompleteTestService(
	t *testing.T,
	buyerSigner contracts.Signer,
	buyerPeerID peer.ID,
) *OrderAppService {
	t.Helper()

	reg := payment.NewRegistry()
	reg.RegisterV2(iwallet.ChainBitcoinCash, &fakeManagedEscrowStrategy{model: payment.PaymentModelMonitored})

	db, err := repo.MockDB()
	require.NoError(t, err)
	require.NoError(t, intdb.MigrateSettlementActionModels(db))
	require.NoError(t, db.Update(func(tx database.Tx) error {
		return tx.SetProfile(&models.Profile{Name: "Test Buyer"})
	}))
	t.Cleanup(func() { _ = db.Close() })

	bus := events.NewBus()
	op := orders.NewOrderProcessor(&orders.Config{
		NodeID:    "utxo-complete-test",
		Db:        db,
		Signer:    buyerSigner,
		Messenger: noopMessenger{},
		EventBus:  bus,
	})

	return NewOrderAppService(OrderAppServiceConfig{
		DB:              db,
		PaymentRegistry: reg,
		Signer:          buyerSigner,
		OrderProcessor:  op,
		Messenger:       noopMessenger{},
		EventBus:        bus,
		NodeID:          "utxo-complete-test",
		PeerID:          func() peer.ID { return buyerPeerID },
	})
}

func TestCompleteOrder_UTXOMonitored_RequiresSettlementAction(t *testing.T) {
	t.Parallel()

	buyerSigner, buyerPeerID := testSigner(t)
	_, sellerPeerID := testSigner(t)
	svc := newUTXOCompleteTestService(t, buyerSigner, buyerPeerID)

	const orderID = "utxo-complete-no-settlement"
	seedUTXOModeratedShippedOrderForComplete(t, svc, orderID, buyerPeerID, sellerPeerID)

	err := svc.CompleteOrder(models.OrderID(orderID), "", nil, false, nil)
	require.Error(t, err)
	assert.True(t, errors.Is(err, coreiface.ErrBadRequest))
	assert.Contains(t, err.Error(), "settlement-actions/complete")
}

func TestCompleteOrder_RejectsClientTxIDWithoutSettlementEvidence(t *testing.T) {
	t.Parallel()

	buyerSigner, buyerPeerID := testSigner(t)
	_, sellerPeerID := testSigner(t)
	svc := newUTXOCompleteTestService(t, buyerSigner, buyerPeerID)

	const orderID = "utxo-complete-fake-txid"
	seedUTXOModeratedShippedOrderForComplete(t, svc, orderID, buyerPeerID, sellerPeerID)

	err := svc.CompleteOrder(models.OrderID(orderID), iwallet.TransactionID("fake-txid-without-settlement"), nil, false, nil)
	require.Error(t, err)
	assert.True(t, errors.Is(err, coreiface.ErrBadRequest))
	assert.Contains(t, err.Error(), "settlement-actions/complete")
}
