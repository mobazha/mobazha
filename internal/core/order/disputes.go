package order

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/libp2p/go-libp2p/core/peer"
	nodepayment "github.com/mobazha/mobazha/internal/core/payment"
	"github.com/mobazha/mobazha/internal/logger"
	"github.com/mobazha/mobazha/internal/orders/utils"
	"github.com/mobazha/mobazha/pkg/core/coreiface"
	"github.com/mobazha/mobazha/pkg/database"
	"github.com/mobazha/mobazha/pkg/events"
	"github.com/mobazha/mobazha/pkg/models"
	npb "github.com/mobazha/mobazha/pkg/net/mbzpb"
	pb "github.com/mobazha/mobazha/pkg/orders/mbzpb"
	"github.com/mobazha/mobazha/pkg/payment"
	iwallet "github.com/mobazha/mobazha/pkg/wallet-interface"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/timestamppb"
	"gorm.io/gorm"
)

// OpenDispute sends a disputeOpen message to both the moderator and the other party.
func (s *OrderAppService) OpenDispute(orderID models.OrderID, reason string, evidenceHashes []string, done chan struct{}) error {
	if err := s.acquireOrderLock(orderID); err != nil {
		return fmt.Errorf("failed to acquire order lock for %s: %w", orderID, err)
	}
	defer s.releaseOrderLock(orderID)

	done1, done2 := make(chan struct{}), make(chan struct{})
	go func() {
		if done != nil {
			timeout := time.After(2 * time.Minute)
			select {
			case <-done1:
			case <-timeout:
				close(done)
				return
			}
			select {
			case <-done2:
			case <-timeout:
			}
			close(done)
		}
	}()

	var order models.Order
	err := s.db.View(func(tx database.Tx) error {
		return tx.Read().Where("id = ?", orderID.String()).Find(&order).Error
	})
	if err != nil {
		return fmt.Errorf("load order %s for dispute: %w", orderID, err)
	}

	if !order.CanDispute() {
		return fmt.Errorf("%w: order is not in a state where it can be disputed", coreiface.ErrBadRequest)
	}

	buyer, err := order.Buyer()
	if err != nil {
		return fmt.Errorf("buyer: %w", err)
	}
	vendor, err := order.Vendor()
	if err != nil {
		return fmt.Errorf("vendor: %w", err)
	}

	moderator, err := order.Moderator()
	if err != nil {
		return fmt.Errorf("moderator: %w", err)
	}

	var (
		role = pb.DisputeOpen_BUYER
		to   = vendor
	)
	if order.Role() == models.RoleVendor {
		role = pb.DisputeOpen_VENDOR
		to = buyer
	}

	serializedContract, err := order.MarshalBinary()
	if err != nil {
		return fmt.Errorf("marshal contract: %w", err)
	}

	paymentSent, err := order.PaymentSentMessage()
	if err != nil {
		return fmt.Errorf("payment sent message: %w", err)
	}
	coinType, err := payment.SettlementCoinFromPaymentSent(paymentSent)
	if err != nil {
		return fmt.Errorf("canonical payment coin: %w", err)
	}

	var payoutAddress iwallet.Address
	if order.Role() == models.RoleBuyer && !coinType.IsFiatPayment() {
		observations := payment.RefundResolutionObservations(s.db, &order, paymentSent)
		refundResult := nodepayment.ResolveBuyerRefundForLocalNode(s.db, &order, paymentSent, coinType, observations, false)
		if refundResult.RequiresUserInput {
			return fmt.Errorf("%w: buyer must provide a refund address before opening a dispute (%s)", models.ErrRefundAddressRequired, refundResult.Reason)
		}
		if !refundResult.Found() {
			return fmt.Errorf("%w: buyer refund address is not available yet (%s)", models.ErrRefundAddressRequired, refundResult.Reason)
		}
		payoutAddress = iwallet.NewAddress(refundResult.Address, coinType)
	} else {
		var err error
		payoutAddress, err = s.escrow.GetPayoutAddress(string(coinType))
		if err != nil {
			logger.LogErrorWithIDf(log, s.nodeID, "Failed to get payout address: %v", err)
			orderConfirmation, err := order.OrderConfirmationMessage()
			if err != nil {
				return fmt.Errorf("failed to get payout address and order confirmation: %w", err)
			}
			payoutAddress = iwallet.NewAddress(orderConfirmation.PayoutAddress, coinType)
		}
	}

	disputeOpen := &pb.DisputeOpen{
		Timestamp:      timestamppb.Now(),
		OpenedBy:       role,
		Reason:         reason,
		Contract:       serializedContract,
		PayoutAddress:  payoutAddress.String(),
		EvidenceHashes: evidenceHashes,
	}

	var disputeEvent interface{}
	if err := s.db.Update(func(tx database.Tx) error {
		disputeOpenAny := &anypb.Any{}
		if err := disputeOpenAny.MarshalFrom(disputeOpen); err != nil {
			return err
		}

		m := &npb.OrderMessage{
			OrderID:     order.ID.String(),
			MessageType: npb.OrderMessage_DISPUTE_OPEN,
			Message:     disputeOpenAny,
		}

		if err := utils.SignOrderMessage(m, s.signer); err != nil {
			return err
		}

		disputeEvent, err = s.orderProcessor.ProcessMessage(tx, m)
		if err != nil {
			return err
		}

		if len(evidenceHashes) > 0 {
			if err := tx.Update("dispute_evidence_hashes", models.StringSlice(evidenceHashes), map[string]interface{}{"id": order.ID.String()}, &models.Order{}); err != nil {
				return fmt.Errorf("failed to save dispute evidence hashes: %w", err)
			}
		}

		payload := &anypb.Any{}
		if err := payload.MarshalFrom(m); err != nil {
			return err
		}

		message1 := newMessageWithID()
		message1.MessageType = npb.Message_ORDER
		message1.Payload = payload
		if err := s.messenger.ReliablySendMessage(tx, to, message1, done1); err != nil {
			close(done1)
			close(done2)
			return err
		}

		message2 := newMessageWithID()
		message2.MessageType = npb.Message_DISPUTE
		message2.Payload = payload
		if err := s.messenger.ReliablySendMessage(tx, moderator, message2, done2); err != nil {
			close(done1)
			close(done2)
			return err
		}
		return nil
	}); err != nil {
		return err
	}
	s.emitOrderProcessorEvents(disputeEvent)
	return nil
}

