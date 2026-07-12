package payment

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/mobazha/mobazha/internal/database/dbstore"
	"github.com/mobazha/mobazha/pkg/contracts"
	"github.com/mobazha/mobazha/pkg/core/coreiface"
	"github.com/mobazha/mobazha/pkg/database/sqlitedialect"
	"github.com/mobazha/mobazha/pkg/models"
	porderpb "github.com/mobazha/mobazha/pkg/orders/mbzpb"
	iwallet "github.com/mobazha/mobazha/pkg/wallet-interface"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

const (
	testStorePolicyPeerA = "QmYwAPJzv5CZsnA625s3Xf2nemtYgPpHdWEz79ojWnPbdG"
	testStorePolicyPeerB = "12D3KooWHHzSeKaY8xuZVzkLbKFfvNgPPeKhFBGrMbNbXRwuFCA5"
)

type fakeCryptoStorePolicy struct {
	policy *models.StorePolicy
}

func (s fakeCryptoStorePolicy) GetPolicy(context.Context) (*models.StorePolicy, error) {
	return s.policy, nil
}

func (s fakeCryptoStorePolicy) GetPublishedPolicy(context.Context) (*models.StorePolicyPublic, error) {
	return nil, nil
}

func (s fakeCryptoStorePolicy) ReplaceModerators(context.Context, *uint64, []models.StorePolicyModeratorInput) (*models.StorePolicy, error) {
	return nil, nil
}

func (s fakeCryptoStorePolicy) UpsertModerator(context.Context, *uint64, models.StorePolicyModeratorInput) (*models.StorePolicy, error) {
	return nil, nil
}

func (s fakeCryptoStorePolicy) RemoveModerator(context.Context, *uint64, string) (*models.StorePolicy, error) {
	return nil, nil
}

type noopRates struct{}

func (noopRates) GetAllRates(base models.CurrencyCode, breakCache bool) (map[models.CurrencyCode]iwallet.Amount, error) {
	panic("unexpected")
}

func (noopRates) GetRate(base models.CurrencyCode, to models.CurrencyCode, breakCache bool) (iwallet.Amount, error) {
	panic("unexpected GetRate — same-currency branch should skip conversion")
}

func (noopRates) LastUpdated(models.CurrencyCode) time.Time {
	panic("unexpected LastUpdated")
}

func TestBuildPaymentSetupParamsFromOrder_SameCurrencyUsesOrderOpenNumeric(t *testing.T) {
	coin := iwallet.CoinType("crypto:eip155:1:native")
	if err := coin.ValidateCanonicalPaymentCoin(); err != nil {
		t.Skip("canonical coin unavailable in build env")
	}

	order := &models.Order{ID: models.OrderID("order.eth")}
	open := &porderpb.OrderOpen{
		Amount:      "42",
		PricingCoin: "ETH",
	}
	got, err := buildPaymentSetupParamsFromOrder(order, open, coin, "", "", "", "", noopRates{})
	if err != nil {
		t.Fatal(err)
	}
	if got.Amount != 42 || got.CoinType != coin {
		t.Fatalf("%+v", got)
	}
}

func TestBuildPaymentSetupParamsFromOrder_ForwardsRefundAddress(t *testing.T) {
	coin := iwallet.CoinType("crypto:solana:mainnet:native")
	if err := coin.ValidateCanonicalPaymentCoin(); err != nil {
		t.Skip("canonical coin unavailable in build env")
	}

	order := &models.Order{ID: models.OrderID("order.sol")}
	open := &porderpb.OrderOpen{
		Amount:      "42",
		PricingCoin: "SOL",
	}
	got, err := buildPaymentSetupParamsFromOrder(
		order,
		open,
		coin,
		"payer-address",
		"refund-address",
		"",
		"",
		noopRates{},
	)
	if err != nil {
		t.Fatal(err)
	}
	if got.PayerAddress != "payer-address" {
		t.Fatalf("payer address = %q", got.PayerAddress)
	}
	if got.RefundAddress != "refund-address" {
		t.Fatalf("refund address = %q", got.RefundAddress)
	}
}

func TestCryptoPaymentFacade_ValidateStorePolicyModeratorAcceptsEnabledModerator(t *testing.T) {
	facade := &CryptoPaymentFacade{
		storePolicy: fakeCryptoStorePolicy{policy: &models.StorePolicy{
			Revision: 11,
			Moderators: []models.StoreModerator{
				{PeerID: testStorePolicyPeerA, Enabled: true},
			},
		}},
	}

	revision, err := facade.validateStorePolicyModerator(context.Background(), "", nil, testStorePolicyPeerA)
	if err != nil {
		t.Fatal(err)
	}
	if revision != 11 {
		t.Fatalf("revision = %d, want 11", revision)
	}
}

func TestCryptoPaymentFacade_ValidateStorePolicyModeratorRejectsDisabledOrMissingModerator(t *testing.T) {
	facade := &CryptoPaymentFacade{
		storePolicy: fakeCryptoStorePolicy{policy: &models.StorePolicy{
			Revision: 11,
			Moderators: []models.StoreModerator{
				{PeerID: testStorePolicyPeerA, Enabled: false},
			},
		}},
	}

	if _, err := facade.validateStorePolicyModerator(context.Background(), "", nil, testStorePolicyPeerA); !errors.Is(err, coreiface.ErrBadRequest) {
		t.Fatalf("disabled moderator error = %v, want ErrBadRequest", err)
	}
	if _, err := facade.validateStorePolicyModerator(context.Background(), "", nil, testStorePolicyPeerB); !errors.Is(err, coreiface.ErrBadRequest) {
		t.Fatalf("missing moderator error = %v, want ErrBadRequest", err)
	}
}

