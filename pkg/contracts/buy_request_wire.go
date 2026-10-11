package contracts

import (
	"context"
	"strings"
	"time"

	"github.com/mobazha/mobazha/pkg/models"
)

// The wanted board has exactly one wire shape: the JSON the HTTP API serves.
// A node that does not own a request relays the operation to the node that
// does (see BuyRequestRelay) and hands the answer straight back, so these
// types are shared by the HTTP handlers and by the relay instead of being
// duplicated with a second, p2p-only shape that could drift.

// BuyRequestPartyDTO identifies one side of a request. Only the peer ID is
// known to the board; the display name and handle are resolved by the client.
type BuyRequestPartyDTO struct {
	PeerID string `json:"peerID"`
	Name   string `json:"name"`
	Handle string `json:"handle"`
}

// BuyRequestAmountDTO is an amount together with the currency it is counted in.
type BuyRequestAmountDTO struct {
	Amount   string `json:"amount"`
	Currency string `json:"currency"`
}

// BuyRequestBudgetDTO carries the buyer's ceiling and, when they stated one,
// the floor they would rather pay.
type BuyRequestBudgetDTO struct {
	Max BuyRequestAmountDTO  `json:"max"`
	Min *BuyRequestAmountDTO `json:"min,omitempty"`
}

// BuyRequestDTO is one wanted-board request as the API and the relay both
// describe it.
type BuyRequestDTO struct {
	Slug               string              `json:"slug"`
	BuyerID            BuyRequestPartyDTO  `json:"buyerID"`
	Title              string              `json:"title"`
	CategorySlug       string              `json:"categorySlug"`
	Description        string              `json:"description,omitempty"`
	Quantity           int                 `json:"quantity"`
	Unit               string              `json:"unit,omitempty"`
	Budget             BuyRequestBudgetDTO `json:"budget"`
	AcceptedCurrencies []string            `json:"acceptedCurrencies"`
	DeliverTo          string              `json:"deliverTo"`
	Condition          string              `json:"condition"`
	Status             string              `json:"status"`
	AcceptedOfferID    string              `json:"acceptedOfferID,omitempty"`
	OfferCount         int                 `json:"offerCount"`
	ExpiresAt          string              `json:"expiresAt"`
	CreatedAt          string              `json:"createdAt"`
	// Images reuses the media service's hash set, so a client can hand the
	// upload response straight back without reshaping it.
	Images []models.ImageHashes `json:"images,omitempty"`
}

// BuyRequestOfferDTO is one seller's binding offer.
type BuyRequestOfferDTO struct {
	ID           string              `json:"id"`
	RequestSlug  string              `json:"requestSlug"`
	SellerID     BuyRequestPartyDTO  `json:"sellerID"`
	UnitPrice    BuyRequestAmountDTO `json:"unitPrice"`
	Quantity     int                 `json:"quantity"`
	LeadTimeDays int                 `json:"leadTimeDays"`
	ShipsFrom    string              `json:"shipsFrom"`
	// Escrow is not optional on Mobazha; the field is constant so a client can
	// never accidentally render an offer as "pay me directly".
	AcceptsEscrow bool   `json:"acceptsEscrow"`
	Note          string `json:"note,omitempty"`
	Status        string `json:"status"`
	CreatedAt     string `json:"createdAt"`
}

// BuyRequestDetailDTO is a request together with every offer made on it.
type BuyRequestDetailDTO struct {
	Request *BuyRequestDTO        `json:"request"`
	Offers  []*BuyRequestOfferDTO `json:"offers"`
}

// BuyRequestListDTO carries a board page and its pagination. It carries the
// pagination itself instead of relying on the response envelope's meta,
// because the huma bridge re-wraps the payload and would drop meta.
type BuyRequestListDTO struct {
	Requests []*BuyRequestDTO `json:"requests"`
	Total    int64            `json:"total"`
	Page     int              `json:"page"`
	PageSize int              `json:"pageSize"`
}

