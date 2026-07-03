package order

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/mobazha/mobazha/internal/database/dbstore"
	"github.com/mobazha/mobazha/internal/orderextensions"
	"github.com/mobazha/mobazha/internal/orders"
	"github.com/mobazha/mobazha/pkg/contracts"
	"github.com/mobazha/mobazha/pkg/database"
	"github.com/mobazha/mobazha/pkg/database/sqlitedialect"
	"github.com/mobazha/mobazha/pkg/events"
	"github.com/mobazha/mobazha/pkg/extensions"
	"github.com/mobazha/mobazha/pkg/identity"
	"github.com/mobazha/mobazha/pkg/models"
	npb "github.com/mobazha/mobazha/pkg/net/mbzpb"
	pb "github.com/mobazha/mobazha/pkg/orders/mbzpb"
	paymentpkg "github.com/mobazha/mobazha/pkg/payment"
	iwallet "github.com/mobazha/mobazha/pkg/wallet-interface"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/timestamppb"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func TestRelayPaymentToCounterparty_DeliversVerifiedPaymentToCoTenant(t *testing.T) {
	shared, err := gorm.Open(sqlitedialect.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err)
	require.NoError(t, shared.AutoMigrate(&models.Order{}, &models.OrderExtensionRecord{}, &models.OrderExtensionReservationRecord{}, &models.OrderExtensionEventSequence{}, &models.ExtensionDelivery{}))

	buyerDB := tenantDB(t, shared, "tenant-buyer")
	sellerDB := tenantDB(t, shared, "tenant-seller")

	buyerSigner, buyerPeerID := testSigner(t)
	sellerSigner, sellerPeerID := testSigner(t)
	bus := events.NewBus()
	op := orders.NewOrderProcessor(&orders.Config{
		NodeID:    "relay-test",
		Db:        buyerDB,
		Signer:    buyerSigner,
		Messenger: noopMessenger{},
		EventBus:  bus,
	})
	svc := NewOrderAppService(OrderAppServiceConfig{
		DB:             buyerDB,
		Signer:         buyerSigner,
		OrderProcessor: op,
		EventBus:       bus,
		NodeID:         "relay-test",
	})
	wireCoTenantVerifiedPayment(t, svc, "tenant-seller", sellerDB, sellerSigner)

	orderID := "cotenant-solana-payment"
	open := signedOrderOpen(t, buyerPeerID, sellerPeerID)
	seedTenantOrder(t, buyerDB, orderID, models.RoleBuyer, open)
	seedTenantOrder(t, sellerDB, orderID, models.RoleVendor, open)
	extension, err := extensions.NewOrderExtension(orderID, "io.mobazha.test", "test", extensions.ContractVersionV1, "resource-1", map[string]string{"value": "same"})
	require.NoError(t, err)
	extension.ReservationRequired = true
	extension.SettlementPolicy = extensions.SettlementPolicyExtensionAttested
	extension.LifecycleEvents = []string{
		extensions.EventOrderPaymentVerified,
		extensions.EventOrderReservationReleaseRequested,
	}
	for _, db := range []database.Database{buyerDB, sellerDB} {
		require.NoError(t, db.Update(func(tx database.Tx) error {
			return orderextensions.PersistTx(tx, orderID, extension)
		}))
	}
	expiresAt := time.Now().UTC().Add(time.Hour)
	require.NoError(t, buyerDB.Update(func(tx database.Tx) error {
		return orderextensions.RecordReservationTx(tx, extensions.ReservationRequest{
			OrderID: orderID, Extension: extension, PaymentCoin: "crypto:solana:mainnet:native",
			IdempotencyKey: "reserve-cotenant", ExpiresAt: expiresAt,
		}, extensions.Reservation{ID: "reservation-cotenant", Version: 1, Status: "reserved"})
	}))

	pd := &models.PaymentData{
		OrderID:       orderID,
		TransactionID: strings.Repeat("a", 64),
		Coin:          iwallet.CoinType("crypto:solana:mainnet:native"),
		Method:        pb.PaymentSent_DIRECT,
		Amount:        1_000_000,
		PayerAddress:  "payer-solana-address",
		ToAddress:     "escrow-solana-address",
		Timestamp:     time.Now().UTC(),
	}
	storedPaymentSent, err := BuildPaymentSentProto(mustFetchTenantOrder(t, buyerDB, orderID), pd)
	require.NoError(t, err)
	storedPaymentSent.FundingFacts = []*pb.PaymentSent_FundingFact{{
		Id:             "solana-fact-1",
		ChainNamespace: "solana",
		ChainReference: "mainnet",
		TxHash:         pd.TransactionID,
		TxHashSource:   models.PaymentTxHashSourceChainTx,
		EventIndex:     0,
		EventType:      models.PaymentEventSolanaTransfer,
		ToAddress:      pd.ToAddress,
		Amount:         "1000000",
		Status:         models.PaymentObservationStatusConfirmed,
	}}
	require.NoError(t, buyerDB.Update(func(tx database.Tx) error {
		var order models.Order
		if err := tx.Read().Where("id = ?", orderID).First(&order).Error; err != nil {
			return err
		}
		require.NoError(t, order.SetPaymentSent(storedPaymentSent))
		return tx.Save(&order)
	}))

	svc.RelayPaymentToCounterparty(context.Background(), orderID, sellerPeerID, pd)

	var sellerOrder models.Order
	require.NoError(t, sellerDB.View(func(tx database.Tx) error {
		return tx.Read().Where("id = ?", orderID).First(&sellerOrder).Error
	}))
	require.NotNil(t, sellerOrder.SerializedPaymentSent)
	require.True(t, sellerOrder.IsPaymentVerified())
	require.Equal(t, models.OrderState_PENDING, sellerOrder.State)
	relayedPaymentSent, err := sellerOrder.PaymentSentMessage()
	require.NoError(t, err)
	require.Len(t, relayedPaymentSent.GetFundingFacts(), 1)
	require.Equal(t, "solana-fact-1", relayedPaymentSent.GetFundingFacts()[0].GetId())
	var sellerReservation *extensions.ReservationBinding
	require.NoError(t, sellerDB.View(func(tx database.Tx) error {
		var err error
		sellerReservation, err = orderextensions.ReservationByExtensionTx(tx, orderID, extension.ExtensionID)
		return err
	}))
	require.NotNil(t, sellerReservation)
	require.Equal(t, "reservation-cotenant", sellerReservation.ReservationID)
	var delivery models.ExtensionDelivery
	require.NoError(t, sellerDB.View(func(tx database.Tx) error {
		return tx.Read().Where("order_id = ?", orderID).First(&delivery).Error
	}))
	var eventPayload extensions.PaymentVerifiedEventPayload
	require.NoError(t, json.Unmarshal(delivery.Payload, &eventPayload))
	require.NotNil(t, eventPayload.Reservation)
	require.Equal(t, sellerReservation.ReservationID, eventPayload.Reservation.ReservationID)
}

