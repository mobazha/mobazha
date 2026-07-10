package database

import (
	"context"
	"errors"
	"sort"
	"time"

	"github.com/mobazha/mobazha/pkg/contracts"
	pkgdb "github.com/mobazha/mobazha/pkg/database"
	"github.com/mobazha/mobazha/pkg/models"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var (
	// ErrSellerAffiliateNotFound indicates a missing tenant-local affiliate resource.
	ErrSellerAffiliateNotFound = models.ErrSellerAffiliateNotFound
	// ErrSellerAffiliateConflict indicates an immutable binding or lifecycle conflict.
	ErrSellerAffiliateConflict = models.ErrSellerAffiliateConflict
)

// GormSellerAffiliateStore persists the minimal tenant-scoped affiliate domain.
type GormSellerAffiliateStore struct {
	db pkgdb.Database
}

var _ contracts.SellerAffiliateStore = (*GormSellerAffiliateStore)(nil)

// NewGormSellerAffiliateStore constructs a tenant-local affiliate store.
func NewGormSellerAffiliateStore(db pkgdb.Database) *GormSellerAffiliateStore {
	return &GormSellerAffiliateStore{db: db}
}

// MigrateSellerAffiliateModels creates only the Phase 1 affiliate tables.
func MigrateSellerAffiliateModels(db pkgdb.Database) error {
	return db.Update(func(tx pkgdb.Tx) error {
		for _, model := range []interface{}{
			&models.AffiliateProgram{},
			&models.AffiliateLink{},
			&models.AffiliateReferralSession{},
			&models.AffiliateAttribution{},
			&models.AffiliateCommissionLine{},
		} {
			if err := tx.Migrate(model); err != nil {
				return err
			}
		}
		return nil
	})
}

// PutAffiliateProgram creates or updates the tenant's single seller program.
func (s *GormSellerAffiliateStore) PutAffiliateProgram(_ context.Context, program *models.AffiliateProgram) error {
	if s == nil || s.db == nil || program == nil {
		return models.ErrInvalidSellerAffiliate
	}
	if err := program.Validate(); err != nil {
		return err
	}
	return s.db.Update(func(tx pkgdb.Tx) error {
		var existing models.AffiliateProgram
		err := tx.Read().First(&existing).Error
		if err == nil && (existing.ID != program.ID || existing.SellerPeerID != program.SellerPeerID) {
			return ErrSellerAffiliateConflict
		}
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		return tx.Save(program)
	})
}

// GetAffiliateProgram returns the tenant's single seller program.
func (s *GormSellerAffiliateStore) GetAffiliateProgram(_ context.Context) (*models.AffiliateProgram, error) {
	var program models.AffiliateProgram
	err := s.db.View(func(tx pkgdb.Tx) error {
		return tx.Read().First(&program).Error
	})
	return affiliateResult(&program, err)
}

// CreateAffiliateLink inserts one direct promoter link.
func (s *GormSellerAffiliateStore) CreateAffiliateLink(_ context.Context, link *models.AffiliateLink) error {
	if link == nil {
		return models.ErrInvalidSellerAffiliate
	}
	if err := link.Validate(); err != nil {
		return err
	}
	return s.db.Update(func(tx pkgdb.Tx) error { return tx.Create(link) })
}

// GetAffiliateLink returns one tenant-local promoter link.
func (s *GormSellerAffiliateStore) GetAffiliateLink(_ context.Context, id string) (*models.AffiliateLink, error) {
	var link models.AffiliateLink
	err := s.db.View(func(tx pkgdb.Tx) error {
		return tx.Read().Where("id = ?", id).First(&link).Error
	})
	return affiliateResult(&link, err)
}

// GetAffiliateLinkByToken resolves one tenant-local public token.
func (s *GormSellerAffiliateStore) GetAffiliateLinkByToken(_ context.Context, token string) (*models.AffiliateLink, error) {
	var link models.AffiliateLink
	err := s.db.View(func(tx pkgdb.Tx) error {
		return tx.Read().Where("public_token = ?", token).First(&link).Error
	})
	return affiliateResult(&link, err)
}

// GetAffiliateLinkByPromoter resolves the tenant's single direct link for a promoter.
func (s *GormSellerAffiliateStore) GetAffiliateLinkByPromoter(_ context.Context, programID, promoterPeerID string) (*models.AffiliateLink, error) {
	var link models.AffiliateLink
	err := s.db.View(func(tx pkgdb.Tx) error {
		return tx.Read().Where("program_id = ? AND promoter_peer_id = ?", programID, promoterPeerID).First(&link).Error
	})
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrSellerAffiliateNotFound
	}
	if err != nil {
		return nil, err
	}
	return &link, nil
}

