package payment

import "time"

// PaymentSelectionQuote is the public immutable quote returned before a Deal
// payment session is provisioned. Monetary fields are smallest-unit decimal
// strings in their named currency.
type PaymentSelectionQuote struct {
	ID                            string    `json:"id"`
	OrderID                       string    `json:"orderID"`
	FeeQuoteID                    string    `json:"feeQuoteID"`
	DealLinkID                    string    `json:"dealLinkID"`
	DealRevision                  uint64    `json:"dealRevision"`
	TermsHash                     string    `json:"termsHash"`
	SchemaVersion                 uint      `json:"schemaVersion"`
	PolicyVersion                 string    `json:"policyVersion"`
	PricingCurrency               string    `json:"pricingCurrency"`
	PricingAmount                 string    `json:"pricingAmount"`
	PricingDivisibility           uint      `json:"pricingDivisibility"`
	PaymentCoin                   string    `json:"paymentCoin"`
	PaymentCurrency               string    `json:"paymentCurrency"`
	PaymentDivisibility           uint      `json:"paymentDivisibility"`
	ConversionRequired            bool      `json:"conversionRequired"`
	ExchangeRate                  string    `json:"exchangeRate"`
	ExchangeRateBase              string    `json:"exchangeRateBase"`
	ExchangeRateQuote             string    `json:"exchangeRateQuote"`
	ExchangeRateQuoteDivisibility uint      `json:"exchangeRateQuoteDivisibility"`
	RateSourceUpdatedAt           time.Time `json:"rateSourceUpdatedAt,omitempty"`
	PaymentSubtotal               string    `json:"paymentSubtotal"`
	ProviderOrNetworkCost         string    `json:"providerOrNetworkCost"`
	PlatformPaymentCost           string    `json:"platformPaymentCost"`
	BuyerPaymentTotal             string    `json:"buyerPaymentTotal"`
	ExpiresAt                     time.Time `json:"expiresAt"`
	CreatedAt                     time.Time `json:"createdAt"`
}
