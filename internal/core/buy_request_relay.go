// SPDX-License-Identifier: MPL-2.0
// Copyright (c) 2026 fengzie and the respective contributors.

package core

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/mobazha/mobazha/internal/database"
	"github.com/mobazha/mobazha/internal/logger"
	"github.com/mobazha/mobazha/pkg/contracts"
	pkgdatabase "github.com/mobazha/mobazha/pkg/database"
	"github.com/mobazha/mobazha/pkg/events"
	"github.com/mobazha/mobazha/pkg/models"
	pb "github.com/mobazha/mobazha/pkg/net/mbzpb"
	"google.golang.org/protobuf/types/known/anypb"
)

// buyRequestAskTimeout bounds how long a node waits for the node that owns a
// request to answer. The board is interactive, so waiting longer than this is
// worse than telling the reader the owner did not answer.
const buyRequestAskTimeout = 20 * time.Second

// buyRequestRelay is the wanted board's transport. A request belongs to the
// node the buyer published it on, so every operation on it is either served
// there or relayed there; this type is what carries the operation and brings
// the answer back.
//
// The payloads are the JSON the HTTP API serves, which keeps one wire shape for
// a request no matter which node answers a reader. The identity that acts on a
// relayed operation is always the peer the stream was authenticated as, never
// anything the sender claims about itself.
type buyRequestRelay struct {
	nodeID    string
	peerID    peer.ID
	db        pkgdatabase.Database
	messenger contracts.Messenger
	board     *BuyRequestAppService
	eventBus  events.Bus

	asksMu sync.Mutex
	asks   map[string]chan *pb.BuyRequestMessage
}

type buyRequestRelayConfig struct {
	NodeID    string
	PeerID    peer.ID
	DB        pkgdatabase.Database
	Messenger contracts.Messenger
	Board     *BuyRequestAppService
	EventBus  events.Bus
}

func newBuyRequestRelay(cfg buyRequestRelayConfig) *buyRequestRelay {
	if cfg.DB == nil || cfg.Messenger == nil || cfg.Board == nil {
		return nil
	}
	return &buyRequestRelay{
		nodeID:    cfg.NodeID,
		peerID:    cfg.PeerID,
		db:        cfg.DB,
		messenger: cfg.Messenger,
		board:     cfg.Board,
		eventBus:  cfg.EventBus,
		asks:      make(map[string]chan *pb.BuyRequestMessage),
	}
}

// Compile-time checks: the relay is both the node's outbound transport and its
// inbound handler.
var (
	_ contracts.BuyRequestRelay = (*buyRequestRelay)(nil)
	_ buyRequestNotifier        = (*buyRequestRelay)(nil)
)

// buyRequestNotifier is how the app service reaches a seller on another node.
// It is the service's only way out of this node, which keeps the board's rules
// free of transport concerns.
type buyRequestNotifier interface {
	// NotifyOfferOutcome tells one seller that its offer was accepted or that
	// the deal went to somebody else. payload is the request's detail JSON, so
	// the winning seller can publish the product without asking again.
	NotifyOfferOutcome(ctx context.Context, sellerPeerID, slug string, accepted bool, payload []byte) error
}

// --- outbound ---

// AskRequest returns the owning node's view of a request and its offers.
func (r *buyRequestRelay) AskRequest(ctx context.Context, ownerPeerID, slug string) ([]byte, error) {
	return r.ask(ctx, ownerPeerID, pb.BuyRequestMessage_REQUEST_ASK, slug, nil)
}

// AskComments returns the owning node's comment thread for a request.
func (r *buyRequestRelay) AskComments(ctx context.Context, ownerPeerID, slug string) ([]byte, error) {
	return r.ask(ctx, ownerPeerID, pb.BuyRequestMessage_COMMENTS_ASK, slug, nil)
}

// RelayOffer submits an offer on the owning node.
func (r *buyRequestRelay) RelayOffer(ctx context.Context, ownerPeerID, slug string, body []byte) ([]byte, error) {
	return r.ask(ctx, ownerPeerID, pb.BuyRequestMessage_OFFER, slug, body)
}

