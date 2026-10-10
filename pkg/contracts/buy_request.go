package contracts

import (
	"context"
	"time"

	"github.com/mobazha/mobazha/pkg/models"
)

// BuyRequestFilter narrows a buy request listing. Empty fields are ignored.
type BuyRequestFilter struct {
	CategorySlug string
	Currency     string
	DeliverTo    string
	Status       string
	Page         int
	PageSize     int
}

// CreateBuyRequestInput is the validated shape of a new buy request. The API
// layer decodes JSON into this; the app service applies the business rules.
type CreateBuyRequestInput struct {
	Title              string
	CategorySlug       string
	Quantity           int
	Unit               string
	BudgetMaxAmount    string
	BudgetMaxCurrency  string
	BudgetMinAmount    string
	AcceptedCurrencies []string
	DeliverTo          string
	Description        string
	Condition          string
	ExpiresAt          time.Time
	Images             []models.ImageHashes
}

// CreateBuyRequestOfferInput is the validated shape of a seller offer.
type CreateBuyRequestOfferInput struct {
	UnitPriceAmount   string
	UnitPriceCurrency string
	Quantity          int
	LeadTimeDays      int
	ShipsFrom         string
	Note              string
}

// CreateBuyRequestCommentInput is the validated shape of a board comment.
// ParentCommentID is empty when the comment starts a new thread.
type CreateBuyRequestCommentInput struct {
	Body            string
	ParentCommentID string
}

// BuyRequestStore abstracts buy request persistence for both standalone and
// SaaS modes. Implementations handle tenant scoping internally via database.Tx.
type BuyRequestStore interface {
	CreateRequest(ctx context.Context, r *models.BuyRequest) error
	GetRequest(ctx context.Context, slug string) (*models.BuyRequest, error)
	ListRequests(ctx context.Context, f BuyRequestFilter) ([]*models.BuyRequest, int64, error)
	ListRequestsByBuyer(ctx context.Context, buyerPeerID string, page, pageSize int) ([]*models.BuyRequest, int64, error)
	UpdateRequest(ctx context.Context, r *models.BuyRequest) error
	CountActiveByBuyer(ctx context.Context, buyerPeerID string) (int64, error)

	CreateOffer(ctx context.Context, o *models.BuyRequestOffer) error
	GetOffer(ctx context.Context, id string) (*models.BuyRequestOffer, error)
	ListOffers(ctx context.Context, requestSlug string) ([]*models.BuyRequestOffer, error)
	UpdateOffer(ctx context.Context, o *models.BuyRequestOffer) error
	HasOfferFromSeller(ctx context.Context, requestSlug, sellerPeerID string) (bool, error)
	RejectPendingOffers(ctx context.Context, requestSlug, exceptOfferID string) error
	CountOffersForRequests(ctx context.Context, slugs []string) (map[string]int64, error)

	CreateComment(ctx context.Context, c *models.BuyRequestComment) error
	GetComment(ctx context.Context, id string) (*models.BuyRequestComment, error)
	ListComments(ctx context.Context, requestSlug string) ([]*models.BuyRequestComment, error)
	CountComments(ctx context.Context, requestSlug string) (int64, error)
}

// BuyRequestService is the business-level interface for the wanted board,
// implemented by BuyRequestAppService in internal/core. The acting party is
// always passed in explicitly so the service stays free of HTTP concerns.
type BuyRequestService interface {
	Create(ctx context.Context, buyerPeerID string, in CreateBuyRequestInput) (*models.BuyRequest, error)
	List(ctx context.Context, f BuyRequestFilter) ([]*models.BuyRequest, int64, error)
	ListMine(ctx context.Context, buyerPeerID string, page, pageSize int) ([]*models.BuyRequest, int64, error)
	Get(ctx context.Context, slug string) (*models.BuyRequest, []*models.BuyRequestOffer, error)
	Cancel(ctx context.Context, buyerPeerID, slug string) (*models.BuyRequest, error)
	SubmitOffer(ctx context.Context, sellerPeerID, slug string, in CreateBuyRequestOfferInput) (*models.BuyRequestOffer, error)
	AcceptOffer(ctx context.Context, buyerPeerID, slug, offerID string) (*models.BuyRequest, *models.BuyRequestOffer, error)
	ListComments(ctx context.Context, slug string) ([]*models.BuyRequestComment, error)
	CreateComment(ctx context.Context, authorPeerID, slug string, in CreateBuyRequestCommentInput) (*models.BuyRequestComment, error)
}

// BuyRequestProvider exposes the per-node wanted board subsystem. Handlers
// obtain this via type assertion on NodeService.
type BuyRequestProvider interface {
	BuyRequests() BuyRequestService
}
