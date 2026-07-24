package models

import "time"

const (
	// PaymentSelectionQuotePilotZeroFeeV1 freezes payment selection with
	// explicit zero provider/network and platform costs until a later policy is
	// approved. It covers Deal and standard (non-Deal) cross-currency orders.
	// A future non-zero policy must use a new version.
	PaymentSelectionQuotePilotZeroFeeV1 = "deal-payment-selection-zero-fee-v1"
)

// PaymentSelectionQuote is an immutable, server-authored snapshot binding a
// signed order's pricing amount to one canonical payment asset. Deal orders
// also bind feeQuoteID / dealLinkID / revision / termsHash; standard orders
// leave those fields empty. Quote rows are never updated; expiry or policy
// changes create another row.
type PaymentSelectionQuote struct {
	TenantID                      string    `gorm:"column:tenant_id;primaryKey;default:''"`
	QuoteID                       string    `gorm:"column:quote_id;primaryKey;size:64"`
	OrderID                       string    `gorm:"column:order_id;size:255;not null;index:idx_payment_selection_order"`
	FeeQuoteID                    string    `gorm:"column:fee_quote_id;size:128;not null;index:idx_payment_selection_fee_quote"`
	DealLinkID                    string    `gorm:"column:deal_link_id;size:128;not null"`
	DealRevision                  uint64    `gorm:"column:deal_revision;not null"`
	TermsHash                     string    `gorm:"column:terms_hash;size:64;not null"`
	SchemaVersion                 uint      `gorm:"column:schema_version;not null;default:1"`
	PolicyVersion                 string    `gorm:"column:policy_version;size:64;not null;index:idx_payment_selection_policy"`
	PricingCurrency               string    `gorm:"column:pricing_currency;size:32;not null"`
	PricingAmount                 string    `gorm:"column:pricing_amount;size:128;not null"`
	PricingDivisibility           uint      `gorm:"column:pricing_divisibility;not null"`
	PaymentCoin                   string    `gorm:"column:payment_coin;size:255;not null;index:idx_payment_selection_coin"`
	PaymentCurrency               string    `gorm:"column:payment_currency;size:32;not null"`
	PaymentDivisibility           uint      `gorm:"column:payment_divisibility;not null"`
	ConversionRequired            bool      `gorm:"column:conversion_required;not null"`
	ExchangeRate                  string    `gorm:"column:exchange_rate;size:128;not null"`
	ExchangeRateBase              string    `gorm:"column:exchange_rate_base;size:32;not null"`
	ExchangeRateQuote             string    `gorm:"column:exchange_rate_quote;size:32;not null"`
	ExchangeRateQuoteDivisibility uint      `gorm:"column:exchange_rate_quote_divisibility;not null"`
	RateSourceUpdatedAt           time.Time `gorm:"column:rate_source_updated_at"`
	PaymentSubtotal               string    `gorm:"column:payment_subtotal;size:128;not null"`
	ProviderOrNetworkCost         string    `gorm:"column:provider_or_network_cost;size:128;not null"`
	PlatformPaymentCost           string    `gorm:"column:platform_payment_cost;size:128;not null"`
	BuyerPaymentTotal             string    `gorm:"column:buyer_payment_total;size:128;not null"`
	ExpiresAt                     time.Time `gorm:"column:expires_at;not null;index:idx_payment_selection_expiry"`
	CreatedAt                     time.Time `gorm:"column:created_at;not null"`
}

func (PaymentSelectionQuote) TableName() string { return "payment_selection_quotes" }
