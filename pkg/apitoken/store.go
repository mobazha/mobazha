package apitoken

import (
	"errors"
	stdlog "log"
	"time"

	"gorm.io/gorm"
)

// APITokenModel is the GORM model for the api_tokens table.
type APITokenModel struct {
	ID          int64      `gorm:"primaryKey;autoIncrement"`
	Name        string     `gorm:"type:varchar(128);not null"`
	TokenHash   string     `gorm:"type:varchar(64);not null;uniqueIndex"`
	TokenPrefix string     `gorm:"column:prefix;type:varchar(12);not null;uniqueIndex"`
	Scopes      string     `gorm:"type:text;not null;default:'[]'"`
	LastUsedAt  *time.Time
	ExpiresAt   *time.Time
	RevokedAt   *time.Time
	CreatedAt   time.Time  `gorm:"not null"`
}

func (APITokenModel) TableName() string { return "api_tokens" }

// Store provides CRUD operations for API tokens backed by GORM.
type Store struct {
	db *gorm.DB
}

// NewStore creates a Store and auto-migrates the api_tokens table.
func NewStore(db *gorm.DB) (*Store, error) {
	if err := db.AutoMigrate(&APITokenModel{}); err != nil {
		return nil, err
	}
	return &Store{db: db}, nil
}

// Create persists a new API token record.
func (s *Store) Create(t *Token) error {
	scopeJSON, err := MarshalScopes(t.Scopes)
	if err != nil {
		return err
	}
	m := APITokenModel{
		Name:        t.Name,
		TokenHash:   t.TokenHash,
		TokenPrefix: t.TokenPrefix,
		Scopes:      scopeJSON,
		ExpiresAt:   t.ExpiresAt,
		CreatedAt:   t.CreatedAt,
	}
	if err := s.db.Create(&m).Error; err != nil {
		return err
	}
	t.ID = m.ID
	return nil
}

// FindByPrefix looks up an active token by its 8-char hex prefix.
func (s *Store) FindByPrefix(prefix string) (*Token, error) {
	var m APITokenModel
	err := s.db.Where("prefix = ? AND revoked_at IS NULL", prefix).First(&m).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrTokenNotFound
	}
	if err != nil {
		return nil, err
	}
	return modelToToken(&m), nil
}

// FindByHash looks up a token by its SHA-256 hash.
func (s *Store) FindByHash(hash string) (*Token, error) {
	var m APITokenModel
	err := s.db.Where("token_hash = ?", hash).First(&m).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrTokenNotFound
	}
	if err != nil {
		return nil, err
	}
	return modelToToken(&m), nil
}

// List returns all tokens (including revoked), ordered by creation time desc.
func (s *Store) List() ([]Token, error) {
	var models []APITokenModel
	if err := s.db.Order("created_at DESC").Find(&models).Error; err != nil {
		return nil, err
	}
	tokens := make([]Token, 0, len(models))
	for i := range models {
		tokens = append(tokens, *modelToToken(&models[i]))
	}
	return tokens, nil
}

// Revoke marks a token as revoked by its ID.
func (s *Store) Revoke(tokenID int64) error {
	now := time.Now()
	result := s.db.Model(&APITokenModel{}).
		Where("id = ? AND revoked_at IS NULL", tokenID).
		Update("revoked_at", now)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrTokenNotFound
	}
	return nil
}

// TouchUsage updates the last_used_at timestamp.
func (s *Store) TouchUsage(tokenID int64) {
	now := time.Now()
	s.db.Model(&APITokenModel{}).Where("id = ?", tokenID).Update("last_used_at", now)
}

// CountActive returns the number of non-revoked, non-expired tokens.
func (s *Store) CountActive() (int64, error) {
	var count int64
	err := s.db.Model(&APITokenModel{}).
		Where("revoked_at IS NULL AND (expires_at IS NULL OR expires_at > ?)", time.Now()).
		Count(&count).Error
	return count, err
}

// modelToToken maps the GORM row to a domain Token. Scope JSON corruption is
// treated as a soft failure: the token still authenticates (TokenHash/Prefix
// are intact), but its scope set is empty so the downstream scope enforcement
// layer denies privileged actions by default. A warning is logged so admins
// can detect and repair the row.
func modelToToken(m *APITokenModel) *Token {
	scopes, err := UnmarshalScopes(m.Scopes)
	if err != nil {
		stdlog.Printf("[apitoken] WARNING: token id=%d prefix=%s has corrupt scopes JSON (%v); treating as empty scope set",
			m.ID, m.TokenPrefix, err)
		scopes = nil
	}
	return &Token{
		ID:          m.ID,
		Name:        m.Name,
		TokenHash:   m.TokenHash,
		TokenPrefix: m.TokenPrefix,
		Scopes:      scopes,
		LastUsedAt:  m.LastUsedAt,
		ExpiresAt:   m.ExpiresAt,
		RevokedAt:   m.RevokedAt,
		CreatedAt:   m.CreatedAt,
	}
}
