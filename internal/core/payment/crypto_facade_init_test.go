//go:build !private_distribution

package payment

import (
	"context"
	"errors"
	"testing"

	"github.com/mobazha/mobazha3.0/pkg/core/coreiface"
	"github.com/mobazha/mobazha3.0/pkg/models"
	porderpb "github.com/mobazha/mobazha3.0/pkg/orders/mbzpb"
	iwallet "github.com/mobazha/mobazha3.0/pkg/wallet-interface"
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
	got, err := buildPaymentSetupParamsFromOrder(order, open, coin, "", "", noopRates{})
	if err != nil {
		t.Fatal(err)
	}
	if got.Amount != 42 || got.CoinType != coin {
		t.Fatalf("%+v", got)
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

	revision, err := facade.validateStorePolicyModerator(context.Background(), testStorePolicyPeerA)
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

	if _, err := facade.validateStorePolicyModerator(context.Background(), testStorePolicyPeerA); !errors.Is(err, coreiface.ErrBadRequest) {
		t.Fatalf("disabled moderator error = %v, want ErrBadRequest", err)
	}
	if _, err := facade.validateStorePolicyModerator(context.Background(), testStorePolicyPeerB); !errors.Is(err, coreiface.ErrBadRequest) {
		t.Fatalf("missing moderator error = %v, want ErrBadRequest", err)
	}
}

func TestNormalizeCryptoRefundAddress_DefaultsToPayerAddress(t *testing.T) {
	coin := iwallet.CoinType("crypto:eip155:1:native")
	if err := coin.ValidateCanonicalPaymentCoin(); err != nil {
		t.Skip("canonical coin unavailable in build env")
	}

	got, err := normalizeCryptoRefundAddress(
		coin,
		"",
		"  0xaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa  ",
	)
	if err != nil {
		t.Fatal(err)
	}
	if got != "0xaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa" {
		t.Fatalf("refund address = %q", got)
	}
}

func TestNormalizeCryptoRefundAddress_ExplicitRefundWins(t *testing.T) {
	coin := iwallet.CoinType("crypto:eip155:1:native")
	if err := coin.ValidateCanonicalPaymentCoin(); err != nil {
		t.Skip("canonical coin unavailable in build env")
	}

	got, err := normalizeCryptoRefundAddress(
		coin,
		"0xbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
		"0xaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
	)
	if err != nil {
		t.Fatal(err)
	}
	if got != "0xbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb" {
		t.Fatalf("refund address = %q", got)
	}
}
