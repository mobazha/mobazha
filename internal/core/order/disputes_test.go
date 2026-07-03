package order

import (
	"encoding/hex"
	"errors"
	"testing"

	"github.com/mobazha/mobazha/pkg/core/coreiface"
	"github.com/mobazha/mobazha/pkg/database"
	"github.com/mobazha/mobazha/pkg/models"
	pb "github.com/mobazha/mobazha/pkg/orders/mbzpb"
	iwallet "github.com/mobazha/mobazha/pkg/wallet-interface"
	"github.com/stretchr/testify/require"
)

func TestReleaseFundsRejectsModeratorRole(t *testing.T) {
	svc := newTestOrderAppService(t, OrderAppServiceConfig{})
	orderID := models.OrderID("moderator-release-order")

	require.NoError(t, svc.db.Update(func(tx database.Tx) error {
		return tx.Save(&models.Order{
			ID:     orderID,
			MyRole: string(models.RoleModerator),
		})
	}))

	err := svc.ReleaseFunds(orderID, iwallet.TransactionID(""), nil)
	require.Error(t, err)
	require.True(t, errors.Is(err, coreiface.ErrBadRequest))
	require.Contains(t, err.Error(), "moderator must resolve disputes via close dispute")
}

func TestDisputeReleaseOutpointsFromFundingFacts_UsesCanonicalUTXOOutpoint(t *testing.T) {
	const (
		txHash         = "f98cd55acb5a344c6e6fa0b192a125656d8c50d0fba125f72deb798b7ddfd8ff"
		paymentAddress = "bitcoincash:ppu9yncdpjgwmq8h5khefmkhrat6pdp08sqsjd0mrc"
	)

	paymentSent := &pb.PaymentSent{
		Coin:      "crypto:bitcoincash:mainnet:native",
		ToAddress: paymentAddress,
		FundingFacts: []*pb.PaymentSent_FundingFact{{
			Id:           "obs-1",
			TxHash:       txHash,
			TxHashSource: models.PaymentTxHashSourceChainTx,
			EventIndex:   0,
			ToAddress:    paymentAddress,
			Amount:       "16522",
			Status:       models.PaymentObservationStatusConfirmed,
		}},
	}

	outpoints := disputeReleaseOutpointsFromFundingFacts(
		paymentSent,
		iwallet.CoinType("crypto:bitcoincash:mainnet:native"),
		paymentAddress,
	)

	require.Len(t, outpoints, 1)
	require.Equal(t, txHash, outpoints[0].Txid)
	require.Equal(t, "16522", outpoints[0].Value)
	require.Equal(t,
		"ffd8df7d8b79eb2df725a1fbd0508c6d6525a192b1a06f6e4c345acb5ad58cf900000000",
		hex.EncodeToString(outpoints[0].FromID),
	)
}
