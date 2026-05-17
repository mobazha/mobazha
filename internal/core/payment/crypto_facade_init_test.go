//go:build !private_distribution

package payment

import (
	"testing"

	"github.com/mobazha/mobazha3.0/pkg/models"
	porderpb "github.com/mobazha/mobazha3.0/pkg/orders/mbzpb"
	iwallet "github.com/mobazha/mobazha3.0/pkg/wallet-interface"
)

type noopRates struct{}

func (noopRates) GetAllRates(base models.CurrencyCode, breakCache bool) (map[models.CurrencyCode]iwallet.Amount, error) {
	panic("unexpected")
}

func (noopRates) GetRate(base models.CurrencyCode, to models.CurrencyCode, breakCache bool) (iwallet.Amount, error) {
	panic("unexpected GetRate — same-currency branch should skip conversion")
}

func TestBuildInitializeEscrowDataFromOrder_SameCurrencyUsesOrderOpenNumeric(t *testing.T) {
	coin := iwallet.CoinType("crypto:eip155:1:native")
	if err := coin.ValidateCanonicalPaymentCoin(); err != nil {
		t.Skip("canonical coin unavailable in build env")
	}

	order := &models.Order{ID: models.OrderID("order.eth")}
	open := &porderpb.OrderOpen{
		Amount:      "42",
		PricingCoin: "ETH",
	}
	got, err := buildInitializeEscrowDataFromOrder(order, open, coin, "0xrefund", "", "", noopRates{})
	if err != nil {
		t.Fatal(err)
	}
	if got.Amount != 42 || got.RefundAddress != "0xrefund" || got.CoinType != coin {
		t.Fatalf("%+v", got)
	}
}
