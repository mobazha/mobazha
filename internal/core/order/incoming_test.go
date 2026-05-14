//go:build !private_distribution

package order

import (
	"testing"

	"github.com/mobazha/mobazha3.0/pkg/database"
	"github.com/mobazha/mobazha3.0/pkg/models"
	npb "github.com/mobazha/mobazha3.0/pkg/net/mbzpb"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TD-025: recordPreProcessError must persist failed messages to the order's
// ErroredMessages list (best-effort, never panics).

func TestOrderAppService_RecordPreProcessError_AppendsToErroredMessages(t *testing.T) {
	svc := newTestOrderAppService(t, OrderAppServiceConfig{})
	seedOrder(t, svc, "order-td025-1", "buyer", models.OrderState_AWAITING_PAYMENT)

	msg := &npb.OrderMessage{
		OrderID:     "order-td025-1",
		MessageType: npb.OrderMessage_PAYMENT_SENT,
	}

	svc.recordPreProcessError(msg, assert.AnError)

	var stored models.Order
	err := svc.db.View(func(tx database.Tx) error {
		return tx.Read().Where("id = ?", "order-td025-1").First(&stored).Error
	})
	require.NoError(t, err)

	errored, err := stored.GetErroredMessages()
	require.NoError(t, err)
	require.Len(t, errored.Messages, 1, "expected one errored message recorded")
	assert.Equal(t, "order-td025-1", errored.Messages[0].OrderID)
	assert.Equal(t, npb.OrderMessage_PAYMENT_SENT, errored.Messages[0].MessageType)
}

func TestOrderAppService_RecordPreProcessError_AccumulatesMultipleErrors(t *testing.T) {
	svc := newTestOrderAppService(t, OrderAppServiceConfig{})
	seedOrder(t, svc, "order-td025-2", "buyer", models.OrderState_AWAITING_PAYMENT)

	msg1 := &npb.OrderMessage{OrderID: "order-td025-2", MessageType: npb.OrderMessage_PAYMENT_SENT}
	msg2 := &npb.OrderMessage{OrderID: "order-td025-2", MessageType: npb.OrderMessage_ORDER_CONFIRMATION}

	svc.recordPreProcessError(msg1, assert.AnError)
	svc.recordPreProcessError(msg2, assert.AnError)

	var stored models.Order
	err := svc.db.View(func(tx database.Tx) error {
		return tx.Read().Where("id = ?", "order-td025-2").First(&stored).Error
	})
	require.NoError(t, err)

	errored, err := stored.GetErroredMessages()
	require.NoError(t, err)
	require.Len(t, errored.Messages, 2, "subsequent failures should accumulate")
	assert.Equal(t, npb.OrderMessage_PAYMENT_SENT, errored.Messages[0].MessageType)
	assert.Equal(t, npb.OrderMessage_ORDER_CONFIRMATION, errored.Messages[1].MessageType)
}

func TestOrderAppService_RecordPreProcessError_OrderNotFound_NoPanic(t *testing.T) {
	svc := newTestOrderAppService(t, OrderAppServiceConfig{})

	msg := &npb.OrderMessage{
		OrderID:     "order-td025-missing",
		MessageType: npb.OrderMessage_PAYMENT_SENT,
	}

	// Best-effort: must not panic when the order does not exist in DB.
	require.NotPanics(t, func() {
		svc.recordPreProcessError(msg, assert.AnError)
	})
}

func TestOrderAppService_RecordPreProcessError_NilMessage_NoPanic(t *testing.T) {
	svc := newTestOrderAppService(t, OrderAppServiceConfig{})

	// Defensive: passing nil should be a no-op, never a panic.
	require.NotPanics(t, func() {
		svc.recordPreProcessError(nil, assert.AnError)
	})
}