// HandleDisputeMessage handles incoming dispute messages from the network.
func (s *OrderAppService) HandleDisputeMessage(from peer.ID, message *npb.Message, isDuplicate func(*npb.Message) bool, sendAck func(string, peer.ID)) error {
	defer sendAck(message.MessageID, from)

	if isDuplicate(message) {
		return nil
	}

	if message.MessageType != npb.Message_DISPUTE {
		return errors.New("message is not type DISPUTE")
	}

	order := new(npb.OrderMessage)
	if err := message.Payload.UnmarshalTo(order); err != nil {
		return err
	}

	switch order.MessageType {
	case npb.OrderMessage_DISPUTE_OPEN:
		disputeOpen := new(pb.DisputeOpen)
		if err := order.Message.UnmarshalTo(disputeOpen); err != nil {
			return err
		}

		orderOpen, err := extractOrderOpen(disputeOpen.Contract)
		if err != nil {
			return err
		}

		var (
			role           = models.RoleBuyer
			disputer       = orderOpen.BuyerID.PeerID
			disputerName   = orderOpen.BuyerID.DisplayName()
			disputerAvatar = orderOpen.BuyerID.DisplayAvatar()
			disputee       = orderOpen.Listings[0].Listing.VendorID.PeerID
			disputeeName   = orderOpen.Listings[0].Listing.VendorID.DisplayName()
			disputeeAvatar = orderOpen.Listings[0].Listing.VendorID.DisplayAvatar()
		)
		if disputeOpen.OpenedBy == pb.DisputeOpen_VENDOR {
			role = models.RoleVendor
			disputer = orderOpen.Listings[0].Listing.VendorID.PeerID
			disputerName = orderOpen.Listings[0].Listing.VendorID.DisplayName()
			disputerAvatar = orderOpen.Listings[0].Listing.VendorID.DisplayAvatar()
			disputee = orderOpen.BuyerID.PeerID
			disputeeName = orderOpen.BuyerID.DisplayName()
			disputeeAvatar = orderOpen.BuyerID.DisplayAvatar()
		}

		validationErrors, err := s.validateDisputeContract(from, disputeOpen.Contract, role)
		if err != nil {
			return err
		}

		return s.db.Update(func(dbtx database.Tx) error {
			dbtx.RegisterCommitHook(func() {
				s.eventBus.Emit(&events.CaseOpen{
					CaseID:         order.OrderID,
					DisputerID:     disputer,
					DisputerName:   disputerName,
					DisputerAvatar: disputerAvatar,
					DisputeeID:     disputee,
					DisputeeName:   disputeeName,
					DisputeeAvatar: disputeeAvatar,
					Thumbnail: events.Thumbnail{
						Tiny:  orderOpen.Listings[0].Listing.Item.Images[0].Tiny,
						Small: orderOpen.Listings[0].Listing.Item.Images[0].Small,
					},
				})
				logger.LogInfoWithIDf(log, s.nodeID, "Received new case. ID: %s", order.OrderID)
			})

			var disputeCase models.Case
			err := dbtx.Read().Where("id = ?", order.OrderID).First(&disputeCase).Error
			if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
				return err
			}

			disputeCase.ID = models.OrderID(order.OrderID)
			if err := disputeCase.PutValidationErrors(validationErrors, role); err != nil {
				return err
			}

			if disputeCase.SerializedDisputeOpen != nil {
				return fmt.Errorf("received duplicate DISPUTE_OPEN message from %s", from)
			}

			err = disputeCase.PutDisputeOpen(disputeOpen)
			if err != nil {
				return err
			}
			return dbtx.Save(&disputeCase)
		})
	case npb.OrderMessage_DISPUTE_UPDATE:
		disputeUpdate := new(pb.DisputeUpdate)
		if err := order.Message.UnmarshalTo(disputeUpdate); err != nil {
			return err
		}

		orderOpen, err := extractOrderOpen(disputeUpdate.Contract)
		if err != nil {
			return err
		}

		var (
			role           = models.RoleVendor
			disputer       = orderOpen.BuyerID.PeerID
			disputerName   = orderOpen.BuyerID.DisplayName()
			disputerAvatar = orderOpen.BuyerID.DisplayAvatar()
			disputee       = orderOpen.Listings[0].Listing.VendorID.PeerID
			disputeeName   = orderOpen.Listings[0].Listing.VendorID.DisplayName()
			disputeeAvatar = orderOpen.Listings[0].Listing.VendorID.DisplayAvatar()
		)
		if orderOpen.BuyerID.PeerID == from.String() {
			role = models.RoleBuyer
			disputer = orderOpen.Listings[0].Listing.VendorID.PeerID
			disputerName = orderOpen.Listings[0].Listing.VendorID.DisplayName()
			disputerAvatar = orderOpen.Listings[0].Listing.VendorID.DisplayAvatar()
			disputee = orderOpen.BuyerID.PeerID
			disputeeName = orderOpen.BuyerID.DisplayName()
			disputeeAvatar = orderOpen.BuyerID.DisplayAvatar()
		}

		validationErrors, err := s.validateDisputeContract(from, disputeUpdate.Contract, role)
		if err != nil {
			return err
		}

		return s.db.Update(func(dbtx database.Tx) error {
			dbtx.RegisterCommitHook(func() {
				s.eventBus.Emit(&events.CaseUpdate{
					CaseID:         order.OrderID,
					DisputerID:     disputer,
					DisputerName:   disputerName,
					DisputerAvatar: disputerAvatar,
					DisputeeID:     disputee,
					DisputeeName:   disputeeName,
					DisputeeAvatar: disputeeAvatar,
					Thumbnail: events.Thumbnail{
						Tiny:  orderOpen.Listings[0].Listing.Item.Images[0].Tiny,
						Small: orderOpen.Listings[0].Listing.Item.Images[0].Small,
					},
				})
				logger.LogInfoWithIDf(log, s.nodeID, "Received case update for case %s", order.OrderID)
			})

			var disputeCase models.Case
			err := dbtx.Read().Where("id = ?", order.OrderID).First(&disputeCase).Error
			if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
				return err
			}

			disputeCase.ID = models.OrderID(order.OrderID)

			if err := disputeCase.PutValidationErrors(validationErrors, role); err != nil {
				return err
			}

			err = disputeCase.PutDisputeUpdate(disputeUpdate)
			if err != nil {
				return err
			}
			return dbtx.Save(&disputeCase)
		})
	}
	return nil
}

