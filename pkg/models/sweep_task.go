package models

import "time"

// SweepTask represents a pending or completed auto-sweep operation.
// After a Guest Order payment is confirmed (FUNDED), the node creates a
// SweepTask to transfer funds from the HD-derived address to the seller's
// receiving account address.
type SweepTask struct {
	TenantMixin
	ID           int    `gorm:"primaryKey;autoIncrement:false" json:"id"`
	OrderToken   string `gorm:"index;size:64" json:"orderToken"`
	ChainKey     string `json:"chainKey"`
	FromAddress  string `json:"fromAddress"`
	ToAddress    string `json:"toAddress"`
	Amount       string `json:"amount"`
	AddressIndex uint32 `json:"-"`
	Status       string `gorm:"index" json:"status"`
	TxHash       string `json:"txHash,omitempty"`
	RetryCount   int    `json:"retryCount"`
	LastError    string `json:"lastError,omitempty"`
	CreatedAt    time.Time `gorm:"autoCreateTime" json:"createdAt"`
	UpdatedAt    time.Time `gorm:"autoUpdateTime" json:"updatedAt"`
}

// TableName overrides the default GORM table name.
func (SweepTask) TableName() string { return "sweep_tasks" }

const (
	SweepStatusPending   = "pending"
	SweepStatusSubmitted = "submitted"
	SweepStatusConfirmed = "confirmed"
	SweepStatusFailed    = "failed"
)

const MaxSweepRetries = 5
