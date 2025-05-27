package orders

import (
	"bytes"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	btcec "github.com/btcsuite/btcd/btcec/v2"
	"github.com/ipfs/go-cid"
	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/mobazha/mobazha3.0/internal/database"
	"github.com/mobazha/mobazha3.0/internal/logger"
	"github.com/mobazha/mobazha3.0/internal/multiwallet"
	"github.com/mobazha/mobazha3.0/internal/multiwallet/coins/eth/util"
	"github.com/mobazha/mobazha3.0/internal/net"
	"github.com/mobazha/mobazha3.0/internal/wallet"
	pkgconfig "github.com/mobazha/mobazha3.0/pkg/config"
	"github.com/mobazha/mobazha3.0/pkg/events"
	"github.com/mobazha/mobazha3.0/pkg/models"
	npb "github.com/mobazha/mobazha3.0/pkg/net/mbzpb"
	iwallet "github.com/mobazha/mobazha3.0/pkg/wallet-interface"
	"github.com/op/go-logging"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"gorm.io/gorm"
)

const rescanTransactionsInterval = time.Minute

var (
	log                  = logging.MustGetLogger("ORDR")
	ErrChangedMessage    = errors.New("different duplicate message")
	ErrUnexpectedMessage = errors.New("unexpected message")
)

// Config holds the objects needed to instantiate a new OrderProcessor.
type Config struct {
	NodeID               string
	Identity             peer.ID
	Db                   database.Database
	IdentityPrivateKey   crypto.PrivKey
	EscrowPrivateKey     *btcec.PrivateKey
	Messenger            *net.Messenger
	Multiwallet          multiwallet.Multiwallet
	ExchangeRateProvider *wallet.ExchangeRateProvider
	EventBus             events.Bus
	CalcCIDFunc          func(file []byte) (cid.Cid, error)
	FeatureManager       *pkgconfig.FeatureManager
}

// OrderProcessor is used to deterministically process orders.
type OrderProcessor struct {
	nodeID             string
	identity           peer.ID
	identityPrivateKey crypto.PrivKey
	db                 database.Database
	messenger          *net.Messenger
	multiwallet        multiwallet.Multiwallet
	escrowPrivateKey   *btcec.PrivateKey
	erp                *wallet.ExchangeRateProvider
	bus                events.Bus
	calcCIDFunc        func(file []byte) (cid.Cid, error)
	featureManager     *pkgconfig.FeatureManager
	shutdown           chan struct{}
}

// NewOrderProcessor initializes and returns a new OrderProcessor
func NewOrderProcessor(cfg *Config) *OrderProcessor {
	return &OrderProcessor{
		nodeID:             cfg.NodeID,
		identity:           cfg.Identity,
		identityPrivateKey: cfg.IdentityPrivateKey,
		db:                 cfg.Db,
		messenger:          cfg.Messenger,
		multiwallet:        cfg.Multiwallet,
		escrowPrivateKey:   cfg.EscrowPrivateKey,
		erp:                cfg.ExchangeRateProvider,
		bus:                cfg.EventBus,
		calcCIDFunc:        cfg.CalcCIDFunc,
		featureManager:     cfg.FeatureManager,
		shutdown:           make(chan struct{}),
	}
}

// Start begins listening for transactions from the wallets that pertain to our
// orders. When we find one we record the payment.
func (op *OrderProcessor) Start() {
	if op.featureManager.IsEnabled(pkgconfig.FeatureNoBuildinWallet) {
		logger.LogInfoWithIDf(log, op.nodeID, "No buildin wallet, skipping buildin wallet transaction listening")
		// if new tx, directly call processWalletTransaction()
		// op.processWalletTransaction(tx)
		return
	}

	for _, wallet := range op.multiwallet {
		go func(w iwallet.Wallet) {
			sub := w.SubscribeTransactions()
			for {
				select {
				case tx := <-sub:
					if w.CoinCategory() == iwallet.CoinCategoryEthereum {
						currentAddress, _ := w.CurrentAddress()
						if strings.EqualFold(util.EnsureCorrectPrefix(tx.From[0].Address.String()),
							util.EnsureCorrectPrefix(currentAddress.String())) {
							tx.Value = iwallet.NewAmount(0).Sub(tx.From[0].Amount)
						}
					}

					op.processWalletTransaction(tx)
				case <-op.shutdown:
					return
				}
			}
		}(wallet)
	}

	ticker := time.NewTicker(rescanTransactionsInterval)
	for {
		select {
		case <-ticker.C:
			op.CheckForMorePayments(false)
		case <-op.shutdown:
			return
		}
	}
}

// Stop shuts down the processor.
func (op *OrderProcessor) Stop() {
	close(op.shutdown)
}

