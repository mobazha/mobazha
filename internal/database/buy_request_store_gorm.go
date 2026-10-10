package database

import (
	"context"
	"errors"
	"strings"
	"time"

	pkgcontracts "github.com/mobazha/mobazha/pkg/contracts"
	pkgdb "github.com/mobazha/mobazha/pkg/database"
	"github.com/mobazha/mobazha/pkg/models"
	"gorm.io/gorm"
)

// Business-rule failures that the API layer maps onto HTTP statuses. They live
// here rather than in internal/core so handlers can reference them without
// importing the core package, matching how collection errors are declared.
var (
	ErrBuyRequestNotFound         = errors.New("buy request not found")
	ErrBuyRequestOfferNotFound    = errors.New("buy request offer not found")
	ErrBuyRequestClosed           = errors.New("buy request is no longer accepting offers")
	ErrBuyRequestExpired          = errors.New("buy request has expired")
	ErrBuyRequestSelfOffer        = errors.New("buyer cannot offer on their own request")
	ErrBuyRequestDuplicateOffer   = errors.New("a seller may only offer once per request")
	ErrBuyRequestCurrencyRejected = errors.New("currency is not accepted by this request")
	ErrBuyRequestInvalid          = errors.New("invalid buy request")
	ErrBuyRequestLimitReached     = errors.New("too many open buy requests")
	ErrBuyRequestNotOwner         = errors.New("only the buyer can perform this action")
	ErrBuyRequestSlugTaken        = errors.New("buy request slug already exists")
	ErrBuyRequestCommentNotFound  = errors.New("buy request comment not found")
	ErrBuyRequestReplyToReply     = errors.New("a reply may not itself be replied to")
)

type GormBuyRequestStore struct {
	db pkgdb.Database
}

var _ pkgcontracts.BuyRequestStore = (*GormBuyRequestStore)(nil)

func NewGormBuyRequestStore(db pkgdb.Database) *GormBuyRequestStore {
	return &GormBuyRequestStore{db: db}
}

// MigrateBuyRequestModels creates/updates the wanted board tables.
func MigrateBuyRequestModels(db pkgdb.Database) error {
	return db.Update(func(tx pkgdb.Tx) error {
		if err := tx.Migrate(&models.BuyRequest{}); err != nil {
			return err
		}
		if err := tx.Migrate(&models.BuyRequestOffer{}); err != nil {
			return err
		}
		return tx.Migrate(&models.BuyRequestComment{})
	})
}

// NormalizeAcceptedCurrencies renders a currency list as ",USD,USDT," so the
// currency filter can use an exact substring match.
func NormalizeAcceptedCurrencies(currencies []string) string {
	var b strings.Builder
	b.WriteByte(',')
	for _, c := range currencies {
		c = strings.ToUpper(strings.TrimSpace(c))
		if c == "" {
			continue
		}
		b.WriteString(c)
		b.WriteByte(',')
	}
	return b.String()
}

func applyBuyRequestFilters(q *gorm.DB, f pkgcontracts.BuyRequestFilter) *gorm.DB {
	q = q.Where("deleted_at IS NULL")
	if f.Status != "" {
		q = q.Where("status = ?", f.Status)
	}
	if f.CategorySlug != "" {
		q = q.Where("category_slug = ?", f.CategorySlug)
	}
	if f.DeliverTo != "" {
		q = q.Where("deliver_to = ?", strings.ToUpper(f.DeliverTo))
	}
	if f.Currency != "" {
		q = q.Where("accepted_currencies LIKE ?", "%,"+strings.ToUpper(f.Currency)+",%")
	}
	return q
}

func (s *GormBuyRequestStore) CreateRequest(_ context.Context, r *models.BuyRequest) error {
	now := time.Now()
	r.CreatedAt = now
	r.UpdatedAt = now
	return s.db.Update(func(tx pkgdb.Tx) error {
		var existing models.BuyRequest
		err := tx.Read().Where("slug = ?", r.Slug).First(&existing).Error
		if err == nil {
			return ErrBuyRequestSlugTaken
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		return tx.Save(r)
	})
}

func (s *GormBuyRequestStore) GetRequest(_ context.Context, slug string) (*models.BuyRequest, error) {
	var r models.BuyRequest
	err := s.db.View(func(tx pkgdb.Tx) error {
		return tx.Read().Where("slug = ? AND deleted_at IS NULL", slug).First(&r).Error
	})
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrBuyRequestNotFound
		}
		return nil, err
	}
	return &r, nil
}