func (s *OrderAppService) validateDisputeContract(from peer.ID, contract []byte, role models.OrderRole) (validationErrors []error, err error) {
	defer func() {
		if r := recover(); r != nil {
			switch x := r.(type) {
			case string:
				err = fmt.Errorf("dispute contract missing required field: %s", x)
			case error:
				err = fmt.Errorf("dispute contract missing required field: %w", x)
			default:
				err = errors.New("unknown dispute contract validation panic")
			}
		}
	}()

	orderOpen, err := extractOrderOpen(contract)
	if err != nil {
		return nil, err
	}

	paymentSent, err := extractPaymentSent(contract)
	if err != nil {
		return nil, err
	}

	peerID := orderOpen.BuyerID.PeerID
	if role == models.RoleVendor {
		peerID = orderOpen.Listings[0].Listing.VendorID.PeerID
	}

	if peerID != from.String() {
		return nil, errors.New("role address in order does not match peer that sent the message")
	}

	myPeerID := s.peerID()
	if paymentSent.Moderator != myPeerID.String() {
		return nil, errors.New("selected moderator does not match own peerID")
	}

	method, ok := payment.ResolvedPaymentMethod(nil, paymentSent)
	if !ok || !payment.MethodIsModerated(method) {
		return nil, errors.New("order payment method is not type moderated")
	}

	coinType, ok := payment.NormalizeSettlementPaymentCoin(paymentSent.Coin)
	if !ok {
		coinType = iwallet.CoinType(paymentSent.Coin)
	}
	if err := coinType.ValidateCanonicalPaymentCoin(); err != nil {
		validationErrors = append(validationErrors, fmt.Errorf("invalid payment coin: %s", err.Error()))
		return validationErrors, nil
	}

	var wal iwallet.Wallet
	settlementSpec, hasSettlementSpec := payment.ResolveSettlementSpec(nil, paymentSent)
	usesManagedValidation := hasSettlementSpec && (settlementSpec.UsesManagedEscrow() || settlementSpec.UsesSolanaEscrow())
	if usesManagedValidation {
		strategy, strategyErr := s.v2StrategyForCoin(coinType)
		if strategyErr != nil {
			return nil, fmt.Errorf("cannot validate managed dispute payment: %w", strategyErr)
		}
		if validationErr := strategy.ValidatePaymentMessage(payment.PaymentMessageParams{
			OrderOpen: orderOpen, PaymentSent: paymentSent,
			ExpectedPaymentAmount: paymentSent.Amount,
			ExpectedPaymentCoin:   paymentSent.Coin,
		}); validationErr != nil {
			validationErrors = append(validationErrors, fmt.Errorf("order payment is invalid: %s", validationErr.Error()))
		}
	} else {
		wal, err = s.multiwallet.WalletForCurrencyCode(string(coinType))
		if err != nil {
			return nil, fmt.Errorf("cannot validate order. coin not supported by moderator. %w", err)
		}
	}

	for i, listing := range orderOpen.Listings {
		if s.listings != nil {
			if err := s.listings.ValidateListing(listing); err != nil {
				validationErrors = append(validationErrors, fmt.Errorf("listing %d in contract is invalid: %s", i, err.Error()))
			}
		}
	}

	var escrowTimeoutHours uint32
	for i, item := range orderOpen.Items {
		listing, err := utils.ExtractListing(item.ListingHash, orderOpen.Listings)
		if err != nil {
			validationErrors = append(validationErrors, fmt.Errorf("order does not contain any listings that match the listing ID for item %d", i))
			continue
		}

		if listing.Metadata.EscrowTimeoutHours > escrowTimeoutHours {
			escrowTimeoutHours = listing.Metadata.EscrowTimeoutHours
		}
	}

	if err := utils.ValidateBuyerID(orderOpen.BuyerID); err != nil {
		validationErrors = append(validationErrors, fmt.Errorf("invalid buyer ID in order: %s", err.Error()))
	}

	// For cross-currency payments (e.g. USD order paid in BTC), order.Amount is in
	// pricing currency units while paymentSent.Amount is in payment currency units —
	// comparing them is meaningless. The actual paid amount is enforced on-chain
	// (EVM/Solana escrow contract, UTXO multisig address) and the moderator should
	// verify against the chain, not the self-reported message amounts.
	pricingCoin := strings.ToUpper(strings.TrimSpace(orderOpen.PricingCoin))
	paymentCoin, priceErr := coinType.PricingCurrencyCode()
	if priceErr != nil {
		validationErrors = append(validationErrors, fmt.Errorf("invalid payment coin: %s", priceErr.Error()))
	} else {
		isCrossCurrency := pricingCoin != "" && pricingCoin != paymentCoin
		if !isCrossCurrency && usesManagedValidation {
			if err := utils.ValidatePaymentAmount(orderOpen.Amount, paymentSent.Amount); err != nil {
				validationErrors = append(validationErrors, fmt.Errorf("order payment is invalid: %s", err.Error()))
			}
		} else if !isCrossCurrency {
			if err := utils.ValidatePayment(orderOpen, paymentSent, escrowTimeoutHours, wal); err != nil {
				validationErrors = append(validationErrors, fmt.Errorf("order payment is invalid: %s", err.Error()))
			}
		}
	}

	return validationErrors, nil
}