// BuyRequestOrderDraftDTO is the escrow-ready summary of an accepted offer.
// It is a snapshot for the checkout flow, not an order.
type BuyRequestOrderDraftDTO struct {
	Slug            string              `json:"slug"`
	VendorID        BuyRequestPartyDTO  `json:"vendorID"`
	Title           string              `json:"title"`
	Quantity        int                 `json:"quantity"`
	UnitPrice       BuyRequestAmountDTO `json:"unitPrice"`
	Subtotal        BuyRequestAmountDTO `json:"subtotal"`
	Currency        string              `json:"currency"`
	ShipsFrom       string              `json:"shipsFrom"`
	DeliverTo       string              `json:"deliverTo"`
	AcceptedOfferID string              `json:"acceptedOfferID"`
}

// AcceptBuyRequestOfferDTO is the answer to accepting an offer.
type AcceptBuyRequestOfferDTO struct {
	Request    *BuyRequestDTO          `json:"request"`
	OrderDraft BuyRequestOrderDraftDTO `json:"orderDraft"`
}

// BuyRequestCommentDTO is one message in a request's thread. Replies are
// nested so a client renders a thread without rebuilding the tree itself.
type BuyRequestCommentDTO struct {
	ID          string             `json:"id"`
	RequestSlug string             `json:"requestSlug"`
	AuthorID    BuyRequestPartyDTO `json:"authorID"`
	// Empty for a top-level comment.
	ParentCommentID string                  `json:"parentCommentID,omitempty"`
	Body            string                  `json:"body"`
	CreatedAt       string                  `json:"createdAt"`
	Replies         []*BuyRequestCommentDTO `json:"replies,omitempty"`
}

// BuyRequestCommentThreadDTO is a request's whole thread.
type BuyRequestCommentThreadDTO struct {
	Comments []*BuyRequestCommentDTO `json:"comments"`
	Total    int                     `json:"total"`
}

// CreateBuyRequestOfferRequest is the body a seller posts to offer on a
// request. It accepts both the nested `unitPrice` form and the flat
// `unitPriceAmount` / `unitPriceCurrency` form, because the publish forms and
// the API docs use different ones.
type CreateBuyRequestOfferRequest struct {
	UnitPrice         *BuyRequestAmountDTO `json:"unitPrice"`
	UnitPriceAmount   string               `json:"unitPriceAmount"`
	UnitPriceCurrency string               `json:"unitPriceCurrency"`
	Quantity          int                  `json:"quantity"`
	LeadTimeDays      int                  `json:"leadTimeDays"`
	ShipsFrom         string               `json:"shipsFrom"`
	Note              string               `json:"note"`
}

// ToInput normalises the accepted body forms into the app service input.
func (b CreateBuyRequestOfferRequest) ToInput() CreateBuyRequestOfferInput {
	in := CreateBuyRequestOfferInput{
		UnitPriceAmount:   b.UnitPriceAmount,
		UnitPriceCurrency: b.UnitPriceCurrency,
		Quantity:          b.Quantity,
		LeadTimeDays:      b.LeadTimeDays,
		ShipsFrom:         b.ShipsFrom,
		Note:              b.Note,
	}
	if b.UnitPrice != nil {
		in.UnitPriceAmount = b.UnitPrice.Amount
		in.UnitPriceCurrency = b.UnitPrice.Currency
	}
	return in
}

// CreateBuyRequestCommentRequest is the body posted to a request's thread.
type CreateBuyRequestCommentRequest struct {
	Body            string `json:"body"`
	ParentCommentID string `json:"parentCommentID"`
}

// ToInput normalises the body into the app service input.
func (b CreateBuyRequestCommentRequest) ToInput() CreateBuyRequestCommentInput {
	return CreateBuyRequestCommentInput{
		Body:            b.Body,
		ParentCommentID: b.ParentCommentID,
	}
}