func (s *GormBuyRequestStore) ListRequests(_ context.Context, f pkgcontracts.BuyRequestFilter) ([]*models.BuyRequest, int64, error) {
	page, size := normalizePage(f.Page, f.PageSize)
	var out []*models.BuyRequest
	var total int64

	err := s.db.View(func(tx pkgdb.Tx) error {
		if err := applyBuyRequestFilters(tx.Read().Model(&models.BuyRequest{}), f).Count(&total).Error; err != nil {
			return err
		}
		return applyBuyRequestFilters(tx.Read().Model(&models.BuyRequest{}), f).
			Order("created_at DESC").
			Offset((page - 1) * size).Limit(size).
			Find(&out).Error
	})
	if err != nil {
		return nil, 0, err
	}
	if err := s.attachOfferCounts(out); err != nil {
		return nil, 0, err
	}
	return out, total, nil
}

func (s *GormBuyRequestStore) ListRequestsByBuyer(_ context.Context, buyerPeerID string, page, pageSize int) ([]*models.BuyRequest, int64, error) {
	page, size := normalizePage(page, pageSize)
	var out []*models.BuyRequest
	var total int64

	err := s.db.View(func(tx pkgdb.Tx) error {
		base := func() *gorm.DB {
			return tx.Read().Model(&models.BuyRequest{}).
				Where("deleted_at IS NULL AND buyer_peer_id = ?", buyerPeerID)
		}
		if err := base().Count(&total).Error; err != nil {
			return err
		}
		return base().Order("created_at DESC").
			Offset((page - 1) * size).Limit(size).
			Find(&out).Error
	})
	if err != nil {
		return nil, 0, err
	}
	if err := s.attachOfferCounts(out); err != nil {
		return nil, 0, err
	}
	return out, total, nil
}

func (s *GormBuyRequestStore) UpdateRequest(_ context.Context, r *models.BuyRequest) error {
	r.UpdatedAt = time.Now()
	return s.db.Update(func(tx pkgdb.Tx) error {
		var existing models.BuyRequest
		if err := tx.Read().Where("slug = ? AND deleted_at IS NULL", r.Slug).First(&existing).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrBuyRequestNotFound
			}
			return err
		}
		existing.Status = r.Status
		existing.AcceptedOfferID = r.AcceptedOfferID
		existing.ExpiresAt = r.ExpiresAt
		existing.UpdatedAt = r.UpdatedAt
		return tx.Save(&existing)
	})
}

func (s *GormBuyRequestStore) CountActiveByBuyer(_ context.Context, buyerPeerID string) (int64, error) {
	var total int64
	err := s.db.View(func(tx pkgdb.Tx) error {
		return tx.Read().Model(&models.BuyRequest{}).
			Where("deleted_at IS NULL AND buyer_peer_id = ? AND status = ?", buyerPeerID, models.BuyRequestStatusOpen).
			Count(&total).Error
	})
	return total, err
}

func (s *GormBuyRequestStore) CreateOffer(_ context.Context, o *models.BuyRequestOffer) error {
	now := time.Now()
	o.CreatedAt = now
	o.UpdatedAt = now
	return s.db.Update(func(tx pkgdb.Tx) error {
		return tx.Save(o)
	})
}

func (s *GormBuyRequestStore) GetOffer(_ context.Context, id string) (*models.BuyRequestOffer, error) {
	var o models.BuyRequestOffer
	err := s.db.View(func(tx pkgdb.Tx) error {
		return tx.Read().Where("id = ? AND deleted_at IS NULL", id).First(&o).Error
	})
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrBuyRequestOfferNotFound
		}
		return nil, err
	}
	return &o, nil
}

