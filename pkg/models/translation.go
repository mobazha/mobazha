// SPDX-License-Identifier: MPL-2.0
// Copyright (c) 2026 fengzie and the respective contributors.

package models

import "time"

// Translation is one finished translation, kept so the same text is never paid
// for twice.
//
// The cache key is a hash of the source language, the target language and the
// source text, so an edited product is a different key and gets translated
// again, while the same text asked for by a hundred visitors is translated once.
// The key is content-addressed rather than tied to a listing, because the same
// sentence can appear in a listing, a wanted request or a store description.
type Translation struct {
	ID       string `json:"id" gorm:"primaryKey;type:text"`
	TenantID string `json:"-" gorm:"column:tenant_id;type:text;not null;default:'_default';index"`
	// CacheKey is the hex sha256 described above. It is stored as a column of
	// its own rather than as the primary key so the row keeps a stable id even
	// if the hashing ever changes.
	CacheKey string `json:"cacheKey" gorm:"column:cache_key;type:text;not null;uniqueIndex"`
	// SourceLang is the language the text was written in, or "" when the engine
	// was left to detect it. It is part of the key: a caller that states the
	// language gets a different entry than one that does not.
	SourceLang string `json:"sourceLang" gorm:"column:source_lang;type:text;not null"`
	TargetLang string `json:"targetLang" gorm:"column:target_lang;type:text;not null"`
	// SourceHash is the sha256 of the source text alone, so a row can be
	// invalidated or audited without re-deriving the full key.
	SourceHash string `json:"sourceHash" gorm:"column:source_hash;type:text;not null"`
	Text       string `json:"text" gorm:"column:text;type:text;not null"`
	// Engine names what produced the text, so a bad batch can be found and
	// re-translated after the engine or the prompt changes.
	Engine    string    `json:"engine" gorm:"column:engine;type:text"`
	CreatedAt time.Time `json:"createdAt" gorm:"column:created_at;autoCreateTime"`
	UpdatedAt time.Time `json:"updatedAt" gorm:"column:updated_at;autoUpdateTime"`
}

func (Translation) TableName() string { return "translations" }

// TranslationUsage is how many characters a node has sent to an engine in one
// month.
//
// A paid engine bills per character, and a node that translates a whole catalog
// is spending its owner's money, so the counter is kept next to the cache rather
// than left to the engine's own dashboard: the node has to be able to refuse
// before it spends, not find out afterwards.
type TranslationUsage struct {
	TenantID string `json:"-" gorm:"column:tenant_id;type:text;not null;default:'_default';primaryKey"`
	// Month is "2006-01" in UTC.
	Month     string    `json:"month" gorm:"column:month;type:text;not null;primaryKey"`
	Chars     int64     `json:"chars" gorm:"column:chars;not null;default:0"`
	UpdatedAt time.Time `json:"updatedAt" gorm:"column:updated_at;autoUpdateTime"`
}

func (TranslationUsage) TableName() string { return "translation_usage" }