func TestPreProcessPaymentSent_HydratesIncomingManagedIntentBeforeValidation(t *testing.T) {
	db, err := dbstore.NewMemoryDB(t.TempDir())
	require.NoError(t, err)
	require.NoError(t, db.Update(func(tx database.Tx) error {
		if err := tx.Migrate(&models.Order{}); err != nil {
			return err
		}
		return tx.Migrate(&models.SharedPaymentIntent{})
	}))

	orderID := "incoming-managed-intent"
	orderOpenAny, err := anypb.New(&pb.OrderOpen{
		Timestamp:   timestamppb.Now(),
		Amount:      "1000",
		PricingCoin: "USD",
	})
	require.NoError(t, err)
	order := &models.Order{
		TenantMixin: models.TenantMixin{TenantID: database.StandaloneTenantID},
		ID:          models.OrderID(orderID),
	}
	order.SetRole(models.RoleVendor)
	require.NoError(t, order.PutMessage(&npb.OrderMessage{
		OrderID:     orderID,
		MessageType: npb.OrderMessage_ORDER_OPEN,
		Message:     orderOpenAny,
	}))
	require.NoError(t, db.Update(func(tx database.Tx) error { return tx.Save(order) }))

	paymentSent := &pb.PaymentSent{
		TransactionID:      "0xmanagedescrowtx",
		ContractAddress:    "0x2222222222222222222222222222222222222222",
		ToAddress:          "0x2222222222222222222222222222222222222222",
		Amount:             "21000000000000000",
		Coin:               "crypto:eip155:1:native",
		RefundAddress:      "0x3333333333333333333333333333333333333333",
		CancelFeeAmount:    "100",
		PlatformAmount:     "200",
		PlatformAddr:       "0x4444444444444444444444444444444444444444",
		SettlementSpec:     paymentpkg.NewManagedEscrowSpec(false).ToPaymentSent(),
		EscrowTimeoutHours: 1,
	}
	paymentSentAny, err := anypb.New(paymentSent)
	require.NoError(t, err)

	verifier := &capturingPaymentVerifier{}
	svc := &OrderAppService{
		db:              db,
		nodeID:          "incoming-managed-test",
		paymentVerifier: verifier,
	}

	ppCtx, err := svc.preProcessPaymentSent(context.Background(), &npb.OrderMessage{
		OrderID:     orderID,
		MessageType: npb.OrderMessage_PAYMENT_SENT,
		Message:     paymentSentAny,
	})
	require.NoError(t, err)
	require.NotNil(t, ppCtx)
	require.Equal(t, "21000000000000000", verifier.params.ExpectedPaymentAmount)
	require.Equal(t, "crypto:eip155:1:native", verifier.params.ExpectedPaymentCoin)

	var stored models.Order
	require.NoError(t, db.View(func(tx database.Tx) error {
		return tx.Read().Where("id = ?", orderID).First(&stored).Error
	}))
	info, err := stored.GetPendingManagedEscrowInfo()
	require.NoError(t, err)
	require.NotNil(t, info)
	require.Equal(t, "0x2222222222222222222222222222222222222222", info.Address)
	require.Equal(t, uint64(21000000000000000), info.Amount)
	require.Equal(t, "100", info.CancelFeeAmount)
	require.Equal(t, "0x3333333333333333333333333333333333333333", stored.RefundAddress)
}

