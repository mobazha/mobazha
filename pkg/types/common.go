// Package types provides common types used across the Mobazha core library.
package types

import "time"

// Hash represents a cryptographic hash (typically SHA-256).
type Hash [32]byte

// String returns the hex string representation of the hash.
func (h Hash) String() string {
	const hexChars = "0123456789abcdef"
	result := make([]byte, 64)
	for i, b := range h {
		result[i*2] = hexChars[b>>4]
		result[i*2+1] = hexChars[b&0x0f]
	}
	return string(result)
}

// IsZero returns true if the hash is all zeros.
func (h Hash) IsZero() bool {
	for _, b := range h {
		if b != 0 {
			return false
		}
	}
	return true
}

// Timestamp wraps time.Time for consistent serialization.
type Timestamp time.Time

// Now returns the current timestamp.
func Now() Timestamp {
	return Timestamp(time.Now())
}

// Time returns the underlying time.Time.
func (t Timestamp) Time() time.Time {
	return time.Time(t)
}

// Unix returns the Unix timestamp.
func (t Timestamp) Unix() int64 {
	return time.Time(t).Unix()
}

// Currency represents a supported currency (crypto or fiat).
type Currency string

const (
	CurrencyBTC  Currency = "BTC"
	CurrencyBCH  Currency = "BCH"
	CurrencyLTC  Currency = "LTC"
	CurrencyZEC  Currency = "ZEC"
	CurrencyETH  Currency = "ETH"
	CurrencySOL  Currency = "SOL"
	CurrencyUSD  Currency = "USD"
	CurrencyEUR  Currency = "EUR"
	CurrencyCNY  Currency = "CNY"
)

// IsCrypto returns true if the currency is a cryptocurrency.
func (c Currency) IsCrypto() bool {
	switch c {
	case CurrencyBTC, CurrencyBCH, CurrencyLTC, CurrencyZEC, CurrencyETH, CurrencySOL:
		return true
	default:
		return false
	}
}

// IsFiat returns true if the currency is a fiat currency.
func (c Currency) IsFiat() bool {
	return !c.IsCrypto()
}
