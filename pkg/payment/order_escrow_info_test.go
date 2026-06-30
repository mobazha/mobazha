package payment

import (
	"encoding/hex"
	"testing"

	gosolana "github.com/gagliardetto/solana-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	pb "github.com/mobazha/mobazha3.0/pkg/orders/mbzpb"
	iwallet "github.com/mobazha/mobazha3.0/pkg/wallet-interface"
)

func TestOrderEscrowInfo_SolanaCancelable_ProjectsSharedSemantics(t *testing.T) {
	buyer := gosolana.NewWallet()
	seller := gosolana.NewWallet()
	payer := gosolana.NewWallet()
	coin, err := iwallet.RequireCanonicalNativeCoinType(iwallet.ChainSolana)
	require.NoError(t, err)
	chainCode := []byte("12345678901234567890")

	info, err := OrderEscrowInfo(
		testEscrowOrder(buyer.PublicKey().Bytes(), seller.PublicKey().Bytes()),
		&pb.PaymentSent{
			Coin:               string(coin),
			Amount:             "42",
			Chaincode:          hex.EncodeToString(chainCode),
			PayerAddress:       payer.PublicKey().String(),
			ContractAddress:    gosolana.NewWallet().PublicKey().String(),
			EscrowTimeoutHours: 24,
			SettlementSpec:     NewSolanaEscrowSpec(false).ToPaymentSent(),
		},
		true,
	)
	require.NoError(t, err)
	assert.Equal(t, uint8(1), info.RequiredSignatures)
	assert.Equal(t, buyer.PublicKey().String(), info.BuyerAddress)
	assert.Equal(t, seller.PublicKey().String(), info.SellerAddress)
	assert.Equal(t, payer.PublicKey().String(), info.PayerAddress)
	assert.Equal(t, buyer.PublicKey().String(), info.RefundAddress)
	assert.Equal(t, uint64(42), info.Amount)
	assert.Equal(t, uint64(24), info.UnlockHours)
	assert.Equal(t, [20]byte(chainCode), info.UniqueId)
	assert.True(t, info.Testnet)
}

func TestOrderEscrowInfo_ShortChaincode_ReturnsError(t *testing.T) {
	wallet := gosolana.NewWallet()
	coin, err := iwallet.RequireCanonicalNativeCoinType(iwallet.ChainSolana)
	require.NoError(t, err)

	_, err = OrderEscrowInfo(
		testEscrowOrder(wallet.PublicKey().Bytes(), wallet.PublicKey().Bytes()),
		&pb.PaymentSent{
			Coin:           string(coin),
			Chaincode:      "0102",
			PayerAddress:   wallet.PublicKey().String(),
			SettlementSpec: NewSolanaEscrowSpec(false).ToPaymentSent(),
		},
		false,
	)
	require.ErrorContains(t, err, "at least 20 decoded bytes")
}

func TestOrderEscrowInfo_DirectPayment_ReturnsEmptyProjection(t *testing.T) {
	info, err := OrderEscrowInfo(
		testEscrowOrder(nil, nil),
		&pb.PaymentSent{SettlementSpec: NewDirectSpec().ToPaymentSent()},
		false,
	)
	require.NoError(t, err)
	assert.Equal(t, iwallet.EscrowInfo{}, info)
}

func TestOrderEscrowInfo_IncompleteOrder_ReturnsError(t *testing.T) {
	_, err := OrderEscrowInfo(&pb.OrderOpen{}, &pb.PaymentSent{}, false)
	require.ErrorContains(t, err, "complete order and payment messages")
}

func testEscrowOrder(buyerSolana, sellerSolana []byte) *pb.OrderOpen {
	return &pb.OrderOpen{
		BuyerID: &pb.ID{Pubkeys: &pb.ID_Pubkeys{Solana: buyerSolana}},
		Listings: []*pb.SignedListing{{
			Listing: &pb.Listing{VendorID: &pb.ID{Pubkeys: &pb.ID_Pubkeys{Solana: sellerSolana}}},
		}},
	}
}
