package order

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"

	"github.com/mobazha/mobazha3.0/internal/database"
	"github.com/mobazha/mobazha3.0/pkg/models"
	pb "github.com/mobazha/mobazha3.0/pkg/net/mbzpb"
	iwallet "github.com/mobazha/mobazha3.0/pkg/wallet-interface"
)

const (
	ListingVersion = 1
	DisputeTimeout = 1080
)

func normalizeCurrencyCode(currencyCode string) string {
	if iwallet.IsValidCoinType(iwallet.CoinType(currencyCode)) {
		return currencyCode
	}
	c, err := models.CurrencyDefinitions.Lookup(currencyCode)
	if err != nil {
		log.Errorf("invalid currency code (%s): %s", currencyCode, err.Error())
		return ""
	}
	return c.String()
}

func newMessageWithID() *pb.Message {
	messageID := make([]byte, 20)
	if _, err := rand.Read(messageID); err != nil {
		panic("crypto/rand unavailable: " + err.Error())
	}
	return &pb.Message{
		MessageID: hex.EncodeToString(messageID),
	}
}

func padOrTruncateBytes(b []byte, length int) []byte {
	if len(b) > length {
		return b[:length]
	}
	if len(b) < length {
		padded := make([]byte, length)
		copy(padded, b)
		return padded
	}
	return b
}

func saveTransactionToFreshOrder(dbtx database.Tx, orderID models.OrderID, tx iwallet.Transaction) error {
	if tx.ID == "" {
		return nil
	}
	var freshOrder models.Order
	if err := dbtx.Read().Where("id = ?", orderID).First(&freshOrder).Error; err != nil {
		return fmt.Errorf("failed to re-fetch order: %w", err)
	}
	if err := freshOrder.PutTransaction(tx); err != nil && !models.IsDuplicateTransactionError(err) {
		return fmt.Errorf("save transaction: %w", err)
	}
	return dbtx.Save(&freshOrder)
}
