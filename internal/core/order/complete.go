//go:build !private_distribution

package order

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/btcsuite/btcd/btcec/v2/ecdsa"
	ordersettlement "github.com/mobazha/mobazha3.0/internal/core/order/settlement"
	"github.com/mobazha/mobazha3.0/internal/logger"
	"github.com/mobazha/mobazha3.0/internal/orders/utils"
	"github.com/mobazha/mobazha3.0/pkg/core/coreiface"
	"github.com/mobazha/mobazha3.0/pkg/database"
	"github.com/mobazha/mobazha3.0/pkg/identity"
	"github.com/mobazha/mobazha3.0/pkg/models"
	npb "github.com/mobazha/mobazha3.0/pkg/net/mbzpb"
	pb "github.com/mobazha/mobazha3.0/pkg/orders/mbzpb"
	"github.com/mobazha/mobazha3.0/pkg/payment"
	iwallet "github.com/mobazha/mobazha3.0/pkg/wallet-interface"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type orderOpTiming struct {
	nodeID  string
	orderID string
	op      string
	step    string
	start   time.Time
	stepAt  time.Time
}

func newOrderOpTiming(nodeID, orderID, op string) *orderOpTiming {
	now := time.Now()
	return &orderOpTiming{
		nodeID:  nodeID,
		orderID: orderID,
		op:      op,
		step:    "start",
		start:   now,
		stepAt:  now,
	}
}

func (t *orderOpTiming) next(step string) {
	logger.LogInfoWithIDf(log, t.nodeID,
		"%s order=%s stage=%s elapsed=%s total=%s",
		t.op, t.orderID, t.step, time.Since(t.stepAt), time.Since(t.start))
	t.step = step
	t.stepAt = time.Now()
}

func (t *orderOpTiming) finish() {
	logger.LogInfoWithIDf(log, t.nodeID,
		"%s order=%s stage=%s elapsed=%s total=%s",
		t.op, t.orderID, t.step, time.Since(t.stepAt), time.Since(t.start))
}

// GetCompleteOrderInstructions preserves the legacy client-signed
// completion surface. Backend-owned completion routes such as ManagedEscrow-backed
// moderated orders do not use this instructions contract.
func (s *OrderAppService) GetCompleteOrderInstructions(orderID models.OrderID, initiatorAddress string) (coinType iwallet.CoinType, instructions any, err error) {
	return s.GetLegacyCompleteOrderInstructions(orderID, initiatorAddress)
}

// GetLegacyCompleteOrderInstructions is the internal legacy-only moderated
// completion instructions path. Address-monitored UTXO routes are handled
// without frontend instructions and therefore return nil here; ManagedEscrow-backed
// moderated completion is a backend-owned action and must not fall through as
// a legacy no-instructions response.
func (s *OrderAppService) GetLegacyCompleteOrderInstructions(orderID models.OrderID, initiatorAddress string) (coinType iwallet.CoinType, instructions any, err error) {
	var order models.Order
	err = s.db.View(func(tx database.Tx) error {
		return tx.Read().Where("id = ?", orderID.String()).Find(&order).Error
	})
	if err != nil {
		return "", nil, fmt.Errorf("failed to get order: %w", err)
	}

	if !order.CanComplete() {
		return "", nil, fmt.Errorf("%w: order is not in a state where it can be completed", coreiface.ErrBadRequest)
	}

	if _, err := order.OrderOpenMessage(); err != nil {
		return "", nil, fmt.Errorf("failed to get order open message: %w", err)
	}

	paymentSent, err := order.PaymentSentMessage()
	if err != nil {
		return "", nil, fmt.Errorf("failed to get payment sent message: %w", err)
	}

	coinType, err = payment.SettlementCoinFromPaymentSent(paymentSent)
	if err != nil {
		return "", nil, err
	}

	method, ok := payment.ResolvedPaymentMethod(&order, paymentSent)
	if !ok {
		return coinType, nil, fmt.Errorf("payment settlement spec is missing")
	}
	if !payment.MethodIsModerated(method) {
		return coinType, nil, nil
	}
	return coinType, nil, fmt.Errorf("%w: moderated completion uses POST /v1/orders/{orderID}/settlement-actions/complete",
		coreiface.ErrBadRequest)
}