// RelayComment posts to the owning node's comment thread.
func (r *buyRequestRelay) RelayComment(ctx context.Context, ownerPeerID, slug string, body []byte) ([]byte, error) {
	return r.ask(ctx, ownerPeerID, pb.BuyRequestMessage_COMMENT, slug, body)
}

// ask performs one operation on the owning node and waits for its answer.
func (r *buyRequestRelay) ask(ctx context.Context, ownerPeerID string, op pb.BuyRequestMessage_Op, slug string, payload []byte) ([]byte, error) {
	if r == nil {
		return nil, errBuyRequestRelayUnavailable
	}
	owner, err := peer.Decode(strings.TrimSpace(ownerPeerID))
	if err != nil {
		return nil, fmt.Errorf("%w: %q is not a peer ID", database.ErrBuyRequestInvalid, ownerPeerID)
	}
	if owner == r.peerID {
		return nil, fmt.Errorf("%w: this node owns the request", database.ErrBuyRequestInvalid)
	}
	if ctx == nil {
		ctx = context.Background()
	}

	requestID := newBuyRequestID()
	answers := r.watch(requestID)
	defer r.forget(requestID)

	if err := r.send(owner, &pb.BuyRequestMessage{
		Op:        op,
		Slug:      slug,
		RequestID: requestID,
		Payload:   payload,
	}); err != nil {
		return nil, err
	}

	timer := time.NewTimer(buyRequestAskTimeout)
	defer timer.Stop()
	select {
	case answer := <-answers:
		if answer.ErrorCode != "" {
			return nil, buyRequestErrorFromCode(answer.ErrorCode, answer.ErrorMessage)
		}
		return answer.Payload, nil
	case <-ctx.Done():
		return nil, fmt.Errorf("%w: %s", database.ErrBuyRequestOwnerUnreachable, owner)
	case <-timer.C:
		return nil, fmt.Errorf("%w: %s did not answer", database.ErrBuyRequestOwnerUnreachable, owner)
	}
}

// NotifyOfferOutcome tells one seller what the buyer decided.
func (r *buyRequestRelay) NotifyOfferOutcome(ctx context.Context, sellerPeerID, slug string, accepted bool, payload []byte) error {
	if r == nil {
		return errBuyRequestRelayUnavailable
	}
	seller, err := peer.Decode(strings.TrimSpace(sellerPeerID))
	if err != nil {
		return fmt.Errorf("%w: %q is not a peer ID", database.ErrBuyRequestInvalid, sellerPeerID)
	}
	// A node is its own counterparty in a hosted setup; there is nobody to tell.
	if seller == r.peerID {
		return nil
	}
	op := pb.BuyRequestMessage_OFFER_REJECTED
	if accepted {
		op = pb.BuyRequestMessage_OFFER_ACCEPTED
	}
	return r.send(seller, &pb.BuyRequestMessage{Op: op, Slug: slug, Payload: payload})
}

// send persists the message and lets the messenger deliver it, so an owner that
// is briefly offline still receives the operation once it is back.
func (r *buyRequestRelay) send(to peer.ID, wire *pb.BuyRequestMessage) error {
	payload, err := anypb.New(wire)
	if err != nil {
		return fmt.Errorf("wrap buy request message: %w", err)
	}
	message := newMessageWithID()
	message.MessageType = pb.Message_BUY_REQUEST
	message.Payload = payload

	return r.db.Update(func(tx pkgdatabase.Tx) error {
		return r.messenger.ReliablySendMessage(tx, to, message, nil)
	})
}

// --- inbound ---

