// SPDX-License-Identifier: MPL-2.0
// Copyright (c) 2026 fengzie and the respective contributors.

package database

import (
	"context"
	"errors"
	"time"

	pkgcontracts "github.com/mobazha/mobazha/pkg/contracts"
	pkgdb "github.com/mobazha/mobazha/pkg/database"
	"github.com/mobazha/mobazha/pkg/models"
	"gorm.io/gorm"
)

type GormTranslationStore struct {
	db pkgdb.Database
}

var _ pkgcontracts.TranslationStore = (*GormTranslationStore)(nil)

func NewGormTranslationStore(db pkgdb.Database) *GormTranslationStore {
	return &GormTranslationStore{db: db}
}

// MigrateTranslationModels creates/updates the translation cache and the
// monthly usage counter.
func MigrateTranslationModels(db pkgdb.Database) error {
	return db.Update(func(tx pkgdb.Tx) error {
		if err := tx.Migrate(&models.Translation{}); err != nil {
			return err
		}
		return tx.Migrate(&models.TranslationUsage{})
	})
}

// GetTranslations returns what is already cached for the given keys. A key that
// is missing is simply absent from the result: a cache miss is the normal case,
// not an error.
func (s *GormTranslationStore) GetTranslations(_ context.Context, cacheKeys []string) (map[string]*models.Translation, error) {
	found := make(map[string]*models.Translation, len(cacheKeys))
	if len(cacheKeys) == 0 {
		return found, nil
	}

	var rows []*models.Translation
	err := s.db.View(func(tx pkgdb.Tx) error {
		return tx.Read().Where("cache_key IN ?", cacheKeys).Find(&rows).Error
	})
	if err != nil {
		return nil, err
	}
	for _, row := range rows {
		if row == nil {
			continue
		}
		found[row.CacheKey] = row
	}
	return found, nil
}

// SaveTranslations writes finished translations.
//
// A row is looked up by its cache key first and updated in place when it is
// already there, rather than inserted blindly: two visitors asking for the same
// text at the same moment would otherwise race on the unique index, and the
// translation itself is identical either way.
func (s *GormTranslationStore) SaveTranslations(_ context.Context, entries []*models.Translation) error {
	if len(entries) == 0 {
		return nil
	}
	now := time.Now()
	return s.db.Update(func(tx pkgdb.Tx) error {
		for _, entry := range entries {
			if entry == nil || entry.CacheKey == "" {
				continue
			}
			var existing models.Translation
			err := tx.Read().Where("cache_key = ?", entry.CacheKey).First(&existing).Error
			switch {
			case err == nil:
				return tx.Read().Model(&models.Translation{}).
					Where("cache_key = ?", entry.CacheKey).
					Updates(map[string]interface{}{
						"text":       entry.Text,
						"engine":     entry.Engine,
						"updated_at": now,
					}).Error
			case errors.Is(err, gorm.ErrRecordNotFound):
				entry.CreatedAt = now
				entry.UpdatedAt = now
				if err := tx.Save(entry); err != nil {
					return err
				}
			default:
				return err
			}
		}
		return nil
	})
}

// AddUsage counts the characters sent to an engine this month. The counter is
// what lets a node refuse the next call instead of discovering the bill later.
func (s *GormTranslationStore) AddUsage(_ context.Context, month string, chars int64) error {
	if month == "" || chars <= 0 {
		return nil
	}
	return s.db.Update(func(tx pkgdb.Tx) error {
		var existing models.TranslationUsage
		err := tx.Read().Where("month = ?", month).First(&existing).Error
		switch {
		case err == nil:
			return tx.Read().Model(&models.TranslationUsage{}).
				Where("month = ?", month).
				Updates(map[string]interface{}{
					"chars":      existing.Chars + chars,
					"updated_at": time.Now(),
				}).Error
		case errors.Is(err, gorm.ErrRecordNotFound):
			return tx.Save(&models.TranslationUsage{Month: month, Chars: chars})
		default:
			return err
		}
	})
}

// Usage reports how many characters have been sent this month. A month with no
// row has spent nothing.
func (s *GormTranslationStore) Usage(_ context.Context, month string) (int64, error) {
	if month == "" {
		return 0, nil
	}
	var row models.TranslationUsage
	err := s.db.View(func(tx pkgdb.Tx) error {
		return tx.Read().Where("month = ?", month).First(&row).Error
	})
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return 0, nil
		}
		return 0, err
	}
	return row.Chars, nil
}

// DeleteTranslationsOlderThan drops cached translations written before a moment.
//
// The cache is a cost optimisation, not a record: an entry that is dropped is
// translated again the next time that wording is seen, which is why the caller
// gives a generous window rather than a tight one.
func (s *GormTranslationStore) DeleteTranslationsOlderThan(_ context.Context, before time.Time) (int64, error) {
	if before.IsZero() {
		return 0, nil
	}
	var deleted int64
	err := s.db.Update(func(tx pkgdb.Tx) error {
		result := tx.Read().Where("updated_at < ?", before).Delete(&models.Translation{})
		deleted = result.RowsAffected
		return result.Error
	})
	return deleted, err
}