// CompleteOrder builds an OrderComplete message and sends it to the vendor.
// Ratings are optional: pass nil/empty to complete without rating.
// Use RateOrder to submit ratings for an already-completed order.
func (s *OrderAppService) CompleteOrder(orderID models.OrderID, txid iwallet.TransactionID, ratings []models.Rating, includeIDInRating bool, done chan struct{}) error {
	timing := newOrderOpTiming(s.nodeID, orderID.String(), "CompleteOrder")
	defer timing.finish()

	if err := s.acquireOrderLock(orderID); err != nil {
		return fmt.Errorf("failed to acquire order lock for %s: %w", orderID, err)
	}
	defer s.releaseOrderLock(orderID)
	timing.next("load_order")

	var (
		order   models.Order
		profile *models.Profile
		err     error
	)
	err = s.db.View(func(tx database.Tx) error {
		profile, err = tx.GetProfile()
		if err != nil {
			return err
		}
		return tx.Read().Where("id = ?", orderID.String()).Find(&order).Error
	})
	if err != nil {
		return fmt.Errorf("load order %s for completion: %w", orderID, err)
	}

	if !order.CanComplete() {
		return fmt.Errorf("%w: order is not in a state where it can be completed", coreiface.ErrBadRequest)
	}
	timing.next("validate_order")

	if err := s.attachSettlementActions(&order); err != nil {
		return fmt.Errorf("load settlement actions for order %s: %w", orderID, err)
	}

	orderOpen, err := order.OrderOpenMessage()
	if err != nil {
		return fmt.Errorf("order open message: %w", err)
	}
	paymentSent, err := order.PaymentSentMessage()
	if err != nil {
		return fmt.Errorf("payment sent message: %w", err)
	}
	coinType, err := payment.SettlementCoinFromPaymentSent(paymentSent)
	if err != nil {
		return fmt.Errorf("canonical payment coin: %w", err)
	}

	shipments, err := order.OrderShipmentMessages()
	if err != nil {
		return fmt.Errorf("order shipment messages: %w", err)
	}

	vendor, err := order.Vendor()
	if err != nil {
		return fmt.Errorf("vendor: %w", err)
	}

	var ratingPBs []*pb.Rating
	if len(ratings) > 0 {
		ratingSignatures, err := order.RatingSignaturesMessage()
		if err != nil {
			return fmt.Errorf("rating signatures: %w", err)
		}
		chaincode, err := hex.DecodeString(paymentSent.Chaincode)
		if err != nil {
			return err
		}
		peerID := s.peerID()
		ratingPBs, err = s.processRatings(ratings, orderOpen, ratingSignatures, profile, includeIDInRating, chaincode, peerID.String())
		if err != nil {
			return fmt.Errorf("process ratings: %w", err)
		}
	}
	timing.next("process_ratings")

	completeMsg := &pb.OrderComplete{
		Timestamp: timestamppb.Now(),
		Ratings:   ratingPBs,
	}

	var releaseTx *iwallet.Transaction
	if method, ok := payment.ResolvedPaymentMethod(&order, paymentSent); ok && payment.MethodIsModerated(method) {
		if !orderRequiresMonitoredSettlementActions(&order, paymentSent, coinType, s.paymentRegistry) {
			return errRetiredClientSignedModeratedSettlement("complete")
		}
		if _, err := requireBackendSubmittedSettlementSpec(&order, paymentSent); err != nil {
			return err
		}

		var releaseAlreadySubmitted bool
		txid, releaseAlreadySubmitted, err = evaluateMonitoredSettlementRelease(&order, txid, "complete")
		if err != nil {
			return err
		}
		if !releaseAlreadySubmitted {
			return errSettlementReleaseActionRequired(orderID, "complete")
		}
		release := ordersettlement.CloneEscrowRelease(shipments[0].ReleaseInfo)
		if release == nil {
			return fmt.Errorf("%w: shipment release info is missing", coreiface.ErrBadRequest)
		}
		if txid != "" {
			release.Txid = txid.String()
			releaseTx = &iwallet.Transaction{ID: txid}
		}
		completeMsg.ReleaseInfo = release
	}
	timing.next("release_complete_escrow_funds")

	var completionEvent interface{}
	if err := s.db.Update(func(tx database.Tx) error {
		completeAny := &anypb.Any{}
		if err := completeAny.MarshalFrom(completeMsg); err != nil {
			return err
		}

		m := &npb.OrderMessage{
			OrderID:     order.ID.String(),
			MessageType: npb.OrderMessage_ORDER_COMPLETE,
			Message:     completeAny,
		}

		if err := utils.SignOrderMessage(m, s.signer); err != nil {
			return err
		}

		payload := &anypb.Any{}
		if err := payload.MarshalFrom(m); err != nil {
			return err
		}

		message := newMessageWithID()
		message.MessageType = npb.Message_ORDER
		message.Payload = payload

		completionEvent, err = s.orderProcessor.ProcessMessage(tx, m)
		if err != nil {
			return err
		}
		timing.next("process_message")

		if releaseTx != nil {
			if err := saveTransactionToFreshOrder(tx, order.ID, *releaseTx); err != nil {
				return err
			}
		}

		if err := s.messenger.ReliablySendMessage(tx, vendor, message, done); err != nil {
			return err
		}
		timing.next("reliably_send_message")
		return nil
	}); err != nil {
		return err
	}
	timing.next("persist_and_dispatch")
	s.emitOrderProcessorEvents(completionEvent)
	return nil
}

