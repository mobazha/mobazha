package core

import (
	"context"
	"testing"

	pb "github.com/mobazha/mobazha/pkg/orders/mbzpb"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFiatPayment_CreateOrder_PricingCoinUnchanged(t *testing.T) {
	network, index := setupMocknetForDiscount(t)
	buyer := network.Nodes()[1]

	purchase := newTestPurchase(index[0].CID)

	var orderOpen *pb.OrderOpen
	retryOnIPNS(t, 5, func() error {
		var err error
		orderOpen, _, err = buyer.orderService.CreateOrderForTesting(context.Background(), purchase)
		return err
	})

	require.NotNil(t, orderOpen)
	assert.Equal(t, purchase.PricingCoin, orderOpen.PricingCoin)
}
