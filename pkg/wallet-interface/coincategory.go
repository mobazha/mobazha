package wallet_interface

type CoinCategory string

const (
	CoinCategoryUnknown  CoinCategory = "Unknown"
	CoinCategoryBitcoin  CoinCategory = "Bitcoin"
	CoinCategoryEthereum CoinCategory = "Ethereum"
	CoinCategorySolana   CoinCategory = "Solana"
	CoinCategoryFiat CoinCategory = "Fiat"
)
