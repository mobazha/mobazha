//go:build !private_distribution

package payment

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/mobazha/mobazha3.0/pkg/database"
	"github.com/mobazha/mobazha3.0/pkg/models"
	pb "github.com/mobazha/mobazha3.0/pkg/orders/mbzpb"
	iwallet "github.com/mobazha/mobazha3.0/pkg/wallet-interface"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestUtxoObservationChainRef_CanonicalAndLegacy(t *testing.T) {
	t.Parallel()

	cases := []struct {
		coin      iwallet.CoinType
		namespace string
		chainRef  string
	}{
		{
			coin:      iwallet.CoinType("crypto:bip122:000000000019d6689c085ae165831e93:native"),
			namespace: "bip122",
			chainRef:  "000000000019d6689c085ae165831e93",
		},
		{
			coin:      iwallet.CoinType("btc"),
			namespace: "bip122",
			chainRef:  "000000000019d6689c085ae165831e93",
		},
		{
			coin:      iwallet.CoinType("crypto:bitcoincash:mainnet:native"),
			namespace: "bitcoincash",
			chainRef:  "mainnet",
		},
		{
			coin:      iwallet.CoinType("bch"),
			namespace: "bitcoincash",
			chainRef:  "mainnet",
		},
		{
			coin:      iwallet.CoinType("crypto:zcash:mainnet:native"),
			namespace: "zcash",
			chainRef:  "mainnet",
		},
		{
			coin:      iwallet.CoinType("ltc"),
			namespace: "bip122",
			chainRef:  "12a765e31ffd4059bada1e25190f6e98",
		},
	}

	for _, tc := range cases {
		t.Run(string(tc.coin), func(t *testing.T) {
			ns, ref, ok := utxoObservationChainRef(tc.coin)
			require.True(t, ok)
			require.Equal(t, tc.namespace, ns)
			require.Equal(t, tc.chainRef, ref)
		})
	}
}

func TestObservationDispatcher_HasAggregator(t *testing.T) {
	t.Parallel()

	withAgg := NewObservationDispatcher(newFakeObsRepo(), &fakeAggregator{}, &fakeTenantResolver{}, "worker")
	require.True(t, withAgg.HasAggregator())
}

func TestBuyerUTXOPaymentContributedToPaymentSent(t *testing.T) {
	t.Parallel()

	paymentSentAt := time.Date(2026, 5, 29, 13, 19, 42, 0, time.UTC)
	repo := newFakeObsRepo()
	svc := &PaymentAppService{}
	svc.SetObservationDispatcher(NewObservationDispatcher(repo, &fakeAggregator{}, &fakeTenantResolver{}, "worker"))
	order := &models.Order{
		TenantMixin: models.TenantMixin{TenantID: database.StandaloneTenantID},
		ID:          models.OrderID("order-1"),
		OrderLifecycle: models.OrderLifecycle{
			PaidAt: &paymentSentAt,
		},
	}
	repo.inserted = append(repo.inserted, &models.PaymentObservation{
		TenantID:     database.StandaloneTenantID,
		OrderID:      "order-1",
		TxHash:       "funding-tx-1",
		TxHashSource: models.PaymentTxHashSourceChainTx,
		Status:       models.PaymentObservationStatusConfirmed,
		CreatedAt:    paymentSentAt.Add(-time.Second),
	}, &models.PaymentObservation{
		TenantID:     database.StandaloneTenantID,
		OrderID:      "order-1",
		TxHash:       "later-extra-tx",
		TxHashSource: models.PaymentTxHashSourceChainTx,
		Status:       models.PaymentObservationStatusConfirmed,
		CreatedAt:    paymentSentAt.Add(time.Second),
	})
	paymentSent := &pb.PaymentSent{Timestamp: timestamppb.New(paymentSentAt)}

	observed, err := svc.buyerUTXOPaymentContributedToPaymentSent(order, "funding-tx-1", paymentSent)
	require.NoError(t, err)
	require.True(t, observed)

	observed, err = svc.buyerUTXOPaymentContributedToPaymentSent(order, "later-extra-tx", paymentSent)
	require.NoError(t, err)
	require.False(t, observed)

	observed, err = svc.buyerUTXOPaymentContributedToPaymentSent(order, "extra-tx", paymentSent)
	require.NoError(t, err)
	require.False(t, observed)
}
