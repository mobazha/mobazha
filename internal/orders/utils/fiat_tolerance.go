package utils

const (
	// Same-currency tolerance: allow 1% under (rounding) and 1% over.
	fiatSameCurrencyMinRatio = 0.99
	fiatSameCurrencyMaxRatio = 1.01

	// Cross-currency tolerance: allow 2% under (FX drift) and 5% over.
	fiatCrossCurrencyMinRatio = 0.98
	fiatCrossCurrencyMaxRatio = 1.05
)
