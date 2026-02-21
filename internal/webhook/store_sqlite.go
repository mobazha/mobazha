package webhook

import (
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/mobazha/mobazha3.0/pkg/database"
	wh "github.com/mobazha/mobazha3.0/pkg/webhook"
	"gorm.io/gorm"
)

// WebhookEndpoint is the GORM model for webhook endpoint persistence.
type WebhookEndpoint struct {
	ID         string     `gorm:"primaryKey;type:varchar(36)" json:"id"`
	TenantID   string     `gorm:"primaryKey;type:varchar(255)" json:"-"`
	URL        string     `gorm:"type:varchar(2048);not null" json:"url"`
	Secret     string     `gorm:"type:varchar(255);not null" json:"-"`
	EventTypes string     `gorm:"type:text;not null" json:"event_types"`
	Active     bool       `gorm:"not null;default:true" json:"active"`
	CreatedAt  time.Time  `gorm:"not null" json:"created_at"`
	UpdatedAt  *time.Time `json:"updated_at,omitempty"`
}

func (WebhookEndpoint) TableName() string { return "webhook_endpoints" }

// WebhookDelivery is the GORM model for webhook delivery tracking.
type WebhookDelivery struct {
	ID             string     `gorm:"primaryKey;type:varchar(36)" json:"id"`
	TenantID       string     `gorm:"primaryKey;type:varchar(255)" json:"-"`
	EndpointID     string     `gorm:"type:varchar(36);not null;index:idx_wh_del_ep" json:"endpoint_id"`
	EventType      string     `gorm:"type:varchar(100);not null" json:"event_type"`
	Payload        string     `gorm:"type:text;not null" json:"payload"`
	Status         string     `gorm:"type:varchar(20);not null;default:pending" json:"status"`
	Attempts       int        `gorm:"not null;default:0" json:"attempts"`
	MaxAttempts    int        `gorm:"not null;default:5" json:"max_attempts"`
	NextRetryAt    *time.Time `json:"next_retry_at,omitempty"`
	LastStatusCode *int       `json:"last_status_code,omitempty"`
	LastError      string     `gorm:"type:text" json:"last_error,omitempty"`
	CreatedAt      time.Time  `gorm:"not null" json:"created_at"`
	CompletedAt    *time.Time `json:"completed_at,omitempty"`
}

func (WebhookDelivery) TableName() string { return "webhook_deliveries" }

// SQLiteStore implements wh.EndpointStore using the node's database.Database.
// TenantID is always StandaloneTenantID ("_default") for standalone nodes.
type SQLiteStore struct {
	db database.Database
}

// NewSQLiteStore creates a new SQLiteStore.
func NewSQLiteStore(db database.Database) *SQLiteStore {
	return &SQLiteStore{db: db}
}

// MigrateModels should be called during repo initialization to create tables.
func MigrateModels(db database.Database) error {
	return db.Update(func(tx database.Tx) error {
		if err := tx.Migrate(&WebhookEndpoint{}); err != nil {
			return err
		}
		return tx.Migrate(&WebhookDelivery{})
	})
}

func (s *SQLiteStore) ListActive() ([]wh.Endpoint, error) {
	var records []WebhookEndpoint
	err := s.db.View(func(tx database.Tx) error {
		return tx.Read().Where("active = ?", true).Find(&records).Error
	})
	if err != nil {
		return nil, err
	}
	return toEndpoints(records), nil
}

func (s *SQLiteStore) GetEndpoint(id string) (*wh.Endpoint, error) {
	var rec WebhookEndpoint
	err := s.db.View(func(tx database.Tx) error {
		return tx.Read().Where("id = ?", id).First(&rec).Error
	})
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, wh.ErrEndpointNotFound
		}
		return nil, err
	}
	ep := toEndpoint(rec)
	return &ep, nil
}

