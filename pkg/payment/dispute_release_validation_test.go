package payment

import (
	"testing"

	"github.com/mobazha/mobazha/pkg/models"
	pb "github.com/mobazha/mobazha/pkg/orders/mbzpb"
	"github.com/stretchr/testify/require"
)

func TestValidateDisputeReleaseBalanceRejectsOutputsPlusFeeAboveInputs(t *testing.T) {
	release := &pb.DisputeClose_ModeratedEscrowRelease{
		Outpoints:        []*pb.Outpoint{{FromID: []byte{0x01}, Value: "10000"}},
		BuyerAmount:      "9000",
		VendorAmount:     "0",
		ModeratorAmount:  "1000",
		TransactionFee:   "1",
		EscrowSignatures: []*pb.Signature{{Signature: []byte{0x01}}},
	}

	err := ValidateDisputeReleaseBalance(release)
	require.Error(t, err)
	require.Contains(t, err.Error(), "outputs plus fee exceed escrow inputs")
}

func TestValidateDisputeReleaseFundingRejectsOutpointValueMismatch(t *testing.T) {
	const txID = "d61914815f5ee984dde1faaa84de33a3fd40a8ecead66a754eca5f2d40704db8"
	outpointID, ok := UTXOOutpointID(txID, 0)
	require.True(t, ok)

	paymentSent := &pb.PaymentSent{
		Coin:               "crypto:bitcoincash:mainnet:native",
		ToAddress:          "pz4n8gcqhdsg80ap2qdt79hfz585qetc45nec0mp76",
		Amount:             "22253",
		ConfirmationPolicy: models.PaymentConfirmationPolicyMempoolAccepted,
		FundingFacts: []*pb.PaymentSent_FundingFact{{
			Id:           "funded-output",
			TxHash:       txID,
			TxHashSource: models.PaymentTxHashSourceChainTx,
			EventIndex:   0,
			ToAddress:    "bitcoincash:pz4n8gcqhdsg80ap2qdt79hfz585qetc45nec0mp76",
			Amount:       "22253",
			Status:       models.PaymentObservationStatusConfirmed,
		}},
	}
	release := &pb.DisputeClose_ModeratedEscrowRelease{
		Outpoints:        []*pb.Outpoint{{FromID: outpointID, Value: "58654"}},
		BuyerAmount:      "57097",
		VendorAmount:     "0",
		ModeratorAmount:  "1165",
		TransactionFee:   "392",
		EscrowSignatures: []*pb.Signature{{Signature: []byte{0x01}}},
	}

	err := ValidateDisputeReleaseFunding(release, paymentSent)
	require.Error(t, err)
	require.Contains(t, err.Error(), "does not match funded value 22253")
}

func TestValidateDisputeReleaseFundingAcceptsCanonicalFundingOutpoint(t *testing.T) {
	const txID = "d61914815f5ee984dde1faaa84de33a3fd40a8ecead66a754eca5f2d40704db8"
	outpointID, ok := UTXOOutpointID(txID, 0)
	require.True(t, ok)

	paymentSent := &pb.PaymentSent{
		Coin:               "crypto:bitcoincash:mainnet:native",
		ToAddress:          "pz4n8gcqhdsg80ap2qdt79hfz585qetc45nec0mp76",
		Amount:             "22253",
		ConfirmationPolicy: models.PaymentConfirmationPolicyMempoolAccepted,
		FundingFacts: []*pb.PaymentSent_FundingFact{{
			Id:           "funded-output",
			TxHash:       txID,
			TxHashSource: models.PaymentTxHashSourceChainTx,
			EventIndex:   0,
			ToAddress:    "bitcoincash:pz4n8gcqhdsg80ap2qdt79hfz585qetc45nec0mp76",
			Amount:       "22253",
			Status:       models.PaymentObservationStatusConfirmed,
		}},
	}
	release := &pb.DisputeClose_ModeratedEscrowRelease{
		Outpoints:        []*pb.Outpoint{{FromID: outpointID, Value: "22253"}},
		BuyerAmount:      "21000",
		VendorAmount:     "0",
		ModeratorAmount:  "1000",
		TransactionFee:   "253",
		EscrowSignatures: []*pb.Signature{{Signature: []byte{0x01}}},
	}

	require.NoError(t, ValidateDisputeReleaseFunding(release, paymentSent))
}