// CloseDispute sends a disputeClose message and resolution to both the buyer and the vendor.
func (s *OrderAppService) CloseDispute(orderID models.OrderID, buyerPercentage, vendorPercentage float32, resolution string, done chan struct{}) error {
	if err := s.acquireOrderLock(orderID); err != nil {
		return fmt.Errorf("failed to acquire order lock for %s: %w", orderID, err)
	}
	defer s.releaseOrderLock(orderID)

	done1, done2 := make(chan struct{}), make(chan struct{})
	go func() {
		if done != nil {
			timeout := time.After(2 * time.Minute)
			select {
			case <-done1:
			case <-timeout:
				close(done)
				return
			}
			select {
			case <-done2:
			case <-timeout:
			}
			close(done)
		}
	}()

	var payDivision = models.PayoutRatio{Buyer: buyerPercentage, Vendor: vendorPercentage}
	if err := payDivision.Validate(); err != nil {
		return err
	}

	disputeCase, err := s.GetCase(orderID.String())
	if err != nil {
		return fmt.Errorf("failed to get case with orderID %s: %w", orderID, err)
	}

	if disputeCase.SerializedDisputeOpen == nil {
		return errors.New("failed to find dispute open info")
	}

	if disputeCase.SerializedBuyerContract == nil && disputeCase.SerializedVendorContract == nil {
		return errors.New("failed to get any of buyer and vendor dispute update info")
	}

	if disputeCase.SerializedDisputeClose != nil {
		return fmt.Errorf("%w: the dispute has already been closed", coreiface.ErrBadRequest)
	}

	if disputeCase.SerializedVendorContract == nil && vendorPercentage > 0 {
		return errors.New("vendor must provide his copy of the contract before you can release funds to the vendor")
	}

	if disputeCase.SerializedBuyerContract == nil {
		disputeCase.SerializedBuyerContract = disputeCase.SerializedVendorContract
	}

	preferredContract, err := disputeCase.ResolutionPaymentContract(payDivision)
	if err != nil {
		return fmt.Errorf("failed to get preferred contract from case %s: %w", disputeCase.ID.String(), err)
	}

	orderOpen := preferredContract.GetOrderOpen()

	buyer, err := peer.Decode(orderOpen.BuyerID.PeerID)
	if err != nil {
		return fmt.Errorf("failed to get buyer id: %w", err)
	}

	vendor, err := peer.Decode(orderOpen.Listings[0].Listing.VendorID.PeerID)
	if err != nil {
		return fmt.Errorf("failed to get vendor id: %w", err)
	}

	paymentSent := preferredContract.GetPaymentSent()
	if paymentSent == nil {
		return fmt.Errorf("%w: dispute contract is missing payment sent", coreiface.ErrBadRequest)
	}

	settlementSpec, hasSettlementSpec := payment.ResolveSettlementSpec(nil, paymentSent)
	usesBalanceEscrow := hasSettlementSpec && (settlementSpec.UsesManagedEscrow() || settlementSpec.UsesSolanaEscrow())
	coinType, ok := payment.NormalizeSettlementPaymentCoin(paymentSent.Coin)
	if !ok {
		coinType = iwallet.CoinType(paymentSent.Coin)
	}

	var txs []*pb.Contract_Transaction
	if !usesBalanceEscrow {
		txs = disputeReleaseOutpointsFromFundingFacts(paymentSent, coinType, paymentSent.ToAddress)
		if len(txs) == 0 {
			txs = preferredContract.GetTransactions()
		}
		if len(txs) == 0 {
			txs = s.disputeReleaseOutpointsFromLocalOrder(orderID, paymentSent)
		}
	} else {
		txs = preferredContract.GetTransactions()
	}

	totalOut := iwallet.NewAmount(0)
	for _, tx := range txs {
		totalOut = totalOut.Add(iwallet.NewAmount(tx.GetValue()))
	}

	if len(txs) == 0 && usesBalanceEscrow {
		// Address-monitored smart escrows do not always project legacy
		// transaction rows into dispute case snapshots. In that route,
		// PaymentSent.Amount is the canonical funded value.
		totalOut = iwallet.NewAmount(paymentSent.Amount)
	}
	if len(txs) == 0 && !usesBalanceEscrow {
		return fmt.Errorf("%w: dispute funding outpoints are missing for order %s", coreiface.ErrBadRequest, orderID)
	}

	disputeStrategy, err := s.v2StrategyForCoin(coinType)
	if err != nil {
		return fmt.Errorf("no chain escrow for coin %s: %w", coinType, err)
	}

	nInputs := len(txs)
	if nInputs < 1 {
		nInputs = 1
	}
	totalFee, err := disputeStrategy.EstimateEscrowFee(string(coinType), nInputs, 3, iwallet.FlNormal)
	if err != nil {
		return fmt.Errorf("failed to estimate escrow fee: %w", err)
	}
	if hasSettlementSpec && settlementSpec.UsesSolanaEscrow() {
		totalFee = iwallet.NewAmount(0)
	}

	totalOut = totalOut.Sub(totalFee)

	modAddr, err := s.escrow.GetPayoutAddress(string(coinType))
	if err != nil {
		return err
	}
	modValue, err := s.moderators.GetModeratorFee(totalOut, string(coinType))
	if err != nil {
		return fmt.Errorf("failed to get moderator fee: %w", err)
	}

	effectiveVal := totalOut.Sub(modValue)
	buyerValue := iwallet.NewAmount(0)
	if payDivision.BuyerAny() {
		buyerValue = effectiveVal.Mul(iwallet.NewAmount(int32(buyerPercentage)))
		buyerValue = buyerValue.Div(iwallet.NewAmount(100))
	}

	vendorValue := iwallet.NewAmount(0)
	if payDivision.VendorAny() {
		vendorValue = effectiveVal.Sub(buyerValue)
	}

	moderatedEscrowRelease := pb.DisputeClose_ModeratedEscrowRelease{
		BuyerAddress:     disputeCase.BuyerPayoutAddress,
		BuyerAmount:      buyerValue.String(),
		VendorAddress:    disputeCase.VendorPayoutAddress,
		VendorAmount:     vendorValue.String(),
		ModeratorAddress: modAddr.String(),
		ModeratorAmount:  modValue.String(),
		TransactionFee:   totalFee.String(),
	}
	if err := validateDisputePayoutAddresses(&moderatedEscrowRelease); err != nil {
		return err
	}

	var txn iwallet.Transaction

	for _, tx := range txs {
		moderatedEscrowRelease.Outpoints = append(moderatedEscrowRelease.Outpoints, &pb.Outpoint{
			FromID: tx.FromID,
			Value:  tx.Value,
		})
	}
	if err := payment.ValidateDisputeReleaseFunding(&moderatedEscrowRelease, paymentSent); err != nil {
		return fmt.Errorf("invalid dispute release funding: %w", err)
	}

	affiliatePayout, err := affiliatePayoutForDisputeSettlement(coinType, preferredContract.GetOrderShipments(), &moderatedEscrowRelease)
	if err != nil {
		return fmt.Errorf("read seller-signed dispute affiliate payout: %w", err)
	}
	if err := requireInterimAffiliateDisputePayout(orderOpen, &moderatedEscrowRelease, affiliatePayout); err != nil {
		return fmt.Errorf("seller-signed dispute affiliate payout is required: %w", err)
	}
	buildAffiliatePayout := affiliatePayout
	if coinInfo, coinErr := iwallet.CoinInfoFromCoinType(coinType); coinErr != nil || !coinInfo.Chain.IsUTXOChain() {
		buildAffiliatePayout = nil
	}
	txn, err = s.BuildDisputeReleaseTransaction(&moderatedEscrowRelease, paymentSent, buildAffiliatePayout)
	if err != nil {
		return fmt.Errorf("failed to build release transaction: %w", err)
	}

	orderData, err := orderDataWithContract(orderID, orderOpen, paymentSent)
	if err != nil {
		return fmt.Errorf("failed to materialize dispute order data for settlement signing: %w", err)
	}
	if settlementSigs, handled, err := s.signSettlementActionRelease(context.Background(), coinType, "dispute_release", payment.ActionParams{
		OrderID:         orderID.String(),
		PaymentCoin:     string(coinType),
		PaymentAmount:   paymentSent.Amount,
		Chaincode:       paymentSent.Chaincode,
		Script:          paymentSent.Script,
		OrderData:       orderData,
		ReleaseInfo:     &moderatedEscrowRelease,
		AffiliatePayout: affiliatePayout,
	}); handled {
		if err != nil {
			return fmt.Errorf("failed to sign settlement dispute release action: %w", err)
		}
		moderatedEscrowRelease.EscrowSignatures = append(moderatedEscrowRelease.EscrowSignatures, settlementSigs...)
	} else {
		if err := errBalanceMonitoredEscrowRequiresSettlementAction(orderData, paymentSent, "dispute_release"); err != nil {
			return err
		}
		legacyStrategy, err := s.v2StrategyForCoin(coinType)
		if err != nil {
			return fmt.Errorf("no chain escrow for coin %s: %w", coinType, err)
		}

		script, err := hex.DecodeString(paymentSent.Script)
		if err != nil {
			return fmt.Errorf("failed to decode payment script: %w", err)
		}

		chainCode, err := hex.DecodeString(paymentSent.Chaincode)
		if err != nil {
			return fmt.Errorf("failed to decode payment chaincode: %w", err)
		}

		sigs, err := legacyStrategy.SignEscrowRelease(context.Background(), payment.SignEscrowParams{
			Transaction: txn,
			Script:      script,
			ChainCode:   chainCode,
			CoinCode:    string(coinType),
		})
		if err != nil {
			return fmt.Errorf("failed to sign escrow release: %w", err)
		}

		for _, sig := range sigs {
			moderatedEscrowRelease.EscrowSignatures = append(moderatedEscrowRelease.EscrowSignatures, &pb.Signature{
				Signature: sig.Signature,
				Index:     uint32(sig.Index),
			})
		}
	}

	disputeClose := &pb.DisputeClose{
		Timestamp:   timestamppb.Now(),
		Verdict:     resolution,
		ReleaseInfo: &moderatedEscrowRelease,
	}
	logger.LogInfoWithIDf(log, s.nodeID, "Dispute resolution: %v", disputeClose)

	return s.db.Update(func(dbtx database.Tx) error {
		disputeCloseAny := &anypb.Any{}
		if err := disputeCloseAny.MarshalFrom(disputeClose); err != nil {
			return err
		}

		m := &npb.OrderMessage{
			OrderID:     orderID.String(),
			MessageType: npb.OrderMessage_DISPUTE_CLOSE,
			Message:     disputeCloseAny,
		}

		if err := utils.SignOrderMessage(m, s.signer); err != nil {
			return err
		}

		payload := &anypb.Any{}
		if err := payload.MarshalFrom(m); err != nil {
			return err
		}

		message1 := newMessageWithID()
		message1.MessageType = npb.Message_ORDER
		message1.Payload = payload
		if err := s.messenger.ReliablySendMessage(dbtx, buyer, message1, done1); err != nil {
			close(done1)
			close(done2)
			return err
		}

		message2 := newMessageWithID()
		message2.MessageType = npb.Message_ORDER
		message2.Payload = payload
		if err := s.messenger.ReliablySendMessage(dbtx, vendor, message2, done2); err != nil {
			close(done1)
			close(done2)
			return err
		}

		err = disputeCase.PutDisputeClose(disputeClose)
		if err != nil {
			return err
		}
		return dbtx.Save(disputeCase)
	})
}