func TestProcessOrderPayment_RelaysVerifiedPaymentWhenPersistFails(t *testing.T) {
	shared, err := gorm.Open(sqlitedialect.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err)
	require.NoError(t, shared.AutoMigrate(&models.Order{}, &models.OrderExtensionRecord{}, &models.OrderExtensionReservationRecord{}))

	buyerDB := tenantDB(t, shared, "tenant-buyer")
	sellerDB := tenantDB(t, shared, "tenant-seller")

	buyerSigner, buyerPeerID := testSigner(t)
	sellerSigner, sellerPeerID := testSigner(t)
	bus := events.NewBus()
	orderID := "cotenant-verified-persist-fails"
	open := signedOrderOpen(t, buyerPeerID, sellerPeerID)
	seedTenantOrder(t, buyerDB, orderID, models.RoleBuyer, open)
	seedTenantOrder(t, sellerDB, orderID, models.RoleVendor, open)

	failingBuyerDB := &failNthUpdateDB{
		Database: buyerDB,
		failOn:   2,
		err:      errors.New("forced verified persist failure"),
	}
	op := orders.NewOrderProcessor(&orders.Config{
		NodeID:    "relay-test",
		Db:        failingBuyerDB,
		Signer:    buyerSigner,
		Messenger: noopMessenger{},
		EventBus:  bus,
	})
	svc := NewOrderAppService(OrderAppServiceConfig{
		DB:             failingBuyerDB,
		Signer:         buyerSigner,
		OrderProcessor: op,
		Messenger:      noopMessenger{},
		EventBus:       bus,
		NodeID:         "relay-test",
	})
	wireCoTenantVerifiedPayment(t, svc, "tenant-seller", sellerDB, sellerSigner)
	paymentAddress := "bcrt1qverifiedpaymentaddress"
	firstToID := append([]byte(strings.Repeat("x", 32)), 1, 0, 0, 0)
	secondToID := append([]byte(strings.Repeat("y", 32)), 2, 0, 0, 0)
	tx := iwallet.Transaction{
		ID:    iwallet.TransactionID(strings.Repeat("b", 64)),
		Value: iwallet.NewAmount(1_000_000),
		To: []iwallet.SpendInfo{
			{
				ID:      firstToID,
				Address: iwallet.NewAddress(paymentAddress, iwallet.CoinType("crypto:bip122:000000000019d6689c085ae165831e93:native")),
				Amount:  iwallet.NewAmount(400_000),
			},
			{
				ID:      secondToID,
				Address: iwallet.NewAddress(paymentAddress, iwallet.CoinType("crypto:bip122:000000000019d6689c085ae165831e93:native")),
				Amount:  iwallet.NewAmount(600_000),
			},
		},
		Height: 7,
	}
	svc.SetPaymentVerifier(&verifiedPaymentVerifier{tx: tx})

	err = svc.ProcessOrderPayment(context.Background(), &models.PaymentData{
		OrderID:        orderID,
		TransactionID:  tx.ID.String(),
		Coin:           iwallet.CoinType("crypto:bip122:000000000019d6689c085ae165831e93:native"),
		Method:         pb.PaymentSent_DIRECT,
		Amount:         1_000_000,
		PayerAddress:   "payer-btc-address",
		ToAddress:      paymentAddress,
		Timestamp:      time.Now().UTC(),
		SettlementSpec: paymentpkg.NewDirectSpec().ToPending(),
	})
	require.NoError(t, err)
	require.Equal(t, 2, failingBuyerDB.updates)

	var sellerOrder models.Order
	require.NoError(t, sellerDB.View(func(tx database.Tx) error {
		return tx.Read().Where("id = ?", orderID).First(&sellerOrder).Error
	}))
	require.NotNil(t, sellerOrder.SerializedPaymentSent)
	require.True(t, sellerOrder.IsPaymentVerified())
	require.Equal(t, models.OrderState_PENDING, sellerOrder.State)
	txs, err := sellerOrder.GetTransactions()
	require.NoError(t, err)
	require.Len(t, txs, 1)
	require.Len(t, txs[0].To, 2)
	require.Equal(t, firstToID, txs[0].To[0].ID)
	require.Equal(t, secondToID, txs[0].To[1].ID)
	require.Equal(t, uint64(7), txs[0].Height)
}

