package orders

import (
	"testing"

	"github.com/mobazha/mobazha/internal/repo"
	"github.com/mobazha/mobazha/pkg/database"
	"github.com/mobazha/mobazha/pkg/models"
	npb "github.com/mobazha/mobazha/pkg/net/mbzpb"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
)

func TestProcessACK_SettlementMessagesAreAccepted(t *testing.T) {
	db, err := repo.MockDB()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	op := NewOrderProcessor(&Config{Db: db})
	messageTypes := []npb.OrderMessage_MessageType{
		npb.OrderMessage_SETTLEMENT_KEY_OFFER,
		npb.OrderMessage_SETTLEMENT_AUTHORIZATION,
		npb.OrderMessage_SETTLEMENT_FUNDING_BASIS,
	}

	for _, messageType := range messageTypes {
		t.Run(messageType.String(), func(t *testing.T) {
			orderMessage := &npb.OrderMessage{
				OrderID:     "QmSettlementAckTest",
				MessageType: messageType,
			}
			payload, err := anypb.New(orderMessage)
			if err != nil {
				t.Fatal(err)
			}
			serialized, err := proto.Marshal(&npb.Message{
				MessageType: npb.Message_ORDER,
				Payload:     payload,
			})
			if err != nil {
				t.Fatal(err)
			}

			if err := db.Update(func(tx database.Tx) error {
				newlyReady, orderID, ackErr := op.ProcessACK(tx, &models.OutgoingMessage{
					ID:                "settlement-ack-" + messageType.String(),
					SerializedMessage: serialized,
				})
				if ackErr != nil {
					return ackErr
				}
				if newlyReady || orderID != "" {
					t.Fatalf("settlement ACK unexpectedly changed payment readiness: newly=%v orderID=%q", newlyReady, orderID)
				}
				return nil
			}); err != nil {
				t.Fatal(err)
			}
		})
	}
}