func (s *SQLiteStore) CreateEndpoint(ep *wh.Endpoint) error {
	if ep.ID == "" {
		ep.ID = uuid.New().String()
	}
	if ep.Secret == "" {
		ep.Secret = uuid.New().String()
	}
	rec := &WebhookEndpoint{
		ID:         ep.ID,
		URL:        ep.URL,
		Secret:     ep.Secret,
		EventTypes: ep.EventTypes,
		Active:     ep.Active,
		CreatedAt:  time.Now(),
	}
	err := s.db.Update(func(tx database.Tx) error {
		return tx.Save(rec)
	})
	if err != nil {
		return err
	}
	ep.Secret = rec.Secret
	ep.CreatedAt = rec.CreatedAt
	return nil
}

func (s *SQLiteStore) UpdateEndpoint(id string, updates map[string]interface{}) error {
	return s.db.Update(func(tx database.Tx) error {
		var rec WebhookEndpoint
		if err := tx.Read().Where("id = ?", id).First(&rec).Error; err != nil {
			return err
		}
		if v, ok := updates["url"]; ok {
			if s, ok := v.(string); ok {
				rec.URL = s
			}
		}
		if v, ok := updates["event_types"]; ok {
			if s, ok := v.(string); ok {
				rec.EventTypes = s
			}
		}
		if v, ok := updates["active"]; ok {
			if b, ok := v.(bool); ok {
				rec.Active = b
			}
		}
		now := time.Now()
		rec.UpdatedAt = &now
		return tx.Save(&rec)
	})
}

func (s *SQLiteStore) DeleteEndpoint(id string) error {
	return s.db.Update(func(tx database.Tx) error {
		if err := tx.Delete("endpoint_id", id, nil, &WebhookDelivery{}); err != nil {
			return err
		}
		return tx.Delete("id", id, nil, &WebhookEndpoint{})
	})
}

func (s *SQLiteStore) ListEndpoints() ([]wh.Endpoint, error) {
	var records []WebhookEndpoint
	err := s.db.View(func(tx database.Tx) error {
		return tx.Read().Order("created_at DESC").Find(&records).Error
	})
	if err != nil {
		return nil, err
	}
	return toEndpoints(records), nil
}

func (s *SQLiteStore) CountEndpoints() (int64, error) {
	var count int64
	err := s.db.View(func(tx database.Tx) error {
		return tx.Read().Model(&WebhookEndpoint{}).Count(&count).Error
	})
	return count, err
}

func (s *SQLiteStore) CreateDeliveries(deliveries []wh.Delivery) error {
	if len(deliveries) == 0 {
		return nil
	}
	return s.db.Update(func(tx database.Tx) error {
		for i := range deliveries {
			if deliveries[i].ID == "" {
				deliveries[i].ID = uuid.New().String()
			}
			rec := &WebhookDelivery{
				ID:          deliveries[i].ID,
				EndpointID:  deliveries[i].EndpointID,
				EventType:   deliveries[i].EventType,
				Payload:     deliveries[i].Payload,
				Status:      deliveries[i].Status,
				Attempts:    deliveries[i].Attempts,
				MaxAttempts: deliveries[i].MaxAttempts,
				NextRetryAt: deliveries[i].NextRetryAt,
				CreatedAt:   deliveries[i].CreatedAt,
			}
			if err := tx.Save(rec); err != nil {
				return err
			}
		}
		return nil
	})
}

func (s *SQLiteStore) GetPending(limit int) ([]wh.Delivery, error) {
	if limit <= 0 {
		limit = 50
	}
	now := time.Now()
	var records []WebhookDelivery
	err := s.db.View(func(tx database.Tx) error {
		return tx.Read().Where(
			"(status = ? AND (next_retry_at IS NULL OR next_retry_at <= ?)) OR (status = ? AND attempts < max_attempts AND next_retry_at <= ?)",
			wh.DeliveryStatusPending, now, wh.DeliveryStatusFailed, now,
		).Order("created_at ASC").Limit(limit).Find(&records).Error
	})
	if err != nil {
		return nil, err
	}
	return toDeliveries(records), nil
}