func wireCoTenantVerifiedPayment(
	t *testing.T,
	source *OrderAppService,
	targetTenant string,
	targetDB database.Database,
	targetSigner contracts.Signer,
) {
	t.Helper()
	bus := events.NewBus()
	op := orders.NewOrderProcessor(&orders.Config{
		NodeID:    targetTenant,
		Db:        targetDB,
		Signer:    targetSigner,
		Messenger: noopMessenger{},
		EventBus:  bus,
	})
	target := NewOrderAppService(OrderAppServiceConfig{
		DB:             targetDB,
		Signer:         targetSigner,
		OrderProcessor: op,
		EventBus:       bus,
		NodeID:         targetTenant,
	})
	source.SetCoTenantVerifiedPayment(func(ctx context.Context, tenantID string, orderMsg *npb.OrderMessage, tx iwallet.Transaction) bool {
		if tenantID != targetTenant {
			return false
		}
		return target.ProcessVerifiedPaymentMessage(ctx, orderMsg, tx) == nil
	})
}

type noopMessenger struct{}

func (noopMessenger) ReliablySendMessage(database.Tx, peer.ID, *npb.Message, chan<- struct{}) error {
	return nil
}
func (noopMessenger) ProcessACK(database.Tx, *npb.AckMessage) error { return nil }
func (noopMessenger) SendACK(string, peer.ID)                       {}
func (noopMessenger) Start()                                        {}
func (noopMessenger) Stop()                                         {}

type failNthUpdateDB struct {
	database.Database
	failOn  int
	updates int
	err     error
}

func (db *failNthUpdateDB) Update(fn func(database.Tx) error) error {
	db.updates++
	if db.updates == db.failOn {
		return db.err
	}
	return db.Database.Update(fn)
}

func (db *failNthUpdateDB) RawDB() *gorm.DB {
	return db.Database.(interface{ RawDB() *gorm.DB }).RawDB()
}

