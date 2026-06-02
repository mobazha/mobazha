package orders

import (
	"testing"
	"time"

	"github.com/mobazha/mobazha3.0/internal/repo"
	"github.com/mobazha/mobazha3.0/pkg/database"
	"github.com/mobazha/mobazha3.0/pkg/models"
	npb "github.com/mobazha/mobazha3.0/pkg/net/mbzpb"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
)

func TestProcessACK_OrderOpen_SetsPaymentReadyAt(t *testing.T) {
	db, err := repo.MockDB()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	orderID := "QmOrderAckTest"
	if err := db.Update(func(tx database.Tx) error {
		if err := tx.Migrate(&models.Order{}); err != nil {
			return err
		}
		return tx.Save(&models.Order{ID: models.OrderID(orderID), Open: true})
	}); err != nil {
		t.Fatal(err)
	}

	orderMsg := &npb.OrderMessage{
		OrderID:     orderID,
		MessageType: npb.OrderMessage_ORDER_OPEN,
	}
	payload, err := anypb.New(orderMsg)
	if err != nil {
		t.Fatal(err)
	}
	ser, err := proto.Marshal(&npb.Message{
		MessageType: npb.Message_ORDER,
		Payload:     payload,
	})
	if err != nil {
		t.Fatal(err)
	}

	op := NewOrderProcessor(&Config{Db: db})
	if err := db.Update(func(tx database.Tx) error {
		newly, _, ackErr := op.ProcessACK(tx, &models.OutgoingMessage{
			ID:                "msg-ack-1",
			SerializedMessage: ser,
		})
		if ackErr != nil {
			return ackErr
		}
		if !newly {
			t.Fatal("expected first ORDER_OPEN ACK to mark payment ready")
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	var order models.Order
	if err := db.View(func(tx database.Tx) error {
		return tx.Read().Where("id = ?", orderID).First(&order).Error
	}); err != nil {
		t.Fatal(err)
	}
	if order.PaymentReadyAt == nil {
		t.Fatal("expected payment_ready_at to be set")
	}
	if !order.OrderOpenAcked {
		t.Fatal("expected order_open_acked to be set")
	}
	if !models.IsPaymentReady(&order) {
		t.Fatal("expected IsPaymentReady true")
	}
	if time.Since(*order.PaymentReadyAt) > time.Minute {
		t.Fatalf("payment_ready_at looks stale: %v", order.PaymentReadyAt)
	}
}
