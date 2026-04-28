package models

// PhysicalDelivery specifies the delivery information for a physical good.
type PhysicalDelivery struct {
	Shipper        string `json:"shipper"`
	TrackingNumber string `json:"trackingNumber"`
}

// DigitalDelivery specifies the delivery information for a digital good.
type DigitalDelivery struct {
	URL      string `json:"url"`
	Password string `json:"password"`
}

// CryptocurrencyDelivery specifies the delivery information for a cryptocurrency listing.
type CryptocurrencyDelivery struct {
	TransactionID string `json:"transactionID"` // 交易哈希（适用于普通加密货币和 RWA Token）
}

// Shipment contains all the data needed to record an order shipment (seller ship action).
type Shipment struct {
	ItemIndex               int                     `json:"itemIndex"`
	Note                    string                  `json:"note"`
	PhysicalDelivery        *PhysicalDelivery       `json:"physicalDelivery"`
	DigitalDelivery         *DigitalDelivery        `json:"digitalDelivery"`
	CryptocurrencyDelivery  *CryptocurrencyDelivery `json:"cryptocurrencyDelivery"` // 也用于 RWA Token 发货
	ReceivingAccountAddress string                  `json:"receivingAccountAddress,omitempty"`
}