func (db *failNthUpdateDB) ForTenant(tenantID string) (database.Database, error) {
	return db.Database.(tenantDatabaseRouter).ForTenant(tenantID)
}

type verifiedPaymentVerifier struct {
	tx iwallet.Transaction
}

func (v *verifiedPaymentVerifier) ValidateMessage(iwallet.CoinType, paymentpkg.PaymentMessageParams) error {
	return nil
}

func (v *verifiedPaymentVerifier) FetchTransaction(context.Context, iwallet.CoinType, string, string) (*iwallet.Transaction, error) {
	return &v.tx, nil
}

func (v *verifiedPaymentVerifier) FetchAndVerify(context.Context, *pb.OrderOpen, *pb.PaymentSent, string) (*contracts.VerifiedPayment, error) {
	return &contracts.VerifiedPayment{Transaction: v.tx, CoinType: iwallet.CoinType("crypto:solana:mainnet:native")}, nil
}

type capturingPaymentVerifier struct {
	params paymentpkg.PaymentMessageParams
}

func (v *capturingPaymentVerifier) ValidateMessage(_ iwallet.CoinType, params paymentpkg.PaymentMessageParams) error {
	v.params = params
	return nil
}

func (v *capturingPaymentVerifier) FetchTransaction(context.Context, iwallet.CoinType, string, string) (*iwallet.Transaction, error) {
	return &iwallet.Transaction{}, nil
}

func (v *capturingPaymentVerifier) FetchAndVerify(context.Context, *pb.OrderOpen, *pb.PaymentSent, string) (*contracts.VerifiedPayment, error) {
	return &contracts.VerifiedPayment{Transaction: iwallet.Transaction{}, CoinType: iwallet.CoinType("crypto:eip155:1:native")}, nil
}

func tenantDB(t *testing.T, shared *gorm.DB, tenantID string) database.Database {
	t.Helper()
	db, err := dbstore.NewTenantDBWithPublicData(shared, tenantID, dbstore.NewDBPublicData(shared, tenantID))
	require.NoError(t, err)
	return db
}

func testSigner(t *testing.T) (contracts.Signer, peer.ID) {
	t.Helper()
	keyPair, err := identity.GenerateKeyPair()
	require.NoError(t, err)
	identityPeerID, err := identity.PeerIDFromPublicKey(keyPair.PubKey)
	require.NoError(t, err)
	pid, err := peer.Decode(identityPeerID.String())
	require.NoError(t, err)
	return contracts.NewKeyPairSigner(keyPair, identityPeerID), pid
}

func signedOrderOpen(t *testing.T, buyerPeerID, sellerPeerID peer.ID) []byte {
	t.Helper()
	open := &pb.OrderOpen{
		BuyerID:   &pb.ID{PeerID: buyerPeerID.String()},
		Chaincode: "01020304",
		Listings: []*pb.SignedListing{
			{
				Listing: &pb.Listing{
					VendorID: &pb.ID{PeerID: sellerPeerID.String()},
					Slug:     "cotenant-solana",
					Metadata: &pb.Listing_Metadata{ContractType: pb.Listing_Metadata_PHYSICAL_GOOD},
					Item: &pb.Listing_Item{
						Title: "Co-tenant Solana item",
						Images: []*pb.Image{{
							Tiny:  "tiny",
							Small: "small",
						}},
					},
				},
			},
		},
	}
	data, err := protojson.Marshal(open)
	require.NoError(t, err)
	return data
}

func seedTenantOrder(t *testing.T, db database.Database, orderID string, role models.OrderRole, open []byte) {
	t.Helper()
	order := &models.Order{
		ID:                  models.OrderID(orderID),
		MyRole:              string(role),
		SerializedOrderOpen: open,
	}
	order.SetFSMState(models.OrderState_AWAITING_PAYMENT)
	require.NoError(t, db.Update(func(tx database.Tx) error {
		return tx.Save(order)
	}))
}

func mustFetchTenantOrder(t *testing.T, db database.Database, orderID string) *models.Order {
	t.Helper()
	var order models.Order
	require.NoError(t, db.View(func(tx database.Tx) error {
		return tx.Read().Where("id = ?", orderID).First(&order).Error
	}))
	return &order
}
