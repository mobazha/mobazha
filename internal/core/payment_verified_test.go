//go:build !private_distribution

package core

import (
	"context"
	"encoding/hex"
	"fmt"
	"testing"
	"time"

	hd "github.com/btcsuite/btcd/btcutil/hdkeychain"
	corepayment "github.com/mobazha/mobazha3.0/internal/core/payment"
	"github.com/mobazha/mobazha3.0/pkg/models"
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

func TestHydratePaymentDataFromObservedTransaction_PreservesUTXOOutpoint(t *testing.T) {
	outpoint, err := hex.DecodeString("000102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f01000000")
	if err != nil {
		t.Fatal(err)
	}
	order := &models.Order{ID: models.OrderID("ord-utxo")}
	if err := order.PutTransaction(iwallet.Transaction{
		ID:     iwallet.TransactionID("funding-tx"),
		Height: 123,
		To: []iwallet.SpendInfo{{
			ID:      outpoint,
			Address: iwallet.NewAddress("payment-address", iwallet.CoinType("crypto:bip122:000000000019d6689c085ae165831e93:native")),
			Amount:  iwallet.NewAmount(30164),
		}},
	}); err != nil {
		t.Fatal(err)
	}

	pd := &models.PaymentData{
		TransactionID: "funding-tx",
		ToAddress:     "payment-address",
	}
	hydratePaymentDataFromObservedTransaction(pd, order)

	if got := hex.EncodeToString(pd.ToID); got != hex.EncodeToString(outpoint) {
		t.Fatalf("ToID = %s, want %s", got, hex.EncodeToString(outpoint))
	}
	if pd.BlockHeight != 123 {
		t.Fatalf("BlockHeight = %d, want 123", pd.BlockHeight)
	}
}

func TestHydratePaymentDataFromTransaction_SelectsUniqueAddressOutput(t *testing.T) {
	changeOutpoint, err := hex.DecodeString("aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa00000000")
	if err != nil {
		t.Fatal(err)
	}
	paymentOutpoint, err := hex.DecodeString("bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb01000000")
	if err != nil {
		t.Fatal(err)
	}
	coin := iwallet.CoinType("crypto:bip122:000000000019d6689c085ae165831e93:native")
	tx := iwallet.Transaction{
		ID:     iwallet.TransactionID("funding-tx"),
		Height: 777,
		To: []iwallet.SpendInfo{
			{
				ID:      changeOutpoint,
				Address: iwallet.NewAddress("change-address", coin),
				Amount:  iwallet.NewAmount(499969867),
			},
			{
				ID:      paymentOutpoint,
				Address: iwallet.NewAddress("payment-address", coin),
				Amount:  iwallet.NewAmount(30133),
			},
		},
	}
	pd := &models.PaymentData{
		Coin:      coin,
		ToAddress: "payment-address",
	}

	hydratePaymentDataFromTransaction(pd, tx)

	if got := hex.EncodeToString(pd.ToID); got != hex.EncodeToString(paymentOutpoint) {
		t.Fatalf("ToID = %s, want %s", got, hex.EncodeToString(paymentOutpoint))
	}
	if pd.TransactionID != "funding-tx" {
		t.Fatalf("TransactionID = %s, want funding-tx", pd.TransactionID)
	}
	if pd.BlockHeight != 777 {
		t.Fatalf("BlockHeight = %d, want 777", pd.BlockHeight)
	}
	if !paymentDataRequiresUTXOOutpoint(pd) {
		t.Fatal("expected BTC payment data to require an outpoint")
	}
}

func TestVerifiedTransactionFromPaymentSentFundingFactsAggregatesUTXO(t *testing.T) {
	coin := iwallet.CoinType("crypto:bip122:000000000019d6689c085ae165831e93:native")
	paymentAddress := "bcrt1qfundingfacts"
	outpointA, err := hex.DecodeString("aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa01000000")
	if err != nil {
		t.Fatal(err)
	}
	outpointB, err := hex.DecodeString("bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb02000000")
	if err != nil {
		t.Fatal(err)
	}

	node := &MobazhaNode{
		appServices: appServices{
			paymentVerificationService: corepayment.NewPaymentVerificationService(nil, paymentVerifiedFundingWalletOperator{
				wallet: paymentVerifiedFundingWallet{txs: map[iwallet.TransactionID]iwallet.Transaction{
					"btc-tx-a": {
						ID: iwallet.TransactionID("btc-tx-a"),
						To: []iwallet.SpendInfo{{
							ID:      outpointA,
							Address: iwallet.NewAddress(paymentAddress, coin),
							Amount:  iwallet.NewAmount(400),
						}},
					},
					"btc-tx-b": {
						ID: iwallet.TransactionID("btc-tx-b"),
						To: []iwallet.SpendInfo{
							{
								ID:      []byte("change"),
								Address: iwallet.NewAddress("change-address", coin),
								Amount:  iwallet.NewAmount(100),
							},
							{
								ID:      outpointB,
								Address: iwallet.NewAddress(paymentAddress, coin),
								Amount:  iwallet.NewAmount(600),
							},
						},
					},
				}},
			}, nil),
		},
	}

	tx, ok := node.verifiedTransactionFromPaymentSent(context.Background(), &mbzpb.PaymentSent{
		TransactionID:  "btc-tx-b",
		Coin:           string(coin),
		ToAddress:      paymentAddress,
		Amount:         "1000",
		SettlementSpec: payment.NewUTXOSpec(false).ToPaymentSent(),
		FundingFacts: []*mbzpb.PaymentSent_FundingFact{
			{
				Id:             "btc-obs-a",
				ChainNamespace: "bip122",
				ChainReference: "000000000019d6689c085ae165831e93",
				TxHash:         "btc-tx-a",
				TxHashSource:   models.PaymentTxHashSourceChainTx,
				EventIndex:     0,
				EventType:      models.PaymentEventUTXOFunding,
				ToAddress:      paymentAddress,
				Amount:         "400",
				Status:         models.PaymentObservationStatusConfirmed,
			},
			{
				Id:             "btc-obs-b",
				ChainNamespace: "bip122",
				ChainReference: "000000000019d6689c085ae165831e93",
				TxHash:         "btc-tx-b",
				TxHashSource:   models.PaymentTxHashSourceChainTx,
				EventIndex:     1,
				EventType:      models.PaymentEventUTXOFunding,
				ToAddress:      paymentAddress,
				Amount:         "600",
				Status:         models.PaymentObservationStatusConfirmed,
			},
		},
	})
	if !ok || tx == nil {
		t.Fatal("expected funding facts to resolve an aggregate transaction")
	}
	if tx.Value.Int64() != 1000 || len(tx.To) != 2 {
		t.Fatalf("unexpected aggregate transaction: value=%s outputs=%d", tx.Value.String(), len(tx.To))
	}
	if hex.EncodeToString(tx.To[0].ID) != hex.EncodeToString(outpointA) || hex.EncodeToString(tx.To[1].ID) != hex.EncodeToString(outpointB) {
		t.Fatalf("aggregate outputs lost facts: got %x / %x", tx.To[0].ID, tx.To[1].ID)
	}
}

type paymentVerifiedFundingWalletOperator struct {
	wallet iwallet.Wallet
}

func (o paymentVerifiedFundingWalletOperator) WalletForCurrencyCode(string) (iwallet.Wallet, error) {
	return o.wallet, nil
}
func (o paymentVerifiedFundingWalletOperator) WalletForChain(iwallet.ChainType) (iwallet.Wallet, bool) {
	return o.wallet, true
}
func (paymentVerifiedFundingWalletOperator) SupportedChains() []iwallet.ChainType { return nil }
func (paymentVerifiedFundingWalletOperator) Start() error                         { return nil }
func (paymentVerifiedFundingWalletOperator) Close() error                         { return nil }

type paymentVerifiedFundingWallet struct {
	txs map[iwallet.TransactionID]iwallet.Transaction
}

func (w paymentVerifiedFundingWallet) WalletExists() bool { return true }
func (w paymentVerifiedFundingWallet) CreateWallet(hd.ExtendedKey, time.Time) error {
	return nil
}
func (w paymentVerifiedFundingWallet) OpenWallet() error  { return nil }
func (w paymentVerifiedFundingWallet) CloseWallet() error { return nil }
func (w paymentVerifiedFundingWallet) Begin() (iwallet.Tx, error) {
	return nil, fmt.Errorf("not implemented")
}
func (w paymentVerifiedFundingWallet) BlockchainInfo() (iwallet.BlockInfo, error) {
	return iwallet.BlockInfo{}, nil
}
func (w paymentVerifiedFundingWallet) CoinCategory() iwallet.CoinCategory {
	return iwallet.CoinCategoryBitcoin
}
func (w paymentVerifiedFundingWallet) IsTestnet() bool { return true }
func (w paymentVerifiedFundingWallet) ValidateAddress(iwallet.Address) error {
	return nil
}
func (w paymentVerifiedFundingWallet) GetTransaction(id iwallet.TransactionID, _ iwallet.CoinType) (*iwallet.Transaction, error) {
	tx, ok := w.txs[id]
	if !ok {
		return nil, fmt.Errorf("missing tx %s", id)
	}
	return &tx, nil
}