// ProcessMessage is the main handler for the OrderProcessor. It ingests a new message
// loads the corresponding order from the database, passes the message off to the appropriate
// handler for processing, then saves the updated state back into the database.
// Any messages that arrive out of order are saved in the database as a parked message which
// will allow for future processing. The same is said for messages that error.
//
// The end result of this process is if the buyer and vendor pass in the same set of messages
// into this function, regardless of order, the exact same state should be calculated for
// both nodes.
//
// If the processing of the message triggers an event to emitted onto the bus, the event is
// returned.
func (op *OrderProcessor) ProcessMessage(dbtx database.Tx, peer peer.ID, message *npb.OrderMessage) (interface{}, error) {
	// Load the order if it exists.
	var (
		order models.Order
		event interface{}
		err   error
	)
	err = dbtx.Read().Where("id = ?", message.OrderID).First(&order).Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	} else if errors.Is(err, gorm.ErrRecordNotFound) && message.MessageType != npb.OrderMessage_ORDER_OPEN {
		// Order does not exist in the DB and the message type is not an order open. This can happen
		// in the case where we download offline messages out of order. In this case we will park
		// the message so that we can try again later if we receive other messages.
		logger.LogInfoWithIDf(log, op.nodeID, "Received %s message from peer %s for an order that does not exist yet", message.MessageType, peer)
		order.ID = models.OrderID(message.OrderID)
		if err := order.ParkMessage(message); err != nil {
			return nil, err
		}
		return nil, dbtx.Save(&order)
	}

	orderCopy := order
	event, err = op.processMessage(dbtx, &order, peer, message)
	if err != nil {
		logger.LogInfoWithIDf(log, op.nodeID, "Error processing order message for order %s: %s", order.ID.String(), err)
		if err1 := orderCopy.PutErrorMessage(message); err1 != nil {
			dbtx.Save(&orderCopy)
		}

		return nil, err
	}

	parkedMessages, err := order.GetParkedMessages()
	if err != nil {
		return nil, err
	}

	sort.Slice(parkedMessages.Messages, func(i, j int) bool {
		return parkedMessages.Messages[i].MessageType < parkedMessages.Messages[j].MessageType
	})

	for _, parked := range parkedMessages.Messages {
		if proto.Equal(parked, message) {
			continue
		}
		_, err = op.processMessage(dbtx, &order, peer, parked)
		if err != nil {
			logger.LogInfoWithIDf(log, op.nodeID, "Error processing parked message for order %s: %s", order.ID.String(), err)
			if err := order.PutErrorMessage(message); err != nil {
				logger.LogInfoWithIDf(log, op.nodeID, "Error saving errored message for order %s: %s", order.ID.String(), err)
			}
		}
	}

	// Save changes to the database.
	return event, dbtx.Save(&order)
}

// ProcessACK loads the order from the database and sets the ACK for the message type.
func (op *OrderProcessor) ProcessACK(tx database.Tx, om *models.OutgoingMessage) error {
	message := new(npb.Message)
	if err := proto.Unmarshal(om.SerializedMessage, message); err != nil {
		return err
	}

	orderMessage := new(npb.OrderMessage)
	if err := message.Payload.UnmarshalTo(orderMessage); err != nil {
		return err
	}

	var key string
	switch orderMessage.MessageType {
	case npb.OrderMessage_ORDER_OPEN:
		key = "order_open_acked"
	case npb.OrderMessage_ORDER_REJECT:
		key = "order_reject_acked"
	case npb.OrderMessage_ORDER_CANCEL:
		key = "order_cancel_acked"
	case npb.OrderMessage_ORDER_CONFIRMATION:
		key = "order_confirmation_acked"
	case npb.OrderMessage_ORDER_FULFILLMENT:
		key = "order_fulfillment_acked"
	case npb.OrderMessage_ORDER_COMPLETE:
		key = "order_complete_acked"
	case npb.OrderMessage_DISPUTE_OPEN:
		if message.MessageType == npb.Message_DISPUTE {
			key = "dispute_open_moderator_acked"
		} else {
			key = "dispute_open_other_party_acked"
		}
	case npb.OrderMessage_DISPUTE_UPDATE:
		key = "dispute_update_acked"
	case npb.OrderMessage_DISPUTE_CLOSE:
		key = "dispute_closed_acked"
	case npb.OrderMessage_DISPUTE_ACCEPT:
		key = "dispute_accepted_acked"
	case npb.OrderMessage_REFUND:
		key = "refund_acked"
	case npb.OrderMessage_PAYMENT_SENT:
		key = "payment_sent_acked"
	case npb.OrderMessage_RATING_SIGNATURES:
		key = "rating_signatures_acked"
	case npb.OrderMessage_PAYMENT_FINALIZED:
		key = "payment_finalized_acked"
	default:
		return fmt.Errorf("unknown order message type")
	}
	return tx.Update(key, true, map[string]interface{}{"id = ?": orderMessage.OrderID}, &models.Order{})
}