// handleMessage serves the operations this node owns and collects the answers
// to its own. A refused operation is answered with a reason instead of an
// error, so the sender learns what happened rather than retrying forever.
func (r *buyRequestRelay) handleMessage(ctx context.Context, from peer.ID, wire *pb.BuyRequestMessage) error {
	if r == nil || wire == nil {
		return nil
	}
	if ctx == nil {
		ctx = context.Background()
	}
	switch wire.Op {
	case pb.BuyRequestMessage_REQUEST_ASK:
		return r.answer(ctx, from, wire, r.snapshotPayload)
	case pb.BuyRequestMessage_COMMENTS_ASK:
		return r.answer(ctx, from, wire, r.commentsPayload)
	case pb.BuyRequestMessage_OFFER:
		return r.answer(ctx, from, wire, r.applyOffer)
	case pb.BuyRequestMessage_COMMENT:
		return r.answer(ctx, from, wire, r.applyComment)
	case pb.BuyRequestMessage_REQUEST_SEND,
		pb.BuyRequestMessage_COMMENTS_SEND,
		pb.BuyRequestMessage_OFFER_RESULT,
		pb.BuyRequestMessage_COMMENT_RESULT:
		r.deliver(wire)
		return nil
	case pb.BuyRequestMessage_OFFER_ACCEPTED, pb.BuyRequestMessage_OFFER_REJECTED:
		return r.recordOutcome(from, wire)
	default:
		return fmt.Errorf("unknown buy request op %s", wire.Op)
	}
}

// answer runs one owner-side operation and replies with either its payload or
// the reason it was refused. Ops are laid out so that an answer is always the
// next value of the op that asked for it.
func (r *buyRequestRelay) answer(
	ctx context.Context,
	from peer.ID,
	ask *pb.BuyRequestMessage,
	handle func(context.Context, peer.ID, *pb.BuyRequestMessage) ([]byte, error),
) error {
	payload, err := handle(ctx, from, ask)
	reply := &pb.BuyRequestMessage{Op: ask.Op + 1, Slug: ask.Slug, RequestID: ask.RequestID}
	if err != nil {
		reply.ErrorCode = buyRequestErrorCode(err)
		reply.ErrorMessage = err.Error()
		logger.LogInfoWithIDf(log, r.nodeID, "BuyRequest: refused %s on %s for %s: %v",
			ask.Op, ask.Slug, from, err)
	} else {
		reply.Payload = payload
	}
	return r.send(from, reply)
}

// snapshotPayload is the owner's answer to a request read. Reading sweeps
// expired rows first, so a reader never sees a stale "open" request.
func (r *buyRequestRelay) snapshotPayload(ctx context.Context, _ peer.ID, ask *pb.BuyRequestMessage) ([]byte, error) {
	request, offers, err := r.board.Get(ctx, ask.Slug)
	if err != nil {
		return nil, err
	}
	return json.Marshal(contracts.NewBuyRequestDetailDTO(request, offers))
}

// commentsPayload is the owner's answer to a comment-thread read.
func (r *buyRequestRelay) commentsPayload(ctx context.Context, _ peer.ID, ask *pb.BuyRequestMessage) ([]byte, error) {
	comments, err := r.board.ListComments(ctx, ask.Slug)
	if err != nil {
		return nil, err
	}
	return json.Marshal(contracts.NewBuyRequestCommentThreadDTO(comments))
}

// applyOffer stores a seller's offer. The seller is the authenticated peer, so
// a node cannot offer as somebody else.
func (r *buyRequestRelay) applyOffer(ctx context.Context, from peer.ID, ask *pb.BuyRequestMessage) ([]byte, error) {
	var body contracts.CreateBuyRequestOfferRequest
	if err := json.Unmarshal(ask.Payload, &body); err != nil {
		return nil, fmt.Errorf("%w: malformed offer body", database.ErrBuyRequestInvalid)
	}
	seller := from.String()
	// The owner applies the same rules it applies to a local offer: an offer is
	// binding, so a seller gets one shot per request. A redelivered message
	// never reaches this point twice (the handler drops duplicates), so a
	// second offer really is a second offer and is refused as one.
	offer, err := r.board.SubmitOffer(ctx, seller, ask.Slug, body.ToInput())
	if err != nil {
		return nil, err
	}
	r.announceOfferReceived(ask.Slug, offer)
	return json.Marshal(contracts.NewBuyRequestOfferDTO(offer))
}