// RateOrder submits ratings for an already-completed order that was completed
// without ratings. Builds a new ORDER_COMPLETE message with ratings and sends
// it to the vendor as a "rating supplement".
func (s *OrderAppService) RateOrder(orderID models.OrderID, ratings []models.Rating, includeIDInRating bool, done chan struct{}) error {
	timing := newOrderOpTiming(s.nodeID, orderID.String(), "RateOrder")
	defer timing.finish()

	if len(ratings) == 0 {
		return fmt.Errorf("%w: ratings cannot be empty", coreiface.ErrBadRequest)
	}

	if err := s.acquireOrderLock(orderID); err != nil {
		return fmt.Errorf("failed to acquire order lock for %s: %w", orderID, err)
	}
	defer s.releaseOrderLock(orderID)

	var (
		order   models.Order
		profile *models.Profile
		err     error
	)
	err = s.db.View(func(tx database.Tx) error {
		profile, err = tx.GetProfile()
		if err != nil {
			return err
		}
		return tx.Read().Where("id = ?", orderID.String()).Find(&order).Error
	})
	if err != nil {
		return fmt.Errorf("load order %s for supplement: %w", orderID, err)
	}

	if order.State != models.OrderState_COMPLETED {
		return fmt.Errorf("%w: order must be COMPLETED to submit ratings", coreiface.ErrBadRequest)
	}
	timing.next("load_order")

	existingComplete, err := order.OrderCompleteMessage()
	if err != nil {
		return fmt.Errorf("order has no ORDER_COMPLETE message: %w", err)
	}
	if len(existingComplete.Ratings) > 0 {
		return fmt.Errorf("%w: order already has ratings", coreiface.ErrBadRequest)
	}

	orderOpen, err := order.OrderOpenMessage()
	if err != nil {
		return fmt.Errorf("order open message: %w", err)
	}
	paymentSent, err := order.PaymentSentMessage()
	if err != nil {
		return fmt.Errorf("payment sent message: %w", err)
	}
	if _, err := payment.SettlementCoinFromPaymentSent(paymentSent); err != nil {
		return fmt.Errorf("canonical payment coin: %w", err)
	}

	vendor, err := order.Vendor()
	if err != nil {
		return fmt.Errorf("vendor: %w", err)
	}

	ratingSignatures, err := order.RatingSignaturesMessage()
	if err != nil {
		return fmt.Errorf("rating signatures: %w", err)
	}
	chaincode, err := hex.DecodeString(paymentSent.Chaincode)
	if err != nil {
		return fmt.Errorf("decode chaincode: %w", err)
	}

	peerID := s.peerID()
	ratingPBs, err := s.processRatings(ratings, orderOpen, ratingSignatures, profile, includeIDInRating, chaincode, peerID.String())
	if err != nil {
		return fmt.Errorf("failed to process ratings: %w", err)
	}
	timing.next("process_ratings")

	supplementMsg := &pb.OrderComplete{
		Timestamp:   existingComplete.Timestamp,
		ReleaseInfo: existingComplete.ReleaseInfo,
		Ratings:     ratingPBs,
	}

	var completionEvent interface{}
	if err := s.db.Update(func(tx database.Tx) error {
		supplementAny := &anypb.Any{}
		if err := supplementAny.MarshalFrom(supplementMsg); err != nil {
			return err
		}

		m := &npb.OrderMessage{
			OrderID:     order.ID.String(),
			MessageType: npb.OrderMessage_ORDER_COMPLETE,
			Message:     supplementAny,
		}

		if err := utils.SignOrderMessage(m, s.signer); err != nil {
			return err
		}

		payload := &anypb.Any{}
		if err := payload.MarshalFrom(m); err != nil {
			return err
		}

		message := newMessageWithID()
		message.MessageType = npb.Message_ORDER
		message.Payload = payload

		completionEvent, err = s.orderProcessor.ProcessMessage(tx, m)
		if err != nil {
			return err
		}
		timing.next("process_message")

		if err := s.messenger.ReliablySendMessage(tx, vendor, message, done); err != nil {
			return err
		}
		timing.next("reliably_send_message")
		return nil
	}); err != nil {
		return err
	}
	timing.next("persist_and_dispatch")
	s.emitOrderProcessorEvents(completionEvent)
	return nil
}