// processMessage passes the message off to the appropriate handler.
func (op *OrderProcessor) processMessage(dbtx database.Tx, order *models.Order, peer peer.ID, message *npb.OrderMessage) (event interface{}, err error) {
	err = verifyOrderMessageSignature(peer, message)
	if err != nil {
		return nil, err
	}
	switch message.MessageType {
	case npb.OrderMessage_ORDER_OPEN:
		event, err = op.processOrderOpenMessage(dbtx, order, peer, message)
	case npb.OrderMessage_PAYMENT_SENT:
		event, err = op.processPaymentSentMessage(dbtx, order, peer, message)
	case npb.OrderMessage_RATING_SIGNATURES:
		event, err = op.processRatingSignaturesMessage(dbtx, order, peer, message)
	case npb.OrderMessage_ORDER_REJECT:
		event, err = op.processOrderRejectMessage(dbtx, order, peer, message)
	case npb.OrderMessage_ORDER_CONFIRMATION:
		event, err = op.processOrderConfirmationMessage(dbtx, order, peer, message)
	case npb.OrderMessage_ORDER_CANCEL:
		event, err = op.processOrderCancelMessage(dbtx, order, peer, message)
	case npb.OrderMessage_REFUND:
		event, err = op.processRefundMessage(dbtx, order, peer, message)
	case npb.OrderMessage_ORDER_FULFILLMENT:
		event, err = op.processOrderFulfillmentMessage(dbtx, order, peer, message)
	case npb.OrderMessage_ORDER_COMPLETE:
		event, err = op.processOrderCompleteMessage(dbtx, order, peer, message)
	case npb.OrderMessage_DISPUTE_OPEN:
		event, err = op.processDisputeOpenMessage(dbtx, order, peer, message)
	case npb.OrderMessage_DISPUTE_CLOSE:
		event, err = op.processDisputeCloseMessage(dbtx, order, peer, message)
	case npb.OrderMessage_DISPUTE_ACCEPT:
		event, err = op.processDisputeAcceptMessage(dbtx, order, peer, message)
	case npb.OrderMessage_PAYMENT_FINALIZED:
		event, err = op.processPaymentFinalizeMessage(dbtx, order, peer, message)
	default:
		return nil, errors.New("unknown order message type")
	}
	return event, err
}

// isDuplicate checks the serialization of the passed in message against the
// passed in serialization and returns true if they match.
func isDuplicate(message proto.Message, serialized []byte) (bool, error) {
	m := protojson.MarshalOptions{
		EmitUnpopulated: true,
		Indent:          "    ",
	}

	ser := m.Format(message)

	return bytes.Equal([]byte(ser), serialized), nil
}

func verifyOrderMessageSignature(peer peer.ID, message *npb.OrderMessage) error {
	peerPubkey, err := peer.ExtractPublicKey()

	msgCpy := *message
	msgCpy.Signature = nil

	ser, err := proto.Marshal(&msgCpy)
	if err != nil {
		return err
	}

	valid, err := peerPubkey.Verify(ser, message.Signature)
	if err != nil {
		return err
	}

	if !valid {
		return errors.New("invalid signature")
	}
	return nil
}

// GetPreferences returns the saved preferences for this node.
func (op *OrderProcessor) GetActiveReceivingAccountByChain(tx database.Tx, chainType string) (*models.ReceivingAccount, error) {
	var records []models.ReceivingAccount
	err := tx.Read().Where("chain_type = ?", chainType).Find(&records).Error
	if err != nil {
		return nil, err
	}

	for _, r := range records {
		tokens, err := r.ActiveTokens()
		if err != nil {
			return nil, err
		}
		if len(tokens) > 0 {
			return &r, nil
		}
	}
	return nil, errors.New("no active receiving account found")
}

func (op *OrderProcessor) GetPayoutAddress(tx database.Tx, coinType string) (iwallet.Address, error) {
	coinInfo, err := iwallet.CoinInfoFromCoinType(iwallet.CoinType(coinType))
	if err != nil {
		return iwallet.Address{}, fmt.Errorf("获取币种信息失败: %v", err)
	}

	// 从激活的收款账户获取地址
	account, err := op.GetActiveReceivingAccountByChain(tx, coinInfo.Chain.String())
	if err == nil && account != nil {
		logger.LogInfoWithIDf(log, op.nodeID, "使用激活的收款账户获取地址: %s", account.Address)
		return iwallet.NewAddress(account.Address, iwallet.CoinType(coinType)), nil
	}

	return iwallet.Address{}, fmt.Errorf("获取激活的收款账户失败: %v", err)

	// // 使用内置钱包获取地址
	// logger.LogInfoWithIDf(log, n.nodeID, "使用内置钱包获取地址: %s", coinInfo.Chain.String())

	// // 使用原生代币获取链的地址信息
	// wallet, err := n.multiwallet.WalletForCurrencyCode(coinInfo.Chain.String())
	// if err != nil {
	// 	return iwallet.Address{}, fmt.Errorf("获取 %s 钱包失败: %v", coinInfo.Chain.String(), err)
	// }

	// address, err := wallet.CurrentAddress()
	// if err != nil {
	// 	return iwallet.Address{}, fmt.Errorf("获取 %s 钱包地址失败: %v", coinInfo.Chain.String(), err)
	// }

	// return address, nil
}