func (s *GormBuyRequestStore) ListOffers(_ context.Context, requestSlug string) ([]*models.BuyRequestOffer, error) {
	var out []*models.BuyRequestOffer
	err := s.db.View(func(tx pkgdb.Tx) error {
		return tx.Read().Where("request_slug = ? AND deleted_at IS NULL", requestSlug).
			Order("created_at DESC").Find(&out).Error
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (s *GormBuyRequestStore) UpdateOffer(_ context.Context, o *models.BuyRequestOffer) error {
	o.UpdatedAt = time.Now()
	return s.db.Update(func(tx pkgdb.Tx) error {
		var existing models.BuyRequestOffer
		if err := tx.Read().Where("id = ? AND deleted_at IS NULL", o.ID).First(&existing).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrBuyRequestOfferNotFound
			}
			return err
		}
		existing.Status = o.Status
		existing.UpdatedAt = o.UpdatedAt
		return tx.Save(&existing)
	})
}

func (s *GormBuyRequestStore) HasOfferFromSeller(_ context.Context, requestSlug, sellerPeerID string) (bool, error) {
	var total int64
	err := s.db.View(func(tx pkgdb.Tx) error {
		return tx.Read().Model(&models.BuyRequestOffer{}).
			Where("request_slug = ? AND seller_peer_id = ? AND deleted_at IS NULL", requestSlug, sellerPeerID).
			Count(&total).Error
	})
	return total > 0, err
}

// RejectPendingOffers closes every still-open offer on a request except the one
// the buyer just accepted, so the board never shows a live offer on a locked
// request.
func (s *GormBuyRequestStore) RejectPendingOffers(_ context.Context, requestSlug, exceptOfferID string) error {
	now := time.Now()
	return s.db.Update(func(tx pkgdb.Tx) error {
		q := tx.Read().Model(&models.BuyRequestOffer{}).
			Where("request_slug = ? AND status = ? AND deleted_at IS NULL", requestSlug, models.BuyRequestOfferSubmitted)
		if exceptOfferID != "" {
			q = q.Where("id <> ?", exceptOfferID)
		}
		return q.Updates(map[string]interface{}{
			"status":     string(models.BuyRequestOfferRejected),
			"updated_at": now,
		}).Error
	})
}

func (s *GormBuyRequestStore) CountOffersForRequests(_ context.Context, slugs []string) (map[string]int64, error) {
	counts := make(map[string]int64, len(slugs))
	if len(slugs) == 0 {
		return counts, nil
	}

	type row struct {
		RequestSlug string
		Total       int64
	}
	var rows []row
	err := s.db.View(func(tx pkgdb.Tx) error {
		return tx.Read().Model(&models.BuyRequestOffer{}).
			Select("request_slug, COUNT(*) AS total").
			Where("request_slug IN ? AND status <> ? AND deleted_at IS NULL", slugs, models.BuyRequestOfferRejected).
			Group("request_slug").
			Scan(&rows).Error
	})
	if err != nil {
		return nil, err
	}
	for _, r := range rows {
		counts[r.RequestSlug] = r.Total
	}
	return counts, nil
}

func (s *GormBuyRequestStore) attachOfferCounts(requests []*models.BuyRequest) error {
	if len(requests) == 0 {
		return nil
	}
	slugs := make([]string, len(requests))
	for i, r := range requests {
		slugs[i] = r.Slug
	}
	counts, err := s.CountOffersForRequests(context.Background(), slugs)
	if err != nil {
		return err
	}
	for _, r := range requests {
		r.OfferCount = int(counts[r.Slug])
	}
	return nil
}

func normalizePage(page, pageSize int) (int, int) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}
	return page, pageSize
}

func (s *GormBuyRequestStore) CreateComment(_ context.Context, c *models.BuyRequestComment) error {
	now := time.Now()
	c.CreatedAt = now
	c.UpdatedAt = now
	return s.db.Update(func(tx pkgdb.Tx) error {
		return tx.Save(c)
	})
}

func (s *GormBuyRequestStore) GetComment(_ context.Context, id string) (*models.BuyRequestComment, error) {
	var c models.BuyRequestComment
	err := s.db.View(func(tx pkgdb.Tx) error {
		return tx.Read().Where("id = ? AND deleted_at IS NULL", id).First(&c).Error
	})
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrBuyRequestCommentNotFound
		}
		return nil, err
	}
	return &c, nil
}

// ListComments returns a request's comments oldest-first, which is the order a
// conversation reads in. The API groups replies under their parent.
func (s *GormBuyRequestStore) ListComments(_ context.Context, requestSlug string) ([]*models.BuyRequestComment, error) {
	var out []*models.BuyRequestComment
	err := s.db.View(func(tx pkgdb.Tx) error {
		return tx.Read().Where("request_slug = ? AND deleted_at IS NULL", requestSlug).
			Order("created_at ASC").Find(&out).Error
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (s *GormBuyRequestStore) CountComments(_ context.Context, requestSlug string) (int64, error) {
	var total int64
	err := s.db.View(func(tx pkgdb.Tx) error {
		return tx.Read().Model(&models.BuyRequestComment{}).
			Where("request_slug = ? AND deleted_at IS NULL", requestSlug).
			Count(&total).Error
	})
	return total, err
}