// applyComment posts to a request's thread, attributed to the authenticated
// peer rather than to anything the body claims.
func (r *buyRequestRelay) applyComment(ctx context.Context, from peer.ID, ask *pb.BuyRequestMessage) ([]byte, error) {
	var body contracts.CreateBuyRequestCommentRequest
	if err := json.Unmarshal(ask.Payload, &body); err != nil {
		return nil, fmt.Errorf("%w: malformed comment body", database.ErrBuyRequestInvalid)
	}
	comment, err := r.board.CreateComment(ctx, from.String(), ask.Slug, body.ToInput())
	if err != nil {
		return nil, err
	}
	return json.Marshal(contracts.NewBuyRequestCommentDTO(comment))
}

// announceOfferReceived tells the buyer, on the node that owns the request,
// that a seller just answered.
func (r *buyRequestRelay) announceOfferReceived(slug string, offer *models.BuyRequestOffer) {
	if r.eventBus == nil || offer == nil {
		return
	}
	r.eventBus.Emit(&events.BuyRequestOfferReceived{
		Slug:         slug,
		OfferID:      offer.ID,
		SellerPeerID: offer.SellerPeerID,
		UnitPrice:    offer.UnitPriceAmount,
		Currency:     offer.UnitPriceCurrency,
		Quantity:     offer.Quantity,
		LeadTimeDays: offer.LeadTimeDays,
		Note:         offer.Note,
	})
}

// recordOutcome mirrors a buyer's decision onto the seller's node, so the
// seller learns the result without keeping a copy of somebody else's board.
func (r *buyRequestRelay) recordOutcome(from peer.ID, wire *pb.BuyRequestMessage) error {
	var detail contracts.BuyRequestDetailDTO
	if err := json.Unmarshal(wire.Payload, &detail); err != nil {
		return fmt.Errorf("decode buy request outcome: %w", err)
	}
	decision := &events.BuyRequestOfferDecided{
		Slug:        wire.Slug,
		OwnerPeerID: from.String(),
		Accepted:    wire.Op == pb.BuyRequestMessage_OFFER_ACCEPTED,
	}
	if detail.Request != nil {
		decision.Title = detail.Request.Title
	}
	for _, offer := range detail.Offers {
		if offer == nil || offer.SellerID.PeerID != r.peerID.String() {
			continue
		}
		decision.OfferID = offer.ID
		decision.UnitPrice = offer.UnitPrice.Amount
		decision.Currency = offer.UnitPrice.Currency
		if offer.Status == string(models.BuyRequestOfferAccepted) {
			decision.Accepted = true
		}
	}
	logger.LogInfoWithIDf(log, r.nodeID, "BuyRequest: offer %s on %s was %s by %s",
		decision.OfferID, wire.Slug, buyRequestOutcomeWord(decision.Accepted), from)
	if r.eventBus != nil {
		r.eventBus.Emit(decision)
	}
	return nil
}

// deliver hands an answer to the caller waiting for it. An answer nobody is
// waiting for — the asker timed out, or this node restarted — is dropped: the
// message is only an answer, never a source of truth.
func (r *buyRequestRelay) deliver(wire *pb.BuyRequestMessage) {
	r.asksMu.Lock()
	answers := r.asks[wire.RequestID]
	r.asksMu.Unlock()
	if answers == nil {
		logger.LogDebugWithIDf(log, r.nodeID, "BuyRequest: dropping unsolicited %s for %s", wire.Op, wire.Slug)
		return
	}
	select {
	case answers <- wire:
	default:
	}
}

func (r *buyRequestRelay) watch(requestID string) chan *pb.BuyRequestMessage {
	answers := make(chan *pb.BuyRequestMessage, 1)
	r.asksMu.Lock()
	r.asks[requestID] = answers
	r.asksMu.Unlock()
	return answers
}

func (r *buyRequestRelay) forget(requestID string) {
	r.asksMu.Lock()
	delete(r.asks, requestID)
	r.asksMu.Unlock()
}

var errBuyRequestRelayUnavailable = errors.New("buy request relay is not available on this node")

func buyRequestOutcomeWord(accepted bool) string {
	if accepted {
		return "accepted"
	}
	return "refused"
}