func (s *OrderAppService) processRatings(ratings []models.Rating, orderOpen *pb.OrderOpen, ratingSignatures *pb.RatingSignatures, profile *models.Profile, includeIDInRating bool, chaincode []byte, peerIDStr string) ([]*pb.Rating, error) {
	if len(ratings) != len(orderOpen.Items) {
		return nil, errors.New("number of ratings does not equal number of items in the order")
	}

	if len(ratingSignatures.Sigs) != len(orderOpen.Items) {
		return nil, errors.New("missing rating signatures from vendor needed to build rating")
	}

	ratingMasterKey, err := s.keyProvider.RatingMasterKey()
	if err != nil {
		return nil, fmt.Errorf("failed to get rating master key: %w", err)
	}
	ratingKeys, err := utils.GenerateRatingPrivateKeys(ratingMasterKey, len(orderOpen.Items), chaincode)
	if err != nil {
		return nil, err
	}

	var ratingPBs []*pb.Rating
	for i, rating := range ratings {
		ratingPB := &pb.Rating{
			Timestamp: timestamppb.Now(),

			VendorSig: ratingSignatures.Sigs[i],
			VendorID:  orderOpen.Listings[0].Listing.VendorID,

			Overall:     uint32(rating.Overall),
			Review:      rating.Review,
			ImageHashes: rating.ImageHashes,
		}

		if includeIDInRating {
			rawPubKey, err := s.signer.PublicKey()
			if err != nil {
				return nil, err
			}
			identityPubkey, err := identity.MarshalPublicKeyFromEd25519(rawPubKey)
			if err != nil {
				return nil, err
			}

			escrowMasterKey, err := s.keyProvider.EscrowMasterKey()
			if err != nil {
				return nil, fmt.Errorf("failed to get escrow master key: %w", err)
			}
			ethMasterKey, err := s.keyProvider.EVMMasterKey()
			if err != nil {
				return nil, fmt.Errorf("failed to get ETH master key: %w", err)
			}
			solPrivKey, err := s.keyProvider.SolanaMasterKey()
			if err != nil {
				return nil, fmt.Errorf("failed to get Solana master key: %w", err)
			}

			idHash := sha256.Sum256([]byte(peerIDStr))
			sig := ecdsa.Sign(escrowMasterKey, idHash[:])

			ratingPB.BuyerName = profile.Name
			ratingPB.BuyerID = &pb.ID{
				PeerID: peerIDStr,
				Pubkeys: &pb.ID_Pubkeys{
					Identity: identityPubkey,
					Escrow:   escrowMasterKey.PubKey().SerializeCompressed(),
					Eth:      ethMasterKey.PubKey().SerializeCompressed(),
					Solana:   solPrivKey.PublicKey().Bytes(),
				},
				Handle:     profile.Handle,
				Name:       profile.Name,
				AvatarTiny: profile.AvatarHashes.Tiny,
				Sig:        sig.Serialize(),
			}

			ratingSigHash := sha256.Sum256(ratingPB.VendorSig.RatingKey)
			buyerSig, err := s.signer.Sign(ratingSigHash[:])
			if err != nil {
				return nil, err
			}
			ratingPB.BuyerSig = buyerSig
		}

		ser, err := proto.Marshal(ratingPB)
		if err != nil {
			return nil, err
		}

		hashed := sha256.Sum256(ser)

		ratingSig := ecdsa.Sign(ratingKeys[i], hashed[:])
		ratingPB.RatingSignature = ratingSig.Serialize()

		ratingPBs = append(ratingPBs, ratingPB)
	}

	return ratingPBs, nil
}