func (s *SQLiteStore) UpdateResult(id string, result wh.DeliveryResult) error {
	return s.db.Update(func(tx database.Tx) error {
		var rec WebhookDelivery
		if err := tx.Read().Where("id = ?", id).First(&rec).Error; err != nil {
			return err
		}
		rec.Status = result.Status
		rec.Attempts++
		if result.StatusCode != nil {
			rec.LastStatusCode = result.StatusCode
		}
		if result.Error != "" {
			rec.LastError = result.Error
		}
		if result.NextRetry != nil {
			rec.NextRetryAt = result.NextRetry
		}
		if result.Status == wh.DeliveryStatusSuccess || result.Status == wh.DeliveryStatusFailed {
			now := time.Now()
			rec.CompletedAt = &now
		}
		return tx.Save(&rec)
	})
}

func (s *SQLiteStore) CleanupOld(olderThan time.Duration) (int64, error) {
	cutoff := time.Now().Add(-olderThan)
	var count int64
	err := s.db.Update(func(tx database.Tx) error {
		if err := tx.Read().Model(&WebhookDelivery{}).
			Where("status IN (?, ?) AND created_at < ?",
				wh.DeliveryStatusSuccess, wh.DeliveryStatusFailed, cutoff).
			Count(&count).Error; err != nil {
			return err
		}
		if count == 0 {
			return nil
		}
		if err := tx.Delete("status", wh.DeliveryStatusSuccess,
			map[string]interface{}{"created_at < ?": cutoff}, &WebhookDelivery{}); err != nil {
			return fmt.Errorf("cleanup success deliveries: %w", err)
		}
		return tx.Delete("status", wh.DeliveryStatusFailed,
			map[string]interface{}{"created_at < ?": cutoff}, &WebhookDelivery{})
	})
	return count, err
}

func (s *SQLiteStore) ListDeliveries(endpointID string, status string, limit, offset int) ([]wh.Delivery, int64, error) {
	if limit <= 0 {
		limit = 50
	}
	var records []WebhookDelivery
	var total int64
	err := s.db.View(func(tx database.Tx) error {
		q := tx.Read().Model(&WebhookDelivery{}).Where("endpoint_id = ?", endpointID)
		if status != "" {
			q = q.Where("status = ?", status)
		}
		if err := q.Count(&total).Error; err != nil {
			return err
		}
		return q.Order("created_at DESC").Limit(limit).Offset(offset).Find(&records).Error
	})
	if err != nil {
		return nil, 0, err
	}
	return toDeliveries(records), total, nil
}

// --- conversion helpers ---

func toEndpoint(r WebhookEndpoint) wh.Endpoint {
	return wh.Endpoint{
		ID:         r.ID,
		URL:        r.URL,
		Secret:     r.Secret,
		EventTypes: r.EventTypes,
		Active:     r.Active,
		CreatedAt:  r.CreatedAt,
	}
}

func toEndpoints(records []WebhookEndpoint) []wh.Endpoint {
	result := make([]wh.Endpoint, len(records))
	for i, r := range records {
		result[i] = toEndpoint(r)
	}
	return result
}

func toDelivery(r WebhookDelivery) wh.Delivery {
	return wh.Delivery{
		ID:          r.ID,
		EndpointID:  r.EndpointID,
		EventType:   r.EventType,
		Payload:     r.Payload,
		Status:      r.Status,
		Attempts:    r.Attempts,
		MaxAttempts: r.MaxAttempts,
		NextRetryAt: r.NextRetryAt,
		CreatedAt:   r.CreatedAt,
	}
}

func toDeliveries(records []WebhookDelivery) []wh.Delivery {
	result := make([]wh.Delivery, len(records))
	for i, r := range records {
		result[i] = toDelivery(r)
	}
	return result
}