func TestCryptoPaymentFacade_ValidateStorePolicyModeratorResolvesSellerTenantFromOrderOpen(t *testing.T) {
	shared, err := gorm.Open(sqlitedialect.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := shared.AutoMigrate(&models.StorePolicy{}, &models.StoreModerator{}); err != nil {
		t.Fatal(err)
	}
	if err := shared.Exec(`CREATE TABLE account_peer_ids (
		account_id varchar(128) NOT NULL,
		peer_id varchar(128) NOT NULL PRIMARY KEY
	)`).Error; err != nil {
		t.Fatal(err)
	}

	const sellerTenantID = "tenant-seller"
	const sellerPeerID = testStorePolicyPeerB
	if err := shared.Exec(
		"INSERT INTO account_peer_ids (account_id, peer_id) VALUES (?, ?)",
		sellerTenantID,
		sellerPeerID,
	).Error; err != nil {
		t.Fatal(err)
	}
	if err := shared.Create(&models.StorePolicy{TenantID: sellerTenantID, Revision: 7}).Error; err != nil {
		t.Fatal(err)
	}
	if err := shared.Create(&models.StoreModerator{
		TenantID: sellerTenantID,
		PeerID:   testStorePolicyPeerA,
		Enabled:  true,
	}).Error; err != nil {
		t.Fatal(err)
	}

	buyerDB, err := dbstore.NewTenantDBWithPublicData(shared, "tenant-buyer", dbstore.NewDBPublicData(shared, "tenant-buyer"))
	if err != nil {
		t.Fatal(err)
	}
	facade := &CryptoPaymentFacade{db: buyerDB}
	orderOpen := &porderpb.OrderOpen{
		Listings: []*porderpb.SignedListing{
			{Listing: &porderpb.Listing{VendorID: &porderpb.ID{PeerID: sellerPeerID}}},
		},
	}

	revision, err := facade.validateStorePolicyModerator(context.Background(), "order-before-vendor-mirror", orderOpen, testStorePolicyPeerA)
	if err != nil {
		t.Fatal(err)
	}
	if revision != 7 {
		t.Fatalf("revision = %d, want 7", revision)
	}
}

func TestResolveCreateSessionRefundAddress_DefaultsToPayerAddress(t *testing.T) {
	coin := iwallet.CoinType("crypto:eip155:1:native")
	if err := coin.ValidateCanonicalPaymentCoin(); err != nil {
		t.Skip("canonical coin unavailable in build env")
	}

	got, err := resolveCreateSessionRefundAddress(
		coin,
		contracts.CreatePaymentSessionRequest{
			PayerAddress: "  0xaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa  ",
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if got != "0xaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa" {
		t.Fatalf("refund address = %q", got)
	}
}

func TestResolveCreateSessionRefundAddress_ExplicitRefundWins(t *testing.T) {
	coin := iwallet.CoinType("crypto:eip155:1:native")
	if err := coin.ValidateCanonicalPaymentCoin(); err != nil {
		t.Skip("canonical coin unavailable in build env")
	}

	got, err := resolveCreateSessionRefundAddress(
		coin,
		contracts.CreatePaymentSessionRequest{
			RefundAddress: "0xbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
			PayerAddress:  "0xaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if got != "0xbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb" {
		t.Fatalf("refund address = %q", got)
	}
}

func TestResolveCreateSessionRefundAddress_CustodialRequiresExplicitRefund(t *testing.T) {
	coin := iwallet.CoinType("crypto:eip155:1:native")
	if err := coin.ValidateCanonicalPaymentCoin(); err != nil {
		t.Skip("canonical coin unavailable in build env")
	}

	_, err := resolveCreateSessionRefundAddress(coin, contracts.CreatePaymentSessionRequest{
		PayerAddress:     "0xaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		PayFromCustodial: true,
	})
	if !errors.Is(err, coreiface.ErrBadRequest) || !errors.Is(err, models.ErrRefundAddressRequired) {
		t.Fatalf("error = %v, want bad request + refund address required", err)
	}
}

func TestStandardOrderUTXOAuthorizationEligible_RequiresSameCurrencyNativeUTXO(t *testing.T) {
	bitcoin, ok := iwallet.CanonicalNativeCoinType(iwallet.ChainBitcoin)
	if !ok {
		t.Fatal("canonical Bitcoin rail unavailable")
	}
	if !standardOrderUTXOAuthorizationEligible(bitcoin, &porderpb.OrderOpen{PricingCoin: "BTC"}) {
		t.Fatal("same-currency native Bitcoin order should use settlement authorization")
	}
	if standardOrderUTXOAuthorizationEligible(bitcoin, &porderpb.OrderOpen{PricingCoin: "USD"}) {
		t.Fatal("cross-currency order must remain outside the first authorization scope")
	}
	ethereum, ok := iwallet.CanonicalNativeCoinType(iwallet.ChainEthereum)
	if !ok {
		t.Fatal("canonical Ethereum rail unavailable")
	}
	if standardOrderUTXOAuthorizationEligible(ethereum, &porderpb.OrderOpen{PricingCoin: "ETH"}) {
		t.Fatal("EVM order must remain disabled until its attempt owner projector is implemented")
	}
}