func validateDisputePayoutAddresses(release *pb.DisputeClose_ModeratedEscrowRelease) error {
	if release == nil {
		return fmt.Errorf("%w: dispute release info is missing", coreiface.ErrBadRequest)
	}
	zero := iwallet.NewAmount(0)
	if iwallet.NewAmount(release.BuyerAmount).Cmp(zero) > 0 && strings.TrimSpace(release.BuyerAddress) == "" {
		return fmt.Errorf("%w: buyer payout address is required when buyer payout amount is greater than zero", coreiface.ErrBadRequest)
	}
	if iwallet.NewAmount(release.VendorAmount).Cmp(zero) > 0 && strings.TrimSpace(release.VendorAddress) == "" {
		return fmt.Errorf("%w: vendor payout address is required when vendor payout amount is greater than zero", coreiface.ErrBadRequest)
	}
	if iwallet.NewAmount(release.ModeratorAmount).Cmp(zero) > 0 && strings.TrimSpace(release.ModeratorAddress) == "" {
		return fmt.Errorf("%w: moderator payout address is required when moderator payout amount is greater than zero", coreiface.ErrBadRequest)
	}
	return nil
}

func (s *OrderAppService) getOrderAndPaymentInfo(orderID models.OrderID) (*models.Order, *pb.OrderOpen, *pb.PaymentSent, *pb.DisputeClose, error) {
	var order models.Order
	err := s.db.View(func(tx database.Tx) error {
		return tx.Read().Where("id = ?", orderID.String()).Find(&order).Error
	})
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("failed to get order: %w", err)
	}

	orderOpen, err := order.OrderOpenMessage()
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("failed to get order open message: %w", err)
	}

	paymentSent, err := order.PaymentSentMessage()
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("failed to get payment sent message: %w", err)
	}
	if _, err := payment.SettlementCoinFromPaymentSent(paymentSent); err != nil {
		return nil, nil, nil, nil, err
	}

	disputeClose, err := order.DisputeClosedMessage()
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("failed to get dispute close message: %w", err)
	}

	return &order, orderOpen, paymentSent, disputeClose, nil
}

func (s *OrderAppService) BuildDisputeReleaseTransaction(releaseInfo *pb.DisputeClose_ModeratedEscrowRelease, paymentSent *pb.PaymentSent, affiliatePayout ...*models.AffiliateSettlementPayout) (iwallet.Transaction, error) {
	var txn iwallet.Transaction
	if paymentSent == nil {
		return txn, errors.New("payment sent is missing")
	}
	if err := payment.ValidateDisputeReleaseFunding(releaseInfo, paymentSent); err != nil {
		return txn, err
	}
	coinType, ok := payment.NormalizeSettlementPaymentCoin(paymentSent.Coin)
	if !ok {
		coinType = iwallet.CoinType(normalizeCurrencyCode(paymentSent.Coin))
	}

	for _, output := range releaseInfo.Outpoints {
		txn.From = append(txn.From, iwallet.SpendInfo{ID: output.FromID, Amount: iwallet.NewAmount(output.Value)})
	}

	if iwallet.NewAmount(releaseInfo.BuyerAmount).Cmp(iwallet.NewAmount(0)) > 0 {
		txn.To = append(txn.To, iwallet.SpendInfo{
			Address: iwallet.NewAddress(releaseInfo.BuyerAddress, coinType),
			Amount:  iwallet.NewAmount(releaseInfo.BuyerAmount),
		})
	}

	vendorOutputIndex := -1
	if iwallet.NewAmount(releaseInfo.VendorAmount).Cmp(iwallet.NewAmount(0)) > 0 {
		vendorOutputIndex = len(txn.To)
		txn.To = append(txn.To, iwallet.SpendInfo{
			Address: iwallet.NewAddress(releaseInfo.VendorAddress, coinType),
			Amount:  iwallet.NewAmount(releaseInfo.VendorAmount),
		})
	}

	if iwallet.NewAmount(releaseInfo.ModeratorAmount).Cmp(iwallet.NewAmount(0)) > 0 {
		txn.To = append(txn.To, iwallet.SpendInfo{
			Address: iwallet.NewAddress(releaseInfo.ModeratorAddress, coinType),
			Amount:  iwallet.NewAmount(releaseInfo.ModeratorAmount),
		})
	}
	if len(affiliatePayout) > 0 && affiliatePayout[0] != nil {
		payout := affiliatePayout[0]
		if vendorOutputIndex < 0 || iwallet.NewAmount(payout.Amount).Cmp(txn.To[vendorOutputIndex].Amount) >= 0 {
			return txn, models.ErrInvalidSellerAffiliate
		}
		txn.To[vendorOutputIndex].Amount = txn.To[vendorOutputIndex].Amount.Sub(iwallet.NewAmount(payout.Amount))
		txn.To = append(txn.To, iwallet.SpendInfo{
			Address: iwallet.NewAddress(payout.Address, coinType),
			Amount:  iwallet.NewAmount(payout.Amount),
		})
	}

	return txn, nil
}