// NewBuyRequestPartyDTO stamps the peer ID the board knows. A node only ever
// learns a counterparty from the identity it was authenticated as, so there is
// nothing else to fill in here.
func NewBuyRequestPartyDTO(peerID string) BuyRequestPartyDTO {
	return BuyRequestPartyDTO{PeerID: peerID}
}

// NewBuyRequestDTO maps a stored request onto the wire shape.
func NewBuyRequestDTO(m *models.BuyRequest) *BuyRequestDTO {
	if m == nil {
		return nil
	}
	out := &BuyRequestDTO{
		Slug:               m.Slug,
		BuyerID:            NewBuyRequestPartyDTO(m.BuyerPeerID),
		Title:              m.Title,
		CategorySlug:       m.CategorySlug,
		Description:        m.Description,
		Quantity:           m.Quantity,
		Unit:               m.Unit,
		Budget:             BuyRequestBudgetDTO{Max: BuyRequestAmountDTO{Amount: m.BudgetMaxAmount, Currency: m.BudgetMaxCurrency}},
		AcceptedCurrencies: SplitBuyRequestCurrencies(m.AcceptedCurrencies),
		DeliverTo:          m.DeliverTo,
		Condition:          string(m.Condition),
		Status:             string(m.Status),
		AcceptedOfferID:    m.AcceptedOfferID,
		OfferCount:         m.OfferCount,
		ExpiresAt:          m.ExpiresAt.UTC().Format(time.RFC3339),
		CreatedAt:          m.CreatedAt.UTC().Format(time.RFC3339),
		Images:             m.Images,
	}
	if strings.TrimSpace(m.BudgetMinAmount) != "" {
		min := BuyRequestAmountDTO{Amount: m.BudgetMinAmount, Currency: m.BudgetMaxCurrency}
		out.Budget.Min = &min
	}
	return out
}

// NewBuyRequestOfferDTO maps a stored offer onto the wire shape.
func NewBuyRequestOfferDTO(m *models.BuyRequestOffer) *BuyRequestOfferDTO {
	if m == nil {
		return nil
	}
	return &BuyRequestOfferDTO{
		ID:            m.ID,
		RequestSlug:   m.RequestSlug,
		SellerID:      NewBuyRequestPartyDTO(m.SellerPeerID),
		UnitPrice:     BuyRequestAmountDTO{Amount: m.UnitPriceAmount, Currency: m.UnitPriceCurrency},
		Quantity:      m.Quantity,
		LeadTimeDays:  m.LeadTimeDays,
		ShipsFrom:     m.ShipsFrom,
		AcceptsEscrow: true,
		Note:          m.Note,
		Status:        string(m.Status),
		CreatedAt:     m.CreatedAt.UTC().Format(time.RFC3339),
	}
}

// NewBuyRequestDetailDTO maps a request and its offers onto the wire shape.
func NewBuyRequestDetailDTO(request *models.BuyRequest, offers []*models.BuyRequestOffer) BuyRequestDetailDTO {
	out := BuyRequestDetailDTO{
		Request: NewBuyRequestDTO(request),
		Offers:  make([]*BuyRequestOfferDTO, 0, len(offers)),
	}
	for _, offer := range offers {
		out.Offers = append(out.Offers, NewBuyRequestOfferDTO(offer))
	}
	return out
}

// NewBuyRequestCommentDTO maps a stored comment onto the wire shape.
func NewBuyRequestCommentDTO(m *models.BuyRequestComment) *BuyRequestCommentDTO {
	if m == nil {
		return nil
	}
	return &BuyRequestCommentDTO{
		ID:              m.ID,
		RequestSlug:     m.RequestSlug,
		AuthorID:        NewBuyRequestPartyDTO(m.AuthorPeerID),
		ParentCommentID: m.ParentCommentID,
		Body:            m.Body,
		CreatedAt:       m.CreatedAt.UTC().Format(time.RFC3339),
	}
}