func newBuyRequestID() string {
	buf := make([]byte, 20)
	if _, err := rand.Read(buf); err != nil {
		return fmt.Sprintf("fallback-%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(buf)
}

// buyRequestErrorCode names the reason an owner refused an operation. Both
// nodes derive the name from the same sentinel errors, so a refusal keeps its
// meaning without sharing error values.
func buyRequestErrorCode(err error) string {
	switch {
	case errors.Is(err, database.ErrBuyRequestNotFound),
		errors.Is(err, database.ErrBuyRequestOfferNotFound),
		errors.Is(err, database.ErrBuyRequestCommentNotFound):
		return contracts.BuyRequestCodeNotFound
	case errors.Is(err, database.ErrBuyRequestInvalid):
		return contracts.BuyRequestCodeInvalid
	case errors.Is(err, database.ErrBuyRequestClosed):
		return contracts.BuyRequestCodeClosed
	case errors.Is(err, database.ErrBuyRequestExpired):
		return contracts.BuyRequestCodeExpired
	case errors.Is(err, database.ErrBuyRequestSelfOffer):
		return contracts.BuyRequestCodeSelfOffer
	case errors.Is(err, database.ErrBuyRequestDuplicateOffer):
		return contracts.BuyRequestCodeDuplicateOffer
	case errors.Is(err, database.ErrBuyRequestCurrencyRejected):
		return contracts.BuyRequestCodeCurrencyRejected
	case errors.Is(err, database.ErrBuyRequestNotOwner):
		return contracts.BuyRequestCodeNotOwner
	case errors.Is(err, database.ErrBuyRequestLimitReached):
		return contracts.BuyRequestCodeLimitReached
	case errors.Is(err, database.ErrBuyRequestSlugTaken):
		return contracts.BuyRequestCodeSlugTaken
	case errors.Is(err, database.ErrBuyRequestReplyToReply):
		return contracts.BuyRequestCodeReplyToReply
	case errors.Is(err, database.ErrBuyRequestOwnerUnreachable):
		return contracts.BuyRequestCodeUnreachable
	default:
		return contracts.BuyRequestCodeInternal
	}
}

// buyRequestErrorFromCode turns a refusal back into the sentinel the HTTP layer
// already knows how to report, keeping the owner's own explanation as detail.
func buyRequestErrorFromCode(code, message string) error {
	var sentinel error
	switch code {
	case contracts.BuyRequestCodeNotFound:
		sentinel = database.ErrBuyRequestNotFound
	case contracts.BuyRequestCodeInvalid:
		sentinel = database.ErrBuyRequestInvalid
	case contracts.BuyRequestCodeClosed:
		sentinel = database.ErrBuyRequestClosed
	case contracts.BuyRequestCodeExpired:
		sentinel = database.ErrBuyRequestExpired
	case contracts.BuyRequestCodeSelfOffer:
		sentinel = database.ErrBuyRequestSelfOffer
	case contracts.BuyRequestCodeDuplicateOffer:
		sentinel = database.ErrBuyRequestDuplicateOffer
	case contracts.BuyRequestCodeCurrencyRejected:
		sentinel = database.ErrBuyRequestCurrencyRejected
	case contracts.BuyRequestCodeNotOwner:
		sentinel = database.ErrBuyRequestNotOwner
	case contracts.BuyRequestCodeLimitReached:
		sentinel = database.ErrBuyRequestLimitReached
	case contracts.BuyRequestCodeSlugTaken:
		sentinel = database.ErrBuyRequestSlugTaken
	case contracts.BuyRequestCodeReplyToReply:
		sentinel = database.ErrBuyRequestReplyToReply
	case contracts.BuyRequestCodeUnreachable:
		sentinel = database.ErrBuyRequestOwnerUnreachable
	case contracts.BuyRequestCodeInternal:
		sentinel = database.ErrBuyRequestInternal
	}
	message = strings.TrimSpace(message)
	switch {
	case sentinel == nil && message == "":
		return fmt.Errorf("%w: the node that owns this request refused it",
			database.ErrBuyRequestInternal)
	case sentinel == nil:
		return fmt.Errorf("%w: %s", database.ErrBuyRequestInternal, message)
	case message == "":
		return sentinel
	default:
		return fmt.Errorf("%w: %s", sentinel, message)
	}
}
