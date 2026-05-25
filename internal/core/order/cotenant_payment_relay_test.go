//go:build !private_distribution

package order

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/mobazha/mobazha3.0/internal/database/dbstore"
	"github.com/mobazha/mobazha3.0/internal/orders"
	"github.com/mobazha/mobazha3.0/pkg/contracts"
	"github.com/mobazha/mobazha3.0/pkg/database"
	"github.com/mobazha/mobazha3.0/pkg/database/sqlitedialect"
	"github.com/mobazha/mobazha3.0/pkg/events"
	"github.com/mobazha/mobazha3.0/pkg/identity"
	"github.com/mobazha/mobazha3.0/pkg/models"
	npb "github.com/mobazha/mobazha3.0/pkg/net/mbzpb"
	pb "github.com/mobazha/mobazha3.0/pkg/orders/mbzpb"
	paymentpkg "github.com/mobazha/mobazha3.0/pkg/payment"
	iwallet "github.com/mobazha/mobazha3.0/pkg/wallet-interface"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/encoding/protojson"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func TestRelayPaymentToCounterparty_DeliversVerifiedPaymentToCoTenant(t *testing.T) {
	shared, err := gorm.Open(sqlitedialect.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err)
	require.NoError(t, shared.AutoMigrate(&models.Order{}))

	buyerDB := tenantDB(t, shared, "tenant-buyer")
	sellerDB := tenantDB(t, shared, "tenant-seller")

	buyerSigner, buyerPeerID := testSigner(t)
	_, sellerPeerID := testSigner(t)
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

	orderID := "cotenant-solana-payment"
	open := signedOrderOpen(t, buyerPeerID, sellerPeerID)
	seedTenantOrder(t, buyerDB, orderID, models.RoleBuyer, open)
	seedTenantOrder(t, sellerDB, orderID, models.RoleVendor, open)

	svc.RelayPaymentToCounterparty(context.Background(), orderID, sellerPeerID, &models.PaymentData{
		OrderID:       orderID,
		TransactionID: strings.Repeat("a", 64),
		Coin:          iwallet.CoinType("crypto:solana:mainnet:native"),
		Method:        pb.PaymentSent_DIRECT,
		Amount:        1_000_000,
		PayerAddress:  "payer-solana-address",
		ToAddress:     "escrow-solana-address",
		Timestamp:     time.Now().UTC(),
	})

	var sellerOrder models.Order
	require.NoError(t, sellerDB.View(func(tx database.Tx) error {
		return tx.Read().Where("id = ?", orderID).First(&sellerOrder).Error
	}))
	require.NotNil(t, sellerOrder.SerializedPaymentSent)
	require.True(t, sellerOrder.IsPaymentVerified())
	require.Equal(t, models.OrderState_PENDING, sellerOrder.State)
}

func TestProcessOrderPayment_RelaysVerifiedPaymentWhenPersistFails(t *testing.T) {
	shared, err := gorm.Open(sqlitedialect.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err)
	require.NoError(t, shared.AutoMigrate(&models.Order{}))

	buyerDB := tenantDB(t, shared, "tenant-buyer")
	sellerDB := tenantDB(t, shared, "tenant-seller")

	buyerSigner, buyerPeerID := testSigner(t)
	_, sellerPeerID := testSigner(t)
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
	tx := iwallet.Transaction{
		ID:     iwallet.TransactionID(strings.Repeat("b", 64)),
		Value:  iwallet.NewAmount(1_000_000),
		Height: 1,
	}
	svc.SetPaymentVerifier(&verifiedPaymentVerifier{tx: tx})

	err = svc.ProcessOrderPayment(context.Background(), &models.PaymentData{
		OrderID:        orderID,
		TransactionID:  tx.ID.String(),
		Coin:           iwallet.CoinType("crypto:solana:mainnet:native"),
		Method:         pb.PaymentSent_DIRECT,
		Amount:         1_000_000,
		PayerAddress:   "payer-solana-address",
		ToAddress:      "escrow-solana-address",
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
					Item:     &pb.Listing_Item{Title: "Co-tenant Solana item"},
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