// NewBuyRequestCommentThreadDTO groups a flat, oldest-first comment list into
// one level of threads. A reply whose parent is missing is surfaced as a
// top-level comment rather than dropped — silently losing a message is worse
// than a slightly odd indentation.
func NewBuyRequestCommentThreadDTO(flat []*models.BuyRequestComment) BuyRequestCommentThreadDTO {
	roots := make([]*BuyRequestCommentDTO, 0, len(flat))
	byID := make(map[string]*BuyRequestCommentDTO, len(flat))

	for _, m := range flat {
		dto := NewBuyRequestCommentDTO(m)
		byID[dto.ID] = dto
		if dto.ParentCommentID == "" {
			roots = append(roots, dto)
		}
	}
	for _, m := range flat {
		dto := byID[m.ID]
		if dto.ParentCommentID == "" {
			continue
		}
		parent, ok := byID[dto.ParentCommentID]
		if !ok {
			roots = append(roots, dto)
			continue
		}
		parent.Replies = append(parent.Replies, dto)
	}
	return BuyRequestCommentThreadDTO{Comments: roots, Total: len(flat)}
}

// SplitBuyRequestCurrencies turns the stored ",USD,USDT," form back into a list.
func SplitBuyRequestCurrencies(stored string) []string {
	out := make([]string, 0, 4)
	for _, part := range strings.Split(stored, ",") {
		if part = strings.TrimSpace(part); part != "" {
			out = append(out, part)
		}
	}
	return out
}

// The reasons a wanted-board operation can be refused. They mirror the
// sentinel errors in internal/database so two nodes can agree on why an
// operation failed without sharing error values.
const (
	BuyRequestCodeNotFound         = "not_found"
	BuyRequestCodeInvalid          = "invalid"
	BuyRequestCodeClosed           = "closed"
	BuyRequestCodeExpired          = "expired"
	BuyRequestCodeSelfOffer        = "self_offer"
	BuyRequestCodeDuplicateOffer   = "duplicate_offer"
	BuyRequestCodeCurrencyRejected = "currency_rejected"
	BuyRequestCodeNotOwner         = "not_owner"
	BuyRequestCodeLimitReached     = "limit_reached"
	BuyRequestCodeSlugTaken        = "slug_taken"
	BuyRequestCodeReplyToReply     = "reply_to_reply"
	// BuyRequestCodeUnreachable means the node that owns the request did not
	// answer: it may be offline, or simply not know the slug.
	BuyRequestCodeUnreachable = "owner_unreachable"
	BuyRequestCodeInternal    = "internal"
)

// BuyRequestRelay moves wanted-board operations to the node that owns the
// request. The payloads it returns are the same JSON the HTTP API serves, so
// a caller can hand them back verbatim.
//
// Only the owning node writes to a request: a relayed operation is validated
// and stored there, and the relay never keeps a second copy of the board.
type BuyRequestRelay interface {
	// AskRequest returns the owning node's view of a request and its offers.
	AskRequest(ctx context.Context, ownerPeerID, slug string) ([]byte, error)
	// AskComments returns the owning node's comment thread for a request.
	AskComments(ctx context.Context, ownerPeerID, slug string) ([]byte, error)
	// RelayOffer submits an offer on the owning node. body is the same JSON the
	// HTTP API accepts for POST /buy-requests/{slug}/offers.
	RelayOffer(ctx context.Context, ownerPeerID, slug string, body []byte) ([]byte, error)
	// RelayComment posts to the owning node's comment thread. body is the same
	// JSON the HTTP API accepts for POST /buy-requests/{slug}/comments.
	RelayComment(ctx context.Context, ownerPeerID, slug string, body []byte) ([]byte, error)
}

// BuyRequestRelayProvider exposes the wanted board's cross-node transport.
// Handlers reach it the same way they reach BuyRequests().
type BuyRequestRelayProvider interface {
	BuyRequestRelay() BuyRequestRelay
}
