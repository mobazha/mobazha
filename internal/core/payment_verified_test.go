//go:build !private_distribution

package core

import (
	"testing"
	"time"

	"github.com/mobazha/mobazha3.0/pkg/orders/mbzpb"
	"github.com/mobazha/mobazha3.0/pkg/payment"
	iwallet "github.com/mobazha/mobazha3.0/pkg/wallet-interface"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestPaymentDataFromVerifiedPaymentSent_PreservesManagedEscrowFields(t *testing.T) {
	ts := time.Unix(1710000000, 0).UTC()
	ps := &mbzpb.PaymentSent{
		TransactionID:       "0xtx",
		Coin:                "crypto:eip155:1:native",
		SettlementSpec:      payment.NewManagedEscrowSpec(false).ToPaymentSent(),
		ContractAddress:     "0xcontract",
		PayerAddress:        "0xbuyer",
		Moderator:           "12D3KooWMod",
		ModeratorAddress:    "0xmod",
		Amount:              "123",
		ToAddress:           "0xmanagedescrow",
		Script:              "deadbeef",
		EscrowTimeoutHours:  720,
		EscrowReleaseFee:    "7",
		PlatformAmount:      "9",
		PlatformAddr:        "0xplatform",
		RefundAddress:       "0xrefund",
		PaymentTokenAddress: "0xtoken",
		BuyerReceiveAddress: "0xbuyerrecv",
		Timestamp:           timestamppb.New(ts),
		PaymentMethod: &mbzpb.PaymentSent_PaymentMethod{
			Type:  "card",
			Brand: "visa",
			Last4: "4242",
		},
	}

	pd := paymentDataFromVerifiedPaymentSent("ord-1", ps)
	if pd == nil {
		t.Fatal("payment data is nil")
	}
	if pd.OrderID != "ord-1" || pd.TransactionID != "0xtx" {
		t.Fatalf("unexpected ids: %+v", pd)
	}
	if pd.Coin != iwallet.CoinType("crypto:eip155:1:native") || pd.Method != mbzpb.PaymentSent_CANCELABLE {
		t.Fatalf("unexpected coin/method: coin=%s method=%v", pd.Coin, pd.Method)
	}
	if pd.ToAddress != "0xmanagedescrow" || pd.PayerAddress != "0xbuyer" || pd.RefundAddress != "0xrefund" {
		t.Fatalf("critical payment routing fields lost: %+v", pd)
	}
	if pd.ContractAddress != "0xcontract" || pd.ModeratorAddress != "0xmod" || pd.PaymentTokenAddress != "0xtoken" {
		t.Fatalf("auxiliary safe fields lost: %+v", pd)
	}
	if pd.Amount != 123 || pd.UnlockHours != 720 || !pd.Timestamp.Equal(ts) {
		t.Fatalf("numeric/timestamp fields wrong: amount=%d unlock=%d ts=%s", pd.Amount, pd.UnlockHours, pd.Timestamp)
	}
	if pd.PaymentMethod.Type != "card" || pd.PaymentMethod.Brand != "visa" || pd.PaymentMethod.Last4 != "4242" {
		t.Fatalf("payment method lost: %+v", pd.PaymentMethod)
	}
}
