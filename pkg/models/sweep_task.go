package models

import "time"

// SweepTask represents a pending or completed auto-sweep operation.
// After a Guest Order payment is confirmed (FUNDED), the node creates a
// SweepTask to transfer funds from the HD-derived address to the seller's
// receiving account address. Affiliate UTXO payouts use the same task and
// lifecycle, but are signed by the local escrow master key instead of BIP44.
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

// SweepKeySource identifies the private-key domain that owns FromAddress.
// It is deliberately task-local: a task must never infer its key source from
// an address or fall back from affiliate escrow keys to Guest BIP44 keys.
type SweepKeySource string

const (
	// SweepKeySourceBIP44 signs Guest Checkout payment addresses derived from
	// the node's BIP44 master key and AddressIndex.
	SweepKeySourceBIP44 SweepKeySource = "bip44"
	// SweepKeySourceAffiliateEscrow signs the promoter-controlled BTC/BCH/LTC
	// address derived directly from the node's EscrowMasterKey public key.
	SweepKeySourceAffiliateEscrow SweepKeySource = "affiliate_escrow"
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