func (s *OrderAppService) signAndSendReleaseTransaction(txn *iwallet.Transaction, paymentSent *pb.PaymentSent, disputeClose *pb.DisputeClose) error {
	script, err := hex.DecodeString(paymentSent.Script)
	if err != nil {
		return fmt.Errorf("failed to decode payment script: %w", err)
	}

	chainCode, err := hex.DecodeString(paymentSent.Chaincode)
	if err != nil {
		return fmt.Errorf("failed to decode payment chaincode: %w", err)
	}

	coinType, ok := payment.NormalizeSettlementPaymentCoin(paymentSent.Coin)
	if !ok {
		coinType = iwallet.CoinType(paymentSent.Coin)
	}
	wallet, err := s.multiwallet.WalletForCurrencyCode(string(coinType))
	if err != nil {
		return fmt.Errorf("cannot validate order. coin not supported by moderator:%s, %w", coinType, err)
	}

	escrowMasterKey, err := s.keyProvider.EscrowMasterKey()
	if err != nil {
		return fmt.Errorf("failed to get escrow master key: %w", err)
	}

	signingKey, err := utils.GenerateEscrowPrivateKey(escrowMasterKey, chainCode)
	if err != nil {
		return fmt.Errorf("failed to generate moderator key: %w", err)
	}

	escrowWallet, ok := wallet.(iwallet.UTXOEscrow)
	if !ok {
		return errors.New("failed to get escrowWallet")
	}

	mySigs, err := escrowWallet.SignMultisigTransaction(*txn, *signingKey, script)
	if err != nil {
		return fmt.Errorf("failed to generate my signature: %w", err)
	}

	var moderatorSigs []iwallet.EscrowSignature
	for _, sig := range disputeClose.ReleaseInfo.EscrowSignatures {
		s := iwallet.EscrowSignature{
			Signature: sig.Signature,
			Index:     int(sig.Index),
		}
		moderatorSigs = append(moderatorSigs, s)
	}

	wtx, err := wallet.Begin()
	if err != nil {
		return err
	}

	txid, err := escrowWallet.BuildAndSend(wtx, *txn, [][]iwallet.EscrowSignature{mySigs, moderatorSigs}, script, iwallet.ORDER_FINISH_RESOLVED)
	if err != nil {
		return err
	}

	if err := wtx.Commit(); err != nil {
		return err
	}

	txn.ID = txid
	txn.Timestamp = time.Now()

	return nil
}