// executeUTXOSyncModeratedCompleteRelease signs and broadcasts a moderated UTXO
// complete release via settlement-actions/complete.
func (s *OrderAppService) executeUTXOSyncModeratedCompleteRelease(order *models.Order, wallet iwallet.Wallet, releaseInfo *pb.EscrowRelease) (*pb.EscrowRelease, *iwallet.Transaction, error) {
	paymentSent, err := order.PaymentSentMessage()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get payment sent message: %w", err)
	}

	coinType, err := payment.SettlementCoinFromPaymentSent(paymentSent)
	if err != nil {
		return nil, nil, err
	}

	txn := iwallet.Transaction{
		To: []iwallet.SpendInfo{
			{
				Address: iwallet.NewAddress(releaseInfo.ToAddress, coinType),
				Amount:  iwallet.NewAmount(releaseInfo.ToAmount),
			},
		},
	}

	if iwallet.NewAmount(releaseInfo.PlatformAmount).Cmp(iwallet.NewAmount(0)) > 0 {
		txn.To = append(txn.To, iwallet.SpendInfo{
			Address: iwallet.NewAddress(releaseInfo.PlatformAddress, coinType),
			Amount:  iwallet.NewAmount(releaseInfo.PlatformAmount),
		})
	}

	for _, outpoint := range releaseInfo.Outpoints {
		txn.From = append(txn.From, iwallet.SpendInfo{
			ID:     outpoint.FromID,
			Amount: iwallet.NewAmount(outpoint.Value),
		})
	}

	script, err := hex.DecodeString(paymentSent.Script)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to decode script: %w", err)
	}

	chainCode, err := hex.DecodeString(paymentSent.Chaincode)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to decode chain code: %w", err)
	}

	strategy, err := s.v2StrategyForCoin(coinType)
	if err != nil {
		return nil, nil, fmt.Errorf("no chain escrow for coin %s: %w", coinType, err)
	}

	buyerSigs, err := strategy.SignEscrowRelease(context.Background(), payment.SignEscrowParams{
		Transaction: txn,
		Script:      script,
		ChainCode:   chainCode,
		CoinCode:    string(coinType),
	})
	if err != nil {
		return nil, nil, fmt.Errorf("failed to sign escrow release: %w", err)
	}

	release := &pb.EscrowRelease{
		ToAddress:       releaseInfo.ToAddress,
		ToAmount:        releaseInfo.ToAmount,
		PlatformAddress: releaseInfo.PlatformAddress,
		PlatformAmount:  releaseInfo.PlatformAmount,
		TransactionFee:  releaseInfo.TransactionFee,
		Outpoints:       releaseInfo.Outpoints,
	}

	for _, sig := range buyerSigs {
		release.EscrowSignatures = append(release.EscrowSignatures, &pb.Signature{
			Signature: sig.Signature,
			Index:     uint32(sig.Index),
		})
	}

	var vendorSigs []iwallet.EscrowSignature
	for _, sig := range releaseInfo.EscrowSignatures {
		vendorSigs = append(vendorSigs, iwallet.EscrowSignature{
			Index:     int(sig.Index),
			Signature: sig.Signature,
		})
	}

	if strategy.Capabilities().HasClientSignedEscrow {
		return release, nil, nil
	}

	escrowWallet, ok := wallet.(iwallet.UTXOEscrow)
	if !ok {
		return nil, nil, errors.New("wallet does not support escrow")
	}
	wtx, err := wallet.Begin()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	txid, err := escrowWallet.BuildAndSend(wtx, txn, [][]iwallet.EscrowSignature{buyerSigs, vendorSigs}, script, iwallet.ORDER_FINISH_COMPLETE)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to build and send transaction: %w", err)
	}
	release.Txid = txid.String()

	if err := wtx.Commit(); err != nil {
		return nil, nil, err
	}

	txn.ID = txid
	txn.Timestamp = time.Now()

	return release, &txn, nil
}
