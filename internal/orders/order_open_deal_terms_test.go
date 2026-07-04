package orders

import (
	"strings"
	"testing"

	"github.com/mobazha/mobazha/internal/orders/utils"
	"github.com/mobazha/mobazha/pkg/database"
	"github.com/mobazha/mobazha/pkg/models"
	"github.com/mobazha/mobazha/pkg/models/factory"
	npb "github.com/mobazha/mobazha/pkg/net/mbzpb"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOrderProcessorProcessMessagePersistsDealTermsSnapshotRef(t *testing.T) {
	op, teardown, err := newMockOrderProcessor()
	require.NoError(t, err)
	defer teardown()

	require.NoError(t, op.db.Update(func(tx database.Tx) error {
		return tx.SetListing(factory.NewSignedListing())
	}))

	orderOpen, _, err := factory.NewOrder()
	require.NoError(t, err)
	orderOpen.DealLinkID = "deal-789"
	orderOpen.DealRevision = 3
	orderOpen.TermsHash = strings.Repeat("c", 64)
	orderOpen.FeeQuoteID = "fee-quote-789"

	orderID, err := utils.CalcOrderID(orderOpen)
	require.NoError(t, err)
	message := &npb.OrderMessage{
		OrderID:     orderID.B58String(),
		MessageType: npb.OrderMessage_ORDER_OPEN,
		Message:     mustBuildAny(orderOpen),
	}
	require.NoError(t, utils.SignOrderMessage(message, op.signer))

	require.NoError(t, op.db.Update(func(tx database.Tx) error {
		_, err := op.ProcessMessage(tx, message)
		return err
	}))

	var stored models.Order
	require.NoError(t, op.db.View(func(tx database.Tx) error {
		return tx.Read().Where("id = ?", orderID.B58String()).First(&stored).Error
	}))
	require.NotNil(t, stored.DealTermsSnapshotRef)
	assert.Equal(t, "deal-789", stored.DealTermsSnapshotRef.DealLinkID)
	assert.Equal(t, uint64(3), stored.DealTermsSnapshotRef.Revision)
	assert.Equal(t, strings.Repeat("c", 64), stored.DealTermsSnapshotRef.TermsHash)
	assert.Equal(t, "fee-quote-789", stored.DealTermsSnapshotRef.FeeQuoteID)

	plain := &models.Order{ID: "plain-order"}
	require.NoError(t, op.db.Update(func(tx database.Tx) error {
		return tx.Save(plain)
	}))
	require.NoError(t, op.db.View(func(tx database.Tx) error {
		return tx.Read().Where("id = ?", plain.ID).First(plain).Error
	}))
	assert.Nil(t, plain.DealTermsSnapshotRef)
}
