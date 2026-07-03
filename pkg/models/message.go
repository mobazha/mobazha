package models

import (
	"time"

	pb "github.com/mobazha/mobazha/pkg/net/mbzpb"
	"google.golang.org/protobuf/proto"
)

// OutgoingMessage represents a message that we've sent to another
// peer. It will remain in the database until the remote peer ACKs
// the message.
type OutgoingMessage struct {
	TenantMixin
	ID                string `gorm:"primaryKey"`
	Recipient         string `gorm:"index"`
	SerializedMessage []byte
	MessageType       string
	Timestamp         time.Time
	LastAttempt       time.Time
}

func (m *OutgoingMessage) Message() (*pb.Message, error) {
	msg := new(pb.Message)
	if err := proto.Unmarshal(m.SerializedMessage, msg); err != nil {
		return nil, err
	}
	return msg, nil
}

// IncomingMessage represents a message that we've received. We store
// all received message IDs in the database so we can tell when we've
// received a duplicate.
type IncomingMessage struct {
	TenantMixin
	ID string `gorm:"primaryKey"`
}
