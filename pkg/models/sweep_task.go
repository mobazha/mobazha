package models

import "time"

// SweepTask is the read-only schema sentinel for pre-ADR-016 Guest auto-sweep
// rows. No runtime service creates or processes this model; startup fails when
// legacy key sources are present so funds must be migrated explicitly.
type SweepTask struct {
	TenantMixin
	ID                     int            `gorm:"primaryKey;autoIncrement:false" json:"id"`
	OrderToken             string         `gorm:"index;size:64" json:"orderToken"`
	ChainKey               string         `json:"chainKey"`
	FromAddress            string         `json:"fromAddress"`
	ToAddress              string         `json:"toAddress"`
	Amount                 string         `json:"amount"`
	AffiliatePayoutAddress string         `gorm:"column:affiliate_payout_address;type:text" json:"-"`
	AffiliatePayoutAmount  string         `gorm:"column:affiliate_payout_amount;type:text" json:"-"`
	AddressIndex           uint32         `json:"-"`
	KeySource              SweepKeySource `gorm:"type:text;not null;default:'bip44'" json:"-"`
	Status                 string         `gorm:"index" json:"status"`
	TxHash                 string         `json:"txHash,omitempty"`
	RetryCount             int            `json:"retryCount"`
	LastError              string         `json:"lastError,omitempty"`
	CreatedAt              time.Time      `gorm:"autoCreateTime" json:"createdAt"`
	UpdatedAt              time.Time      `gorm:"autoUpdateTime" json:"updatedAt"`
}

// SweepKeySource identifies a retired sweep private-key domain.
type SweepKeySource string

const (
	// SweepKeySourceBIP44 identifies the retired Guest account-0 path.
	SweepKeySourceBIP44 SweepKeySource = "bip44"
)

// TableName overrides the default GORM table name.
func (SweepTask) TableName() string { return "sweep_tasks" }

const (
	SweepStatusPending    = "pending"
	SweepStatusProcessing = "processing"
	SweepStatusSubmitted  = "submitted"
	SweepStatusConfirmed  = "confirmed"
	SweepStatusFailed     = "failed"
)

const SweepStaleTimeout = 10 * time.Minute

const MaxSweepRetries = 5