// ReleaseFunds releases funds from dispute escrow.
func (s *OrderAppService) ReleaseFunds(orderID models.OrderID, txid iwallet.TransactionID, done chan struct{}) error {
	if err := s.requireDisputeReleaseParticipant(orderID); err != nil {
		return err
	}

	order, orderOpen, paymentSent, disputeClose, err := s.getOrderAndPaymentInfo(orderID)
	if err != nil {
		return fmt.Errorf("get order payment info for %s: %w", orderID, err)
	}

	if err := s.attachSettlementActions(order); err != nil {
		return fmt.Errorf("load settlement actions for order %s: %w", orderID, err)
	}

	shipments, err := order.OrderShipmentMessages()
	if err != nil && !models.IsMessageNotExistError(err) {
		return fmt.Errorf("load seller shipment payout terms: %w", err)
	}
	if models.IsMessageNotExistError(err) {
		shipments = nil
	}
	coinType, err := payment.SettlementCoinFromPaymentSent(paymentSent)
	if err != nil {
		return err
	}
	affiliatePayout, err := affiliatePayoutForDisputeSettlement(coinType, shipments, disputeClose.ReleaseInfo)
	if err != nil {
		return fmt.Errorf("read seller-signed dispute affiliate payout: %w", err)
	}
	if err := requireInterimAffiliateDisputePayout(orderOpen, disputeClose.ReleaseInfo, affiliatePayout); err != nil {
		return fmt.Errorf("seller-signed dispute affiliate payout is required: %w", err)
	}
	releaseTx, err := s.BuildDisputeReleaseTransaction(disputeClose.ReleaseInfo, paymentSent, affiliatePayout)
	if err != nil {
		return fmt.Errorf("build dispute release tx: %w", err)
	}

	if !orderRequiresMonitoredSettlementActions(order, paymentSent, coinType, s.paymentRegistry) {
		return errRetiredClientSignedModeratedSettlement("dispute_release")
	}
	if _, err := requireBackendSubmittedSettlementSpec(order, paymentSent); err != nil {
		return err
	}

	var releaseAlreadySubmitted bool
	txid, releaseAlreadySubmitted, err = evaluateMonitoredSettlementRelease(order, txid, "dispute_release")
	if err != nil {
		return err
	}
	if !releaseAlreadySubmitted {
		return errSettlementReleaseActionRequired(orderID, "dispute_release")
	}
	releaseTx.ID = txid

	buyer, err := order.Buyer()
	if err != nil {
		return err
	}
	vendor, err := order.Vendor()
	if err != nil {
		return err
	}
	var (
		to   = vendor
		from = buyer
	)
	if order.Role() == models.RoleVendor {
		to = buyer
		from = vendor
	}

	disputeAccept := &pb.DisputeAccept{
		Timestamp: timestamppb.Now(),
		ClosedBy:  from.String(),
		Txid:      string(txid),
	}

	disputeAcceptAny := &anypb.Any{}
	if err := disputeAcceptAny.MarshalFrom(disputeAccept); err != nil {
		return err
	}

	m := &npb.OrderMessage{
		OrderID:     order.ID.String(),
		MessageType: npb.OrderMessage_DISPUTE_ACCEPT,
		Message:     disputeAcceptAny,
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

	var disputeEvent interface{}
	if err := s.db.Update(func(tx database.Tx) error {
		disputeEvent, err = s.orderProcessor.ProcessMessage(tx, m)
		if err != nil {
			return err
		}

		if err := saveTransactionToFreshOrder(tx, order.ID, releaseTx); err != nil {
			return err
		}

		return s.messenger.ReliablySendMessage(tx, to, message, done)
	}); err != nil {
		return err
	}
	s.emitOrderProcessorEvents(disputeEvent)
	return nil
}

func (s *OrderAppService) requireDisputeReleaseParticipant(orderID models.OrderID) error {
	var order models.Order
	if err := s.db.View(func(tx database.Tx) error {
		return tx.Read().Select("id", "my_role").Where("id = ?", orderID.String()).First(&order).Error
	}); err != nil {
		return fmt.Errorf("failed to get order role: %w", err)
	}

	switch order.Role() {
	case models.RoleBuyer, models.RoleVendor:
		return nil
	case models.RoleModerator:
		return fmt.Errorf("%w: moderator must resolve disputes via close dispute, not release funds", coreiface.ErrBadRequest)
	default:
		return fmt.Errorf("%w: dispute release requires buyer or vendor role, got %s", coreiface.ErrBadRequest, order.Role())
	}
}

// ReleaseFundsAfterTimeout releases escrow funds after dispute timeout.
func (s *OrderAppService) ReleaseFundsAfterTimeout(orderID models.OrderID, done chan struct{}) error {
	var order models.Order
	err := s.db.View(func(tx database.Tx) error {
		return tx.Read().Where("id = ?", orderID.String()).Find(&order).Error
	})
	if err != nil {
		return fmt.Errorf("load order %s for timeout release: %w", orderID, err)
	}

	state := order.State
	if state != models.OrderState_PENDING && state != models.OrderState_SHIPPED && state != models.OrderState_DISPUTED {
		return fmt.Errorf("release escrow can only be called when sale is pending, shipped, or disputed, state: %s", state)
	}

	disputeTimeout, err := s.disputeIsTimeout(&order)
	if err != nil {
		return fmt.Errorf("check dispute timeout: %w", err)
	}
	if !disputeTimeout {
		return errors.New("release escrow can only be called after dispute has expired")
	}

	paymentSent, err := order.PaymentSentMessage()
	if err != nil {
		return fmt.Errorf("payment sent message: %w", err)
	}
	coinType, err := payment.SettlementCoinFromPaymentSent(paymentSent)
	if err != nil {
		return fmt.Errorf("canonical payment coin: %w", err)
	}

	wallet, err := s.multiwallet.WalletForCurrencyCode(string(coinType))
	if err != nil {
		return fmt.Errorf("cannot validate order. coin not supported by moderator:%s, %w", coinType, err)
	}

	escrowTimeoutWallet, walletSupportsEscrowTimeout := wallet.(iwallet.UTXOEscrowWithTimeout)
	if !walletSupportsEscrowTimeout {
		return fmt.Errorf("wallet cannot support escrow timeout, coin: %s", coinType)
	}

	script, err := hex.DecodeString(paymentSent.Script)
	if err != nil {
		return fmt.Errorf("decode escrow script: %w", err)
	}

	chainCode, err := hex.DecodeString(paymentSent.Chaincode)
	if err != nil {
		return fmt.Errorf("decode chaincode: %w", err)
	}

	escrowMasterKey, err := s.keyProvider.EscrowMasterKey()
	if err != nil {
		return fmt.Errorf("escrow master key: %w", err)
	}

	vendorKey, err := utils.GenerateEscrowPrivateKey(escrowMasterKey, chainCode)
	if err != nil {
		return fmt.Errorf("generate escrow private key: %w", err)
	}

	txs, err := order.GetTransactions()
	if err != nil {
		return fmt.Errorf("get order transactions: %w", err)
	}

	var txn iwallet.Transaction

	totalOut := iwallet.NewAmount(0)
	for _, tx := range txs {
		for _, to := range tx.To {
			if payment.SameUTXOAddress(to.Address.String(), order.PaymentAddress) {
				txn.From = append(txn.From, to)
				totalOut = totalOut.Add(to.Amount)
			}
		}
	}

	escrowWallet, ok := wallet.(iwallet.UTXOEscrow)
	if !ok {
		return errors.New("failed to get escrowWallet")
	}

	totalFee, err := escrowWallet.EstimateEscrowFee(len(txn.From), 1, 1, iwallet.FlNormal)
	if err != nil {
		return errors.New("failed to estimate escrow fee")
	}

	totalOut = totalOut.Sub(totalFee)

	payoutAddress, err := s.escrow.GetPayoutAddress(string(coinType))
	if err != nil {
		return err
	}

	txn.To = append(txn.To, iwallet.SpendInfo{
		Address: payoutAddress,
		Amount:  totalOut,
	})

	wtx, err := wallet.Begin()
	if err != nil {
		return err
	}

	var finishType iwallet.OrderFinishType
	switch state {
	case models.OrderState_PENDING:
		finishType = iwallet.ORDER_FINISH_CANCEL
	case models.OrderState_SHIPPED:
		finishType = iwallet.ORDER_FINISH_COMPLETE
	case models.OrderState_DISPUTED:
		finishType = iwallet.ORDER_FINISH_RESOLVED
	}
	txid, err := escrowTimeoutWallet.ReleaseFundsAfterTimeout(wtx, txn, *vendorKey, script, finishType)
	if err != nil {
		return err
	}
	logger.LogInfoWithIDf(log, s.nodeID, "Release funds after timeout, order ID: %s, tx ID: %s", orderID, txid)

	err = wtx.Commit()
	if err != nil {
		return err
	}

	txn.ID = txid
	txn.Timestamp = time.Now()

	paymentFinalized := &pb.PaymentFinalized{}

	buyer, err := order.Buyer()
	if err != nil {
		return err
	}
	paymentFinalizedAny := &anypb.Any{}
	if err := paymentFinalizedAny.MarshalFrom(paymentFinalized); err != nil {
		return err
	}

	m := &npb.OrderMessage{
		OrderID:     order.ID.String(),
		MessageType: npb.OrderMessage_PAYMENT_FINALIZED,
		Message:     paymentFinalizedAny,
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

	var finalizedEvent interface{}
	if err := s.db.Update(func(dbtx database.Tx) error {
		finalizedEvent, err = s.orderProcessor.ProcessMessage(dbtx, m)
		if err != nil {
			return err
		}

		if err := saveTransactionToFreshOrder(dbtx, order.ID, txn); err != nil {
			return err
		}

		return s.messenger.ReliablySendMessage(dbtx, buyer, message, done)
	}); err != nil {
		return err
	}
	s.emitOrderProcessorEvents(finalizedEvent)
	return nil
}

func (s *OrderAppService) disputeIsTimeout(order *models.Order) (bool, error) {
	if !order.UnderActiveDispute() {
		return false, nil
	}

	disputeTimeout := time.Duration(DisputeTimeout) * time.Hour
	if s.testnet {
		disputeTimeout = time.Duration(10) * time.Second
	}

	disputeOpen, err := order.DisputeOpenMessage()
	if err != nil {
		return false, err
	}
	disputeStart := disputeOpen.Timestamp.AsTime()
	disputeExpiration := disputeStart.Add(disputeTimeout)
	return time.Now().After(disputeExpiration), nil
}

// GetReleaseFundsInstructions preserves the legacy client-signed dispute
// release surface.
func (s *OrderAppService) GetReleaseFundsInstructions(orderID models.OrderID, initiatorAddress string) (coinType iwallet.CoinType, instructions any, err error) {
	return s.GetLegacyReleaseFundsInstructions(orderID, initiatorAddress)
}

// GetLegacyReleaseFundsInstructions is the internal legacy-only dispute
// release instructions path. backend-managed moderated payouts must stay on the
// backend close/release flow instead of the old instruction contract.
func (s *OrderAppService) GetLegacyReleaseFundsInstructions(orderID models.OrderID, initiatorAddress string) (coinType iwallet.CoinType, instructions any, err error) {
	order, _, paymentSent, _, err := s.getOrderAndPaymentInfo(orderID)
	if err != nil {
		return "", nil, err
	}

	coinType, err = payment.SettlementCoinFromPaymentSent(paymentSent)
	if err != nil {
		return "", nil, err
	}

	method, ok := payment.ResolvedPaymentMethod(order, paymentSent)
	if ok && payment.MethodIsModerated(method) {
		return coinType, nil, fmt.Errorf("%w: moderated dispute release uses POST /v1/orders/{orderID}/settlement-actions/dispute-release",
			coreiface.ErrBadRequest)
	}

	if !orderRequiresClientSignedInstructions(order, paymentSent) {
		return coinType, nil, nil
	}

	strategy, err := s.v2StrategyForCoin(coinType)
	if err != nil {
		return coinType, nil, fmt.Errorf("no client-signed settlement strategy for coin %s: %w", coinType, err)
	}

	result, err := strategy.DisputeRelease(context.Background(), payment.ActionParams{
		OrderID:       orderID.String(),
		InitiatorAddr: initiatorAddress,
		OrderData:     order,
	})
	if err != nil {
		return coinType, nil, err
	}

	return coinType, result.Instructions, nil
}

// RequestAddress requests a fresh payment address from the remote peer.
func (s *OrderAppService) RequestAddress(ctx context.Context, to peer.ID, coinType iwallet.CoinType) (iwallet.Address, error) {
	addrReq := npb.AddressRequestMessage{
		Coin: coinType.CurrencyCode(),
	}

	payload := &anypb.Any{}
	if err := payload.MarshalFrom(&addrReq); err != nil {
		return iwallet.Address{}, err
	}

	message := newMessageWithID()
	message.MessageType = npb.Message_ADDRESS_REQUEST
	message.Payload = payload

	sub, err := s.eventBus.Subscribe(&events.AddressRequestResponse{})
	if err != nil {
		return iwallet.Address{}, err
	}
	defer sub.Close()

	ctx, cancel := context.WithTimeout(ctx, addressRequestTimeout)
	defer cancel()

	go s.networkService.SendMessage(ctx, to, message)

	for {
		select {
		case resp := <-sub.Out():
			addrReqResp := resp.(*events.AddressRequestResponse)

			if addrReqResp.PeerID != to.String() || normalizeCurrencyCode(addrReqResp.Coin) != coinType.CurrencyCode() {
				continue
			}

			return iwallet.NewAddress(addrReqResp.Address, coinType), nil
		case <-time.After(addressRequestTimeout):
			return iwallet.Address{}, ErrNoResponse
		case <-ctx.Done():
			return iwallet.Address{}, ErrNoResponse
		}
	}
}

// HandleAddressRequest handles ADDRESS_REQUEST messages.
func (s *OrderAppService) HandleAddressRequest(from peer.ID, message *npb.Message) error {
	if message.MessageType != npb.Message_ADDRESS_REQUEST {
		return errors.New("message is not type ADDRESS_REQUEST")
	}

	req := new(npb.AddressRequestMessage)
	if err := message.Payload.UnmarshalTo(req); err != nil {
		return err
	}

	addr, err := s.escrow.GetPayoutAddress(req.Coin)
	if err != nil {
		return err
	}

	addrResp := npb.AddressResponseMessage{
		Address: addr.String(),
		Coin:    addr.CoinType().CurrencyCode(),
	}

	respPayload := &anypb.Any{}
	if err := respPayload.MarshalFrom(&addrResp); err != nil {
		return err
	}

	resp := newMessageWithID()
	resp.MessageType = npb.Message_ADDRESS_RESPONSE
	resp.Payload = respPayload

	return s.networkService.SendMessage(context.Background(), from, resp)
}

// HandleAddressResponse handles ADDRESS_RESPONSE messages.
func (s *OrderAppService) HandleAddressResponse(from peer.ID, message *npb.Message) error {
	if message.MessageType != npb.Message_ADDRESS_RESPONSE {
		return errors.New("message is not type ADDRESS_RESPONSE")
	}

	resp := new(npb.AddressResponseMessage)
	if err := message.Payload.UnmarshalTo(resp); err != nil {
		return err
	}

	s.eventBus.Emit(&events.AddressRequestResponse{
		PeerID:  from.String(),
		Address: resp.Address,
		Coin:    resp.Coin,
	})
	return nil
}

const (
	addressRequestTimeout = time.Second * 3
)

// ErrNoResponse indicates a failed address request due to the remote peer not responding.
var ErrNoResponse = errors.New("no response to address request from peer")

func extractOrderOpen(contract []byte) (*pb.OrderOpen, error) {
	var c pb.Contract
	if err := proto.Unmarshal(contract, &c); err != nil {
		return nil, err
	}
	return c.OrderOpen, nil
}

func extractPaymentSent(contract []byte) (*pb.PaymentSent, error) {
	var c pb.Contract
	if err := proto.Unmarshal(contract, &c); err != nil {
		return nil, err
	}
	return c.PaymentSent, nil
}

func (s *OrderAppService) disputeReleaseOutpointsFromLocalOrder(orderID models.OrderID, paymentSent *pb.PaymentSent) []*pb.Contract_Transaction {
	if paymentSent == nil {
		return nil
	}

	paymentAddress := strings.TrimSpace(paymentSent.ToAddress)
	if paymentAddress == "" {
		return nil
	}

	var order models.Order
	if err := s.db.View(func(tx database.Tx) error {
		return tx.Read().Where("id = ?", orderID.String()).First(&order).Error
	}); err != nil {
		logger.LogWarningWithIDf(log, s.nodeID, "Unable to load local order %s for dispute outpoint recovery: %v", orderID, err)
		return nil
	}

	if order.PaymentAddress != "" {
		paymentAddress = strings.TrimSpace(order.PaymentAddress)
	}

	orderTxs, err := order.GetTransactions()
	if err != nil {
		if !models.IsMessageNotExistError(err) {
			logger.LogWarningWithIDf(log, s.nodeID, "Unable to load local transactions for dispute outpoint recovery on order %s: %v", orderID, err)
		}
		return nil
	}

	var outpoints []*pb.Contract_Transaction
	for _, tx := range orderTxs {
		for _, to := range tx.To {
			if !payment.SameUTXOAddress(to.Address.String(), paymentAddress) {
				continue
			}
			outpoints = append(outpoints, &pb.Contract_Transaction{
				Txid:   tx.ID.String(),
				FromID: to.ID,
				Value:  to.Amount.String(),
			})
		}
	}
	if len(outpoints) > 0 {
		logger.LogInfoWithIDf(log, s.nodeID, "Recovered %d dispute funding outpoint(s) from local order %s", len(outpoints), orderID)
	}
	return outpoints
}

func disputeReleaseOutpointsFromFundingFacts(paymentSent *pb.PaymentSent, coinType iwallet.CoinType, paymentAddress string) []*pb.Contract_Transaction {
	if paymentSent == nil {
		return nil
	}
	paymentAddress = strings.TrimSpace(paymentAddress)
	if paymentAddress == "" {
		paymentAddress = strings.TrimSpace(paymentSent.ToAddress)
	}
	if paymentAddress == "" {
		return nil
	}

	outpoints := make([]*pb.Contract_Transaction, 0, len(paymentSent.GetFundingFacts()))
	seen := make(map[string]struct{}, len(paymentSent.GetFundingFacts()))
	for _, fact := range paymentSent.GetFundingFacts() {
		if fact == nil {
			continue
		}
		txHash := strings.TrimSpace(fact.GetTxHash())
		if txHash == "" || models.NormalizePaymentTxHashSource(fact.GetTxHashSource()) != models.PaymentTxHashSourceChainTx {
			continue
		}
		if !models.FundingFactStatusCountsTowardTotal(fact.GetStatus(), paymentSent.GetConfirmationPolicy()) {
			continue
		}
		if fact.GetEventIndex() < 0 {
			continue
		}
		if toAddress := strings.TrimSpace(fact.GetToAddress()); toAddress != "" && !payment.SameUTXOAddress(toAddress, paymentAddress) {
			continue
		}
		amount := iwallet.NewAmount(strings.TrimSpace(fact.GetAmount()))
		if amount.Cmp(iwallet.NewAmount(0)) <= 0 {
			continue
		}
		seenKey := fmt.Sprintf("%s:%d", txHash, fact.GetEventIndex())
		if _, ok := seen[seenKey]; ok {
			continue
		}
		fromID := models.BuildPaymentDataOutpointID(iwallet.TransactionID(txHash), coinType, uint32(fact.GetEventIndex()))
		if len(fromID) == 0 {
			continue
		}
		outpoints = append(outpoints, &pb.Contract_Transaction{
			Txid:   txHash,
			FromID: fromID,
			Value:  amount.String(),
		})
		seen[seenKey] = struct{}{}
	}
	return outpoints
}