// CreateAffiliateReferralSession inserts one immutable referral session.
func (s *GormSellerAffiliateStore) CreateAffiliateReferralSession(_ context.Context, session *models.AffiliateReferralSession) error {
	if session == nil {
		return models.ErrInvalidSellerAffiliate
	}
	if err := session.Validate(); err != nil {
		return err
	}
	return s.db.Update(func(tx pkgdb.Tx) error { return tx.Create(session) })
}

// GetAffiliateReferralSession returns one tenant-local referral session.
func (s *GormSellerAffiliateStore) GetAffiliateReferralSession(_ context.Context, id string) (*models.AffiliateReferralSession, error) {
	var session models.AffiliateReferralSession
	err := s.db.View(func(tx pkgdb.Tx) error {
		return tx.Read().Where("id = ?", id).First(&session).Error
	})
	return affiliateResult(&session, err)
}

// RecordAffiliateOrder atomically inserts attribution and line commissions.
func (s *GormSellerAffiliateStore) RecordAffiliateOrder(_ context.Context, result *models.AffiliateOrderResult) (*models.AffiliateOrderResult, error) {
	if result == nil || len(result.Lines) == 0 {
		return nil, models.ErrInvalidSellerAffiliate
	}
	if err := result.Attribution.Validate(); err != nil {
		return nil, err
	}
	for index := range result.Lines {
		line := &result.Lines[index]
		if line.AttributionID != result.Attribution.ID || line.OrderID != result.Attribution.OrderID {
			return nil, models.ErrInvalidSellerAffiliate
		}
		if err := line.Validate(); err != nil {
			return nil, err
		}
	}

	stored := new(models.AffiliateOrderResult)
	err := s.db.Update(func(tx pkgdb.Tx) error {
		var existing models.AffiliateAttribution
		err := tx.Read().Where("order_id = ?", result.Attribution.OrderID).First(&existing).Error
		if err == nil {
			var lines []models.AffiliateCommissionLine
			if err := tx.Read().Where("order_id = ?", existing.OrderID).Order("order_line_id ASC").Find(&lines).Error; err != nil {
				return err
			}
			if !sameAffiliateOrderSnapshot(existing, lines, result) {
				return ErrSellerAffiliateConflict
			}
			stored.Attribution = existing
			stored.Lines = lines
			return nil
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		if err := tx.Create(&result.Attribution); err != nil {
			return err
		}
		for index := range result.Lines {
			if err := tx.Create(&result.Lines[index]); err != nil {
				return err
			}
		}
		stored.Attribution = result.Attribution
		stored.Lines = append([]models.AffiliateCommissionLine(nil), result.Lines...)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return stored, nil
}

// GetAffiliateAttributionByOrder returns one immutable order attribution.
func (s *GormSellerAffiliateStore) GetAffiliateAttributionByOrder(_ context.Context, orderID string) (*models.AffiliateAttribution, error) {
	var attribution models.AffiliateAttribution
	err := s.db.View(func(tx pkgdb.Tx) error {
		return tx.Read().Where("order_id = ?", orderID).First(&attribution).Error
	})
	return affiliateResult(&attribution, err)
}

// ListAffiliateCommissionLinesByOrder returns stable line ordering for statements.
func (s *GormSellerAffiliateStore) ListAffiliateCommissionLinesByOrder(_ context.Context, orderID string) ([]models.AffiliateCommissionLine, error) {
	var lines []models.AffiliateCommissionLine
	err := s.db.View(func(tx pkgdb.Tx) error {
		return tx.Read().Where("order_id = ?", orderID).Order("order_line_id ASC").Find(&lines).Error
	})
	if err != nil {
		return nil, err
	}
	return lines, nil
}

// ListAffiliateStatementLines returns line-level statement projections without
// copying commission amounts into a second persistent aggregate.
func (s *GormSellerAffiliateStore) ListAffiliateStatementLines(_ context.Context, promoterPeerID string) ([]models.AffiliateStatementLine, error) {
	attributions := make([]models.AffiliateAttribution, 0)
	lines := make([]models.AffiliateCommissionLine, 0)
	err := s.db.View(func(tx pkgdb.Tx) error {
		query := tx.Read().Order("attributed_at DESC, id ASC")
		if promoterPeerID != "" {
			query = query.Where("promoter_peer_id = ?", promoterPeerID)
		}
		if err := query.Find(&attributions).Error; err != nil || len(attributions) == 0 {
			return err
		}
		attributionIDs := make([]string, 0, len(attributions))
		for _, attribution := range attributions {
			attributionIDs = append(attributionIDs, attribution.ID)
		}
		return tx.Read().Where("attribution_id IN ?", attributionIDs).
			Order("created_at DESC, order_line_id ASC").Find(&lines).Error
	})
	if err != nil {
		return nil, err
	}
	attributionByID := make(map[string]models.AffiliateAttribution, len(attributions))
	for _, attribution := range attributions {
		attributionByID[attribution.ID] = attribution
	}
	statement := make([]models.AffiliateStatementLine, 0, len(lines))
	for _, line := range lines {
		attribution, ok := attributionByID[line.AttributionID]
		if !ok {
			continue
		}
		statement = append(statement, models.AffiliateStatementLine{Attribution: attribution, CommissionLine: line})
	}
	return statement, nil
}

// ListPendingAffiliateCommissionOrderIDs returns distinct tenant-local pending orders.
func (s *GormSellerAffiliateStore) ListPendingAffiliateCommissionOrderIDs(_ context.Context) ([]string, error) {
	var orderIDs []string
	err := s.db.View(func(tx pkgdb.Tx) error {
		return tx.Read().Model(&models.AffiliateCommissionLine{}).
			Distinct("order_id").
			Where("status = ?", models.AffiliateCommissionStatusPending).
			Order("order_id ASC").
			Pluck("order_id", &orderIDs).Error
	})
	if err != nil {
		return nil, err
	}
	return orderIDs, nil
}

// TransitionAffiliateCommission advances all order lines without manual review.
func (s *GormSellerAffiliateStore) TransitionAffiliateCommission(_ context.Context, orderID string, status models.AffiliateCommissionStatus, reason models.AffiliateCommissionReversalReason, at time.Time) ([]models.AffiliateCommissionLine, error) {
	var lines []models.AffiliateCommissionLine
	err := s.db.Update(func(tx pkgdb.Tx) error {
		if err := tx.Read().Clauses(clause.Locking{Strength: "UPDATE"}).Where("order_id = ?", orderID).Order("order_line_id ASC").Find(&lines).Error; err != nil {
			return err
		}
		if len(lines) == 0 {
			return ErrSellerAffiliateNotFound
		}
		for index := range lines {
			line := &lines[index]
			if line.Status == status {
				if status == models.AffiliateCommissionStatusReversed && line.ReversalReason != reason {
					return ErrSellerAffiliateConflict
				}
				continue
			}
			switch status {
			case models.AffiliateCommissionStatusEarned:
				if line.Status != models.AffiliateCommissionStatusPending {
					return ErrSellerAffiliateConflict
				}
				line.Status = status
			case models.AffiliateCommissionStatusReversed:
				if line.Status != models.AffiliateCommissionStatusPending && line.Status != models.AffiliateCommissionStatusEarned {
					return ErrSellerAffiliateConflict
				}
				line.Status = status
				line.ReversalReason = reason
				reversedAt := at
				line.ReversedAt = &reversedAt
			default:
				return models.ErrInvalidSellerAffiliate
			}
			line.UpdatedAt = at
			if err := line.Validate(); err != nil {
				return err
			}
			if err := tx.Save(line); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return lines, nil
}

func affiliateResult[T any](value *T, err error) (*T, error) {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrSellerAffiliateNotFound
	}
	if err != nil {
		return nil, err
	}
	return value, nil
}

func sameAffiliateOrderSnapshot(existing models.AffiliateAttribution, storedLines []models.AffiliateCommissionLine, requested *models.AffiliateOrderResult) bool {
	if existing.OrderID != requested.Attribution.OrderID || existing.ReferralSessionID != requested.Attribution.ReferralSessionID ||
		existing.ProgramID != requested.Attribution.ProgramID || existing.SellerPeerID != requested.Attribution.SellerPeerID ||
		existing.BuyerPeerID != requested.Attribution.BuyerPeerID || existing.PromoterPeerID != requested.Attribution.PromoterPeerID ||
		existing.CommissionRateBPSSnapshot != requested.Attribution.CommissionRateBPSSnapshot || len(storedLines) != len(requested.Lines) {
		return false
	}
	requestedLines := append([]models.AffiliateCommissionLine(nil), requested.Lines...)
	sort.Slice(requestedLines, func(i, j int) bool { return requestedLines[i].OrderLineID < requestedLines[j].OrderLineID })
	for index := range storedLines {
		stored := storedLines[index]
		request := requestedLines[index]
		if stored.OrderLineID != request.OrderLineID || stored.NetMerchandiseAtomic != request.NetMerchandiseAtomic ||
			stored.Currency != request.Currency || stored.CommissionRateBPSSnapshot != request.CommissionRateBPSSnapshot ||
			stored.CommissionAtomic != request.CommissionAtomic {
			return false
		}
	}
	return true
}
