package core

import (
	"errors"
	"strings"

	"github.com/mobazha/mobazha3.0/internal/database"
	"github.com/mobazha/mobazha3.0/pkg/models"
	"gorm.io/gorm"
)

// NotificationAppService encapsulates notification query and management logic.
type NotificationAppService struct {
	db database.Database
}

type NotificationAppServiceConfig struct {
	DB database.Database
}

func NewNotificationAppService(cfg NotificationAppServiceConfig) *NotificationAppService {
	return &NotificationAppService{db: cfg.DB}
}

func (s *NotificationAppService) GetNotifications(offsetID string, limit int, typeFilters []string) ([]models.NotificationRecord, int64, error) {
	typeFilterClause := ""
	var types []string
	if len(typeFilters) > 0 {
		typeFilterClauseParts := make([]string, 0, len(typeFilters))
		for i := 0; i < len(typeFilters); i++ {
			types = append(types, typeFilters[i])
			typeFilterClauseParts = append(typeFilterClauseParts, "?")
		}
		typeFilterClause = "type in (" + strings.Join(typeFilterClauseParts, ",") + ")"
	}
	var args []any
	if len(types) > 0 {
		for _, a := range types {
			args = append(args, a)
		}
	}

	var (
		notifications []models.NotificationRecord
		totalCount    int64
	)
	err := s.db.View(func(tx database.Tx) error {
		if len(types) > 0 {
			if err := tx.Read().Where(typeFilterClause, args...).Find(&models.NotificationRecord{}).Count(&totalCount).Error; err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
				return err
			}
		} else {
			if err := tx.Read().Find(&models.NotificationRecord{}).Count(&totalCount).Error; err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
				return err
			}
		}

		if offsetID != "" {
			var notification models.NotificationRecord
			err := tx.Read().Where("id = ?", offsetID).Order("timestamp desc").First(&notification).Error
			if err != nil {
				return err
			}
			if len(types) > 0 {
				return tx.Read().Where(typeFilterClause, args...).Where("timestamp < ?", notification.Timestamp).Limit(limit).Order("timestamp desc").Find(&notifications).Error
			}
			return tx.Read().Where("timestamp < ?", notification.Timestamp).Limit(limit).Order("timestamp desc").Find(&notifications).Error
		}
		if len(types) > 0 {
			return tx.Read().Where(typeFilterClause, args...).Limit(limit).Order("timestamp desc").Find(&notifications).Error
		}
		return tx.Read().Limit(limit).Order("timestamp desc").Find(&notifications).Error
	})
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, 0, err
	}
	return notifications, totalCount, nil
}

func (s *NotificationAppService) MarkNotificationAsRead(notifID string) error {
	return s.db.Update(func(tx database.Tx) error {
		notification := models.NotificationRecord{}
		if err := tx.Read().Order("timestamp desc").Where("id = ?", notifID).First(&notification).Error; err != nil {
			return err
		}
		if notification.Read {
			return nil
		}
		return tx.Update("read", true, map[string]any{"id = ?": notifID}, &models.NotificationRecord{})
	})
}

func (s *NotificationAppService) MarkAllNotificationsAsRead() error {
	return s.db.Update(func(tx database.Tx) error {
		return tx.Update("read", true, map[string]any{"read = ?": false}, &models.NotificationRecord{})
	})
}

func (s *NotificationAppService) GetNotificationsUnreadCount() (int, error) {
	var unreadCount int64
	err := s.db.View(func(tx database.Tx) error {
		return tx.Read().Model(&models.NotificationRecord{}).Where("read = ?", false).Count(&unreadCount).Error
	})
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return 0, err
	}
	return int(unreadCount), nil
}

func (s *NotificationAppService) GetNotificationsTotalCount() (int64, error) {
	var totalCount int64
	err := s.db.View(func(tx database.Tx) error {
		return tx.Read().Model(&models.NotificationRecord{}).Count(&totalCount).Error
	})
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return 0, err
	}
	return totalCount, nil
}

// BatchMarkNotificationsAsRead marks multiple notifications as read by IDs.
func (s *NotificationAppService) BatchMarkNotificationsAsRead(ids []string) error {
	if len(ids) == 0 {
		return nil
	}
	return s.db.Update(func(tx database.Tx) error {
		return tx.Update("read", true, map[string]interface{}{"id IN ?": ids}, &models.NotificationRecord{})
	})
}

// BatchDeleteNotifications deletes multiple notifications by IDs.
func (s *NotificationAppService) BatchDeleteNotifications(ids []string) error {
	if len(ids) == 0 {
		return nil
	}
	return s.db.Update(func(tx database.Tx) error {
		for _, id := range ids {
			if err := tx.Delete("id", id, nil, &models.NotificationRecord{}); err != nil {
				return err
			}
		}
		return nil
	})
}
