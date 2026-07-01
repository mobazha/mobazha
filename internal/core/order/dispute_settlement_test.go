package order

import (
	"context"
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

func seedManagedEscrowModeratedDecidedOrderForDisputeRelease(
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
	order.SetFSMState(models.OrderState_DECIDED)

	require.NoError(t, order.PutMessage(utils.MustWrapOrderMessage(&pb.OrderOpen{
		BuyerID:   &pb.ID{PeerID: buyerPeerID.String()},
		Chaincode: "01020304",
		Listings: []*pb.SignedListing{{
			Listing: &pb.Listing{
				VendorID: &pb.ID{PeerID: sellerPeerID.String()},
				Slug:     "managed_escrow-dispute-release-test",
				Metadata: &pb.Listing_Metadata{ContractType: pb.Listing_Metadata_PHYSICAL_GOOD},
				Item: &pb.Listing_Item{
					Title: "ManagedEscrow dispute item",
					Images: []*pb.Image{{
						Tiny:  "ipfs://tiny",
						Small: "ipfs://small",
					}},
				},
			},
		}},
		Items: []*pb.OrderOpen_Item{{ListingHash: "listing-1", Quantity: "1"}},
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
	require.NoError(t, order.PutMessage(utils.MustWrapOrderMessage(&pb.DisputeClose{
		Verdict: "buyer wins partial",
		ReleaseInfo: &pb.DisputeClose_ModeratedEscrowRelease{
			BuyerAddress:     "0x1111111111111111111111111111111111111111",
			BuyerAmount:      "540000000000000000",
			VendorAddress:    "0x2222222222222222222222222222222222222222",
			VendorAmount:     "460000000000000000",
			ModeratorAddress: "0x3333333333333333333333333333333333333333",
			ModeratorAmount:  "0",
			TransactionFee:   "0",
		},
	})))

	require.NoError(t, svc.db.Update(func(tx database.Tx) error {
		return tx.Save(order)
	}))
}

func seedSettlementDisputeReleaseAction(
	t *testing.T,
	svc *OrderAppService,
	orderID, actionID, state, txHash string,
) {
	t.Helper()

	row := &models.SettlementAction{
		ActionID:   actionID,
		OrderID:    orderID,
		ActionKind: "dispute_release",
		State:      state,
		TxHash:     txHash,
	}
	require.NoError(t, svc.db.Update(func(tx database.Tx) error {
		return tx.Save(row)
	}))
}

func newManagedEscrowDisputeReleaseTestService(
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
		NodeID:    "managed_escrow-dispute-release-test",
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
		NodeID:          "managed_escrow-dispute-release-test",
		PeerID:          func() peer.ID { return buyerPeerID },
	})
}

func TestReleaseFunds_ManagedEscrowMonitored_RejectsPendingSettlementRelease(t *testing.T) {
	t.Parallel()

	buyerSigner, buyerPeerID := testSigner(t)
	_, sellerPeerID := testSigner(t)
	strategy := &fakeManagedEscrowStrategy{model: payment.PaymentModelMonitored}
	svc := newManagedEscrowDisputeReleaseTestService(t, strategy, buyerSigner, buyerPeerID)

	const orderID = "managed_escrow-dispute-pending"
	seedManagedEscrowModeratedDecidedOrderForDisputeRelease(t, svc, orderID, buyerPeerID, sellerPeerID)
	seedSettlementDisputeReleaseAction(t, svc, orderID, "dispute-act-pending", "submitted", "")

	err := svc.ReleaseFunds(models.OrderID(orderID), "", nil)
	require.Error(t, err)
	assert.True(t, errors.Is(err, coreiface.ErrBadRequest))
	assert.Contains(t, err.Error(), "still pending")
	assert.Equal(t, 0, strategy.disputeCalls)
}

func TestReleaseFunds_ManagedEscrowMonitored_SkipsInlineReleaseWhenSettlementTxHashReady(t *testing.T) {
	t.Parallel()

	buyerSigner, buyerPeerID := testSigner(t)
	_, sellerPeerID := testSigner(t)
	strategy := &fakeManagedEscrowStrategy{model: payment.PaymentModelMonitored}
	svc := newManagedEscrowDisputeReleaseTestService(t, strategy, buyerSigner, buyerPeerID)

	const (
		orderID = "managed_escrow-dispute-ready"
		txHash  = "0xdisputeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee"
	)
	seedManagedEscrowModeratedDecidedOrderForDisputeRelease(t, svc, orderID, buyerPeerID, sellerPeerID)
	seedSettlementDisputeReleaseAction(t, svc, orderID, "dispute-act-ready", "submitted", txHash)

	err := svc.ReleaseFunds(models.OrderID(orderID), iwallet.TransactionID(txHash), nil)
	require.NoError(t, err)
	assert.Equal(t, 0, strategy.disputeCalls)

	var updated models.Order
	require.NoError(t, svc.db.View(func(tx database.Tx) error {
		return tx.Read().Where("id = ?", orderID).First(&updated).Error
	}))
	assert.False(t, updated.Open)
	accept, err := updated.DisputeAcceptMessage()
	require.NoError(t, err)
	assert.Equal(t, txHash, accept.Txid)
}

func TestExecuteSettlementDisputeReleaseAction_ReturnsActionIDWithoutTxHash(t *testing.T) {
	t.Parallel()

	buyerSigner, buyerPeerID := testSigner(t)
	_, sellerPeerID := testSigner(t)
	strategy := &fakeManagedEscrowStrategy{
		model: payment.PaymentModelMonitored,
		actionResult: &payment.ActionResult{
			Mode:     payment.ActionModeSubmitted,
			ActionID: "dispute-act-async",
		},
	}
	svc := newManagedEscrowDisputeReleaseTestService(t, strategy, buyerSigner, buyerPeerID)

	const orderID = "managed_escrow-dispute-async-action"
	seedManagedEscrowModeratedDecidedOrderForDisputeRelease(t, svc, orderID, buyerPeerID, sellerPeerID)

	result, coinType, err := svc.ExecuteSettlementDisputeReleaseAction(context.Background(), models.OrderID(orderID))
	require.NoError(t, err)
	assert.NotEmpty(t, coinType)
	require.NotNil(t, result)
	assert.Equal(t, payment.ActionModeSubmitted, result.Mode)
	assert.Equal(t, "dispute-act-async", result.ActionID)
	assert.Empty(t, result.SubmittedTxHash)
	assert.Equal(t, 1, strategy.disputeCalls)
}

func TestExecuteSettlementDisputeReleaseAction_IdempotentWhenActionReady(t *testing.T) {
	t.Parallel()

	buyerSigner, buyerPeerID := testSigner(t)
	_, sellerPeerID := testSigner(t)
	strategy := &fakeManagedEscrowStrategy{
		model: payment.PaymentModelMonitored,
		actionResult: &payment.ActionResult{
			Mode:     payment.ActionModeSubmitted,
			ActionID: "should-not-be-called",
		},
	}
	svc := newManagedEscrowDisputeReleaseTestService(t, strategy, buyerSigner, buyerPeerID)

	const (
		orderID = "managed_escrow-dispute-idempotent-ready"
		txHash  = "0xdisputeidempotenteeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee"
	)
	seedManagedEscrowModeratedDecidedOrderForDisputeRelease(t, svc, orderID, buyerPeerID, sellerPeerID)
	seedSettlementDisputeReleaseAction(t, svc, orderID, "dispute-act-existing", "submitted", txHash)

	result, _, err := svc.ExecuteSettlementDisputeReleaseAction(context.Background(), models.OrderID(orderID))
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, "dispute-act-existing", result.ActionID)
	assert.Equal(t, txHash, result.SubmittedTxHash)
	assert.Equal(t, 0, strategy.disputeCalls)
}

func TestExecuteSettlementDisputeReleaseAction_IdempotentWhenActionPending(t *testing.T) {
	t.Parallel()

	buyerSigner, buyerPeerID := testSigner(t)
	_, sellerPeerID := testSigner(t)
	strategy := &fakeManagedEscrowStrategy{model: payment.PaymentModelMonitored}
	svc := newManagedEscrowDisputeReleaseTestService(t, strategy, buyerSigner, buyerPeerID)

	const orderID = "managed_escrow-dispute-idempotent-pending"
	seedManagedEscrowModeratedDecidedOrderForDisputeRelease(t, svc, orderID, buyerPeerID, sellerPeerID)
	seedSettlementDisputeReleaseAction(t, svc, orderID, "dispute-act-pending-retry", "submitted", "")

	result, _, err := svc.ExecuteSettlementDisputeReleaseAction(context.Background(), models.OrderID(orderID))
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, "dispute-act-pending-retry", result.ActionID)
	assert.Empty(t, result.SubmittedTxHash)
	assert.Equal(t, 0, strategy.disputeCalls)
}

func seedSolanaModeratedDecidedOrderForDisputeRelease(
	t *testing.T,
	svc *OrderAppService,
	orderID string,
	buyerPeerID, sellerPeerID peer.ID,
) {
	t.Helper()

	coinType := iwallet.CoinType("crypto:solana:mainnet:native")
	order := &models.Order{
		ID:             models.OrderID(orderID),
		MyRole:         string(models.RoleBuyer),
		PaymentAddress: "SolEscrow1111111111111111111111111111111111",
		Open:           true,
	}
	order.SetFSMState(models.OrderState_DECIDED)

	require.NoError(t, order.PutMessage(utils.MustWrapOrderMessage(&pb.OrderOpen{
		BuyerID:   &pb.ID{PeerID: buyerPeerID.String()},
		Chaincode: "01020304",
		Listings: []*pb.SignedListing{{
			Listing: &pb.Listing{
				VendorID: &pb.ID{PeerID: sellerPeerID.String()},
				Slug:     "solana-dispute-release-test",
				Metadata: &pb.Listing_Metadata{ContractType: pb.Listing_Metadata_PHYSICAL_GOOD},
				Item: &pb.Listing_Item{
					Title: "Solana dispute item",
					Images: []*pb.Image{{
						Tiny:  "ipfs://tiny",
						Small: "ipfs://small",
					}},
				},
			},
		}},
		Items: []*pb.OrderOpen_Item{{ListingHash: "listing-1", Quantity: "1"}},
	})))

	paymentSent := &pb.PaymentSent{
		Coin:           coinType.String(),
		Amount:         "1000000000",
		ToAddress:      order.PaymentAddress,
		Moderator:      "12D3KooWSolModerator",
		Chaincode:      "abcd",
		Script:         "beef",
		SettlementSpec: payment.NewSolanaEscrowSpec(true).ToPaymentSent(),
	}
	require.NoError(t, order.SetPaymentSent(paymentSent))
	require.NoError(t, order.PutMessage(utils.MustWrapOrderMessage(&pb.DisputeClose{
		Verdict: "buyer wins partial",
		ReleaseInfo: &pb.DisputeClose_ModeratedEscrowRelease{
			BuyerAddress:     "Buyer1111111111111111111111111111111111111111",
			BuyerAmount:      "540000000",
			VendorAddress:    "Vendor111111111111111111111111111111111111111",
			VendorAmount:     "460000000",
			ModeratorAddress: "Mod11111111111111111111111111111111111111111",
			ModeratorAmount:  "0",
			TransactionFee:   "0",
		},
	})))

	require.NoError(t, svc.db.Update(func(tx database.Tx) error {
		return tx.Save(order)
	}))
}

func newSolanaDisputeReleaseTestService(
	t *testing.T,
	strategy *fakeManagedEscrowStrategy,
	buyerSigner contracts.Signer,
	buyerPeerID peer.ID,
) *OrderAppService {
	t.Helper()

	reg := payment.NewRegistry()
	reg.RegisterV2(iwallet.ChainSolana, strategy)

	db, err := repo.MockDB()
	require.NoError(t, err)
	require.NoError(t, intdb.MigrateSettlementActionModels(db))
	require.NoError(t, db.Update(func(tx database.Tx) error {
		return tx.SetProfile(&models.Profile{Name: "Test Buyer"})
	}))
	t.Cleanup(func() { _ = db.Close() })

	bus := events.NewBus()
	op := orders.NewOrderProcessor(&orders.Config{
		NodeID:    "solana-dispute-release-test",
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
		NodeID:          "solana-dispute-release-test",
		PeerID:          func() peer.ID { return buyerPeerID },
	})
}

func TestReleaseFunds_SolanaMonitored_SkipsInlineReleaseWhenSettlementTxHashReady(t *testing.T) {
	t.Parallel()

	buyerSigner, buyerPeerID := testSigner(t)
	_, sellerPeerID := testSigner(t)
	strategy := &fakeManagedEscrowStrategy{model: payment.PaymentModelMonitored}
	svc := newSolanaDisputeReleaseTestService(t, strategy, buyerSigner, buyerPeerID)

	const (
		orderID = "solana-dispute-ready"
		txHash  = "SolTxDisputeReleaseSig111111111111111111111111111111111"
	)
	seedSolanaModeratedDecidedOrderForDisputeRelease(t, svc, orderID, buyerPeerID, sellerPeerID)
	seedSettlementDisputeReleaseAction(t, svc, orderID, "sol-dispute-act-ready", "submitted", txHash)

	err := svc.ReleaseFunds(models.OrderID(orderID), iwallet.TransactionID(txHash), nil)
	require.NoError(t, err)
	assert.Equal(t, 0, strategy.disputeCalls)

	var updated models.Order
	require.NoError(t, svc.db.View(func(tx database.Tx) error {
		return tx.Read().Where("id = ?", orderID).First(&updated).Error
	}))
	assert.False(t, updated.Open)
	accept, err := updated.DisputeAcceptMessage()
	require.NoError(t, err)
	assert.Equal(t, txHash, accept.Txid)
}

func TestExecuteSettlementDisputeReleaseAction_Solana_SubmitsRelease(t *testing.T) {
	t.Parallel()

	buyerSigner, buyerPeerID := testSigner(t)
	_, sellerPeerID := testSigner(t)
	strategy := &fakeManagedEscrowStrategy{
		model: payment.PaymentModelMonitored,
		actionResult: &payment.ActionResult{
			Mode:            payment.ActionModeSubmitted,
			ActionID:        "sol-dispute-act-async",
			SubmittedTxHash: "SolTxAsync1111111111111111111111111111111111111",
		},
	}
	svc := newSolanaDisputeReleaseTestService(t, strategy, buyerSigner, buyerPeerID)

	const orderID = "solana-dispute-async-action"
	seedSolanaModeratedDecidedOrderForDisputeRelease(t, svc, orderID, buyerPeerID, sellerPeerID)

	result, coinType, err := svc.ExecuteSettlementDisputeReleaseAction(context.Background(), models.OrderID(orderID))
	require.NoError(t, err)
	assert.NotEmpty(t, coinType)
	require.NotNil(t, result)
	assert.Equal(t, payment.ActionModeSubmitted, result.Mode)
	assert.Equal(t, "sol-dispute-act-async", result.ActionID)
	assert.Equal(t, 1, strategy.disputeCalls)
}

func seedUTXOModeratedDecidedOrderForDisputeRelease(
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
	order.SetFSMState(models.OrderState_DECIDED)

	require.NoError(t, order.PutMessage(utils.MustWrapOrderMessage(&pb.OrderOpen{
		BuyerID:   &pb.ID{PeerID: buyerPeerID.String()},
		Chaincode: "01020304",
		Listings: []*pb.SignedListing{{
			Listing: &pb.Listing{
				VendorID: &pb.ID{PeerID: sellerPeerID.String()},
				Slug:     "utxo-dispute-release-test",
				Metadata: &pb.Listing_Metadata{ContractType: pb.Listing_Metadata_PHYSICAL_GOOD},
				Item: &pb.Listing_Item{
					Title: "UTXO dispute item",
					Images: []*pb.Image{{
						Tiny:  "ipfs://tiny",
						Small: "ipfs://small",
					}},
				},
			},
		}},
		Items: []*pb.OrderOpen_Item{{ListingHash: "listing-1", Quantity: "1"}},
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
	require.NoError(t, order.PutMessage(utils.MustWrapOrderMessage(&pb.DisputeClose{
		Verdict: "buyer wins partial",
		ReleaseInfo: &pb.DisputeClose_ModeratedEscrowRelease{
			BuyerAddress:  "bitcoincash:qbuyer",
			BuyerAmount:   "54000",
			VendorAddress: "bitcoincash:qvendor",
			VendorAmount:  "46000",
		},
	})))

	require.NoError(t, svc.db.Update(func(tx database.Tx) error {
		return tx.Save(order)
	}))
}

func newUTXODisputeReleaseTestService(
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
		NodeID:    "utxo-dispute-release-test",
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
		NodeID:          "utxo-dispute-release-test",
		PeerID:          func() peer.ID { return buyerPeerID },
	})
}

func TestReleaseFunds_UTXOMonitored_RequiresSettlementAction(t *testing.T) {
	t.Parallel()

	buyerSigner, buyerPeerID := testSigner(t)
	_, sellerPeerID := testSigner(t)
	svc := newUTXODisputeReleaseTestService(t, buyerSigner, buyerPeerID)

	const orderID = "utxo-dispute-no-settlement"
	seedUTXOModeratedDecidedOrderForDisputeRelease(t, svc, orderID, buyerPeerID, sellerPeerID)

	err := svc.ReleaseFunds(models.OrderID(orderID), "", nil)
	require.Error(t, err)
	assert.True(t, errors.Is(err, coreiface.ErrBadRequest))
	assert.Contains(t, err.Error(), "settlement-actions/dispute-release")
}

func TestReleaseFunds_UTXOMonitored_SkipsInlineReleaseWhenSettlementTxHashReady(t *testing.T) {
	t.Parallel()

	buyerSigner, buyerPeerID := testSigner(t)
	_, sellerPeerID := testSigner(t)
	svc := newUTXODisputeReleaseTestService(t, buyerSigner, buyerPeerID)

	const (
		orderID = "utxo-dispute-ready"
		txHash  = "utxo-dispute-tx-hash-001"
	)
	seedUTXOModeratedDecidedOrderForDisputeRelease(t, svc, orderID, buyerPeerID, sellerPeerID)
	seedSettlementDisputeReleaseAction(t, svc, orderID, "utxo-dispute-act-ready", "confirmed", txHash)

	err := svc.ReleaseFunds(models.OrderID(orderID), iwallet.TransactionID(txHash), nil)
	require.NoError(t, err)

	var updated models.Order
	require.NoError(t, svc.db.View(func(tx database.Tx) error {
		return tx.Read().Where("id = ?", orderID).First(&updated).Error
	}))
	assert.False(t, updated.Open)
	accept, err := updated.DisputeAcceptMessage()
	require.NoError(t, err)
	assert.Equal(t, txHash, accept.Txid)
}

func TestReleaseFunds_RejectsClientTxIDWithoutSettlementEvidence(t *testing.T) {
	t.Parallel()

	buyerSigner, buyerPeerID := testSigner(t)
	_, sellerPeerID := testSigner(t)
	svc := newUTXODisputeReleaseTestService(t, buyerSigner, buyerPeerID)

	const orderID = "utxo-dispute-fake-txid"
	seedUTXOModeratedDecidedOrderForDisputeRelease(t, svc, orderID, buyerPeerID, sellerPeerID)

	err := svc.ReleaseFunds(models.OrderID(orderID), iwallet.TransactionID("fake-txid-without-settlement"), nil)
	require.Error(t, err)
	assert.True(t, errors.Is(err, coreiface.ErrBadRequest))
	assert.Contains(t, err.Error(), "settlement-actions/dispute-release")
}
