package models

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"math/big"
	"runtime/debug"
	"strings"
	"time"

	iwallet "github.com/mobazha/mobazha3.0/pkg/wallet-interface"
	"github.com/op/go-logging"
)

// DefaultCurrencyDivisibility is the Divisibility of the Currency if not
// defined otherwise
const DefaultCurrencyDivisibility uint32 = 1e8

var (
	log = logging.MustGetLogger("MDLS")

	ErrCurrencyValueInsufficientPrecision = errors.New("unable to accurately represent value as int64")
	ErrCurrencyValueNegativeRate          = errors.New("conversion rate must be greater than zero")
	ErrCurrencyValueAmountInvalid         = errors.New("invalid amount")
	ErrCurrencyDefinitionUndefined        = errors.New("currency definition is not defined")

	CurrencyDefinitions = CurrencyDictionary{
		// Testing
		"MCK": {Name: "Mock", Code: CurrencyCode("MCK"), CurrencyType: CurrencyTypeCrypto, Divisibility: 8, BlockInterval: time.Minute * 10},

		// Crypto
		"BTC":       {Name: "Bitcoin", Code: CurrencyCode("BTC"), CurrencyType: CurrencyTypeCrypto, Divisibility: 8, Bip44Code: 0, BlockInterval: time.Minute * 10},
		"BCH":       {Name: "Bitcoin Cash", Code: CurrencyCode("BCH"), CurrencyType: CurrencyTypeCrypto, Divisibility: 8, Bip44Code: 145, BlockInterval: time.Minute * 10},
		"LTC":       {Name: "Litecoin", Code: CurrencyCode("LTC"), CurrencyType: CurrencyTypeCrypto, Divisibility: 8, Bip44Code: 2, BlockInterval: time.Second * 150},
		"ZEC":       {Name: "Zcash", Code: CurrencyCode("ZEC"), CurrencyType: CurrencyTypeCrypto, Divisibility: 8, Bip44Code: 133, BlockInterval: time.Second * 150},
		"ETH":       {Name: "Ethereum", Code: CurrencyCode("ETH"), CurrencyType: CurrencyTypeCrypto, Divisibility: 18, Bip44Code: 60, BlockInterval: time.Second * 12},
		"EXTERNAL_PAYMENT":       {Name: "ExternalPayment", Code: CurrencyCode("EXTERNAL_PAYMENT"), CurrencyType: CurrencyTypeCrypto, Divisibility: 12, Bip44Code: 128, BlockInterval: time.Minute * 2},
		"DASH":      {Name: "Dash", Code: CurrencyCode("DASH"), CurrencyType: CurrencyTypeCrypto, Divisibility: 8, Bip44Code: 5, BlockInterval: time.Second * 150},
		"XRP":       {Name: "Ripple", Code: CurrencyCode("XRP"), CurrencyType: CurrencyTypeCrypto, Divisibility: 6, Bip44Code: 144},
		"BNB":       {Name: "Binance Coin", Code: CurrencyCode("BNB"), CurrencyType: CurrencyTypeCrypto, Divisibility: 18, Bip44Code: 60, BlockInterval: time.Second * 3},
		"BNBUSDT":   {Name: "Tether", Code: CurrencyCode("BNBUSDT"), CurrencyType: CurrencyTypeCrypto, Divisibility: 6, BlockInterval: time.Second * 3},
		"BNBUSDC":   {Name: "USD Coin", Code: CurrencyCode("BNBUSDC"), CurrencyType: CurrencyTypeCrypto, Divisibility: 6, BlockInterval: time.Second * 3},
		"BNBMBZ":    {Name: "Mobazha Token", Code: CurrencyCode("BNBMBZ"), CurrencyType: CurrencyTypeCrypto, Divisibility: 18, BlockInterval: time.Second * 3},
		"MATIC":     {Name: "Polygon", Code: CurrencyCode("MATIC"), CurrencyType: CurrencyTypeCrypto, Divisibility: 18, Bip44Code: 60, BlockInterval: time.Second * 3},
		"MATICUSDT": {Name: "Tether", Code: CurrencyCode("MATICUSDT"), CurrencyType: CurrencyTypeCrypto, Divisibility: 6, BlockInterval: time.Second * 3},
		"MATICUSDC": {Name: "USD Coin", Code: CurrencyCode("MATICUSDC"), CurrencyType: CurrencyTypeCrypto, Divisibility: 6, BlockInterval: time.Second * 3},
		"MATICMBZ":  {Name: "Mobazha Token", Code: CurrencyCode("MATICMBZ"), CurrencyType: CurrencyTypeCrypto, Divisibility: 18, BlockInterval: time.Second * 3},

		"CFX":     {Name: "Conflux", Code: CurrencyCode("CFX"), CurrencyType: CurrencyTypeCrypto, Divisibility: 18, Bip44Code: 60, BlockInterval: time.Second * 3},
		"CFXUSDT": {Name: "Tether", Code: CurrencyCode("CFXUSDT"), CurrencyType: CurrencyTypeCrypto, Divisibility: 18, BlockInterval: time.Second * 3},
		"CFXUSDC": {Name: "USD Coin", Code: CurrencyCode("CFXUSDC"), CurrencyType: CurrencyTypeCrypto, Divisibility: 18, BlockInterval: time.Second * 3},
		"CFXMBZ":  {Name: "Mobazha Token", Code: CurrencyCode("CFXMBZ"), CurrencyType: CurrencyTypeCrypto, Divisibility: 18, BlockInterval: time.Second * 3},

		"SOL": {Name: "Solana", Code: CurrencyCode("SOL"), CurrencyType: CurrencyTypeCrypto, Divisibility: 9, Bip44Code: 501, BlockInterval: time.Millisecond * 400},

		"USDT":  {Name: "Tether", Code: CurrencyCode("USDT"), CurrencyType: CurrencyTypeCrypto, Divisibility: 6, BlockInterval: time.Second * 3},
		"USDC":  {Name: "USD Coin", Code: CurrencyCode("USDC"), CurrencyType: CurrencyTypeCrypto, Divisibility: 6, BlockInterval: time.Second * 3},
		"EOS":   {Name: "EOS", Code: CurrencyCode("EOS"), CurrencyType: CurrencyTypeCrypto, Divisibility: 4, Bip44Code: 194},
		"BSV":   {Name: "Bitcoin SV", Code: CurrencyCode("BSV"), CurrencyType: CurrencyTypeCrypto, Divisibility: 8, Bip44Code: 236},
		"XLM":   {Name: "Stellar Lumens", Code: CurrencyCode("XLM"), CurrencyType: CurrencyTypeCrypto, Divisibility: 7, Bip44Code: 148},
		"LEO":   {Name: "UNUS SED LEO", Code: CurrencyCode("LEO"), CurrencyType: CurrencyTypeCrypto, Divisibility: 18},
		"ADA":   {Name: "Cardano", Code: CurrencyCode("ADA"), CurrencyType: CurrencyTypeCrypto, Divisibility: 6, Bip44Code: 1815},
		"TRX":   {Name: "Tron", Code: CurrencyCode("TRX"), CurrencyType: CurrencyTypeCrypto, Divisibility: 6, Bip44Code: 195},
		"LINK":  {Name: "Chainlink", Code: CurrencyCode("LINK"), CurrencyType: CurrencyTypeCrypto, Divisibility: 18},
		"XTZ":   {Name: "Tezos", Code: CurrencyCode("XTZ"), CurrencyType: CurrencyTypeCrypto, Divisibility: 6, Bip44Code: 1729},
		"NEO":   {Name: "NEO", Code: CurrencyCode("NEO"), CurrencyType: CurrencyTypeCrypto, Divisibility: 8, Bip44Code: 888},
		"MIOTA": {Name: "IOTA", Code: CurrencyCode("MIOTA"), CurrencyType: CurrencyTypeCrypto, Divisibility: 6, Bip44Code: 4218},
		"ETC":   {Name: "Ethereum Classic", Code: CurrencyCode("ETC"), CurrencyType: CurrencyTypeCrypto, Divisibility: 18, Bip44Code: 61},

		// Fiat
		"AED": {Name: "UAE Dirham", Code: CurrencyCode("AED"), CurrencyType: CurrencyTypeFiat, Divisibility: 2},
		"AFN": {Name: "Afghani", Code: CurrencyCode("AFN"), CurrencyType: CurrencyTypeFiat, Divisibility: 2},
		"ALL": {Name: "Lek", Code: CurrencyCode("ALL"), CurrencyType: CurrencyTypeFiat, Divisibility: 2},
		"AMD": {Name: "Armenian Dram", Code: CurrencyCode("AMD"), CurrencyType: CurrencyTypeFiat, Divisibility: 2},
		"ANG": {Name: "Netherlands Antillean Guilder", Code: CurrencyCode("ANG"), CurrencyType: CurrencyTypeFiat, Divisibility: 2},
		"AOA": {Name: "Kwanza", Code: CurrencyCode("AOA"), CurrencyType: CurrencyTypeFiat, Divisibility: 2},
		"ARS": {Name: "Argentine Peso", Code: CurrencyCode("ARS"), CurrencyType: CurrencyTypeFiat, Divisibility: 2},
		"AUD": {Name: "Australian Dollar", Code: CurrencyCode("AUD"), CurrencyType: CurrencyTypeFiat, Divisibility: 2},
		"AWG": {Name: "Aruban Florin", Code: CurrencyCode("AWG"), CurrencyType: CurrencyTypeFiat, Divisibility: 2},
		"AZN": {Name: "Azerbaijanian Manat", Code: CurrencyCode("AZN"), CurrencyType: CurrencyTypeFiat, Divisibility: 2},
		"BAM": {Name: "Convertible Mark", Code: CurrencyCode("BAM"), CurrencyType: CurrencyTypeFiat, Divisibility: 2},
		"BBD": {Name: "Barbados Dollar", Code: CurrencyCode("BBD"), CurrencyType: CurrencyTypeFiat, Divisibility: 2},
		"BDT": {Name: "Taka", Code: CurrencyCode("BDT"), CurrencyType: CurrencyTypeFiat, Divisibility: 2},
		"BGN": {Name: "Bulgarian Lev", Code: CurrencyCode("BGN"), CurrencyType: CurrencyTypeFiat, Divisibility: 2},
		"BHD": {Name: "Bahraini Dinar", Code: CurrencyCode("BHD"), CurrencyType: CurrencyTypeFiat, Divisibility: 2},
		"BIF": {Name: "Burundi Franc", Code: CurrencyCode("BIF"), CurrencyType: CurrencyTypeFiat, Divisibility: 2},
		"BMD": {Name: "Bermudian Dollar", Code: CurrencyCode("BMD"), CurrencyType: CurrencyTypeFiat, Divisibility: 2},
		"BND": {Name: "Brunei Dollar", Code: CurrencyCode("BND"), CurrencyType: CurrencyTypeFiat, Divisibility: 2},
		"BOB": {Name: "Boliviano", Code: CurrencyCode("BOB"), CurrencyType: CurrencyTypeFiat, Divisibility: 2},
		"BRL": {Name: "Brazilian Real", Code: CurrencyCode("BRL"), CurrencyType: CurrencyTypeFiat, Divisibility: 2},
		"BSD": {Name: "Bahamian Dollar", Code: CurrencyCode("BSD"), CurrencyType: CurrencyTypeFiat, Divisibility: 2},
		"BTN": {Name: "Ngultrum", Code: CurrencyCode("BTN"), CurrencyType: CurrencyTypeFiat, Divisibility: 2},
		"BWP": {Name: "Pula", Code: CurrencyCode("BWP"), CurrencyType: CurrencyTypeFiat, Divisibility: 2},
		"BYR": {Name: "Belarussian Ruble", Code: CurrencyCode("BYR"), CurrencyType: CurrencyTypeFiat, Divisibility: 2},
		"BZD": {Name: "Belize Dollar", Code: CurrencyCode("BZD"), CurrencyType: CurrencyTypeFiat, Divisibility: 2},
		"CAD": {Name: "Canadian Dollar", Code: CurrencyCode("CAD"), CurrencyType: CurrencyTypeFiat, Divisibility: 2},
		"CDF": {Name: "Congolese Franc", Code: CurrencyCode("CDF"), CurrencyType: CurrencyTypeFiat, Divisibility: 2},
		"CHF": {Name: "Swiss Franc", Code: CurrencyCode("CHF"), CurrencyType: CurrencyTypeFiat, Divisibility: 2},
		"CLP": {Name: "Chilean Peso", Code: CurrencyCode("CLP"), CurrencyType: CurrencyTypeFiat, Divisibility: 2},
		"CNY": {Name: "Yuan Renminbi", Code: CurrencyCode("CNY"), CurrencyType: CurrencyTypeFiat, Divisibility: 2},
		"COP": {Name: "Colombian Peso", Code: CurrencyCode("COP"), CurrencyType: CurrencyTypeFiat, Divisibility: 2},
		"CRC": {Name: "Costa Rican Colon", Code: CurrencyCode("CRC"), CurrencyType: CurrencyTypeFiat, Divisibility: 2},
		"CUP": {Name: "Cuban Peso", Code: CurrencyCode("CUP"), CurrencyType: CurrencyTypeFiat, Divisibility: 2},
		"CVE": {Name: "Cabo Verde Escudo", Code: CurrencyCode("CVE"), CurrencyType: CurrencyTypeFiat, Divisibility: 2},
		"CZK": {Name: "Czech Koruna", Code: CurrencyCode("CZK"), CurrencyType: CurrencyTypeFiat, Divisibility: 2},
		"DJF": {Name: "Djibouti Franc", Code: CurrencyCode("DJF"), CurrencyType: CurrencyTypeFiat, Divisibility: 2},
		"DKK": {Name: "Danish Krone", Code: CurrencyCode("DKK"), CurrencyType: CurrencyTypeFiat, Divisibility: 2},
		"DOP": {Name: "Dominican Peso", Code: CurrencyCode("DOP"), CurrencyType: CurrencyTypeFiat, Divisibility: 2},
		"DZD": {Name: "Algerian Dinar", Code: CurrencyCode("DZD"), CurrencyType: CurrencyTypeFiat, Divisibility: 2},
		"EGP": {Name: "Egyptian Pound", Code: CurrencyCode("EGP"), CurrencyType: CurrencyTypeFiat, Divisibility: 2},
		"ERN": {Name: "Nakfa", Code: CurrencyCode("ERN"), CurrencyType: CurrencyTypeFiat, Divisibility: 2},
		"ETB": {Name: "Ethiopian Birr", Code: CurrencyCode("ETB"), CurrencyType: CurrencyTypeFiat, Divisibility: 2},
		"EUR": {Name: "Euro", Code: CurrencyCode("EUR"), CurrencyType: CurrencyTypeFiat, Divisibility: 2},
		"FJD": {Name: "Fiji Dollar", Code: CurrencyCode("FJD"), CurrencyType: CurrencyTypeFiat, Divisibility: 2},
		"FKP": {Name: "Falkland Islands Pound", Code: CurrencyCode("FKP"), CurrencyType: CurrencyTypeFiat, Divisibility: 2},
		"GBP": {Name: "Pound Sterling", Code: CurrencyCode("GBP"), CurrencyType: CurrencyTypeFiat, Divisibility: 2},
		"GEL": {Name: "Lari", Code: CurrencyCode("GEL"), CurrencyType: CurrencyTypeFiat, Divisibility: 2},
		"GHS": {Name: "Ghana Cedi", Code: CurrencyCode("GHS"), CurrencyType: CurrencyTypeFiat, Divisibility: 2},
		"GIP": {Name: "Gibraltar Pound", Code: CurrencyCode("GIP"), CurrencyType: CurrencyTypeFiat, Divisibility: 2},
		"GMD": {Name: "Dalasi", Code: CurrencyCode("GMD"), CurrencyType: CurrencyTypeFiat, Divisibility: 2},
		"GNF": {Name: "Guinea Franc", Code: CurrencyCode("GNF"), CurrencyType: CurrencyTypeFiat, Divisibility: 2},
		"GTQ": {Name: "Quetzal", Code: CurrencyCode("GTQ"), CurrencyType: CurrencyTypeFiat, Divisibility: 2},
		"GYD": {Name: "Guyana Dollar", Code: CurrencyCode("GYD"), CurrencyType: CurrencyTypeFiat, Divisibility: 2},
		"HKD": {Name: "Hong Kong Dollar", Code: CurrencyCode("HKD"), CurrencyType: CurrencyTypeFiat, Divisibility: 2},
		"HNL": {Name: "Lempira", Code: CurrencyCode("HNL"), CurrencyType: CurrencyTypeFiat, Divisibility: 2},
		"HRK": {Name: "Kuna", Code: CurrencyCode("HRK"), CurrencyType: CurrencyTypeFiat, Divisibility: 2},
		"HTG": {Name: "Gourde", Code: CurrencyCode("HTG"), CurrencyType: CurrencyTypeFiat, Divisibility: 2},
		"HUF": {Name: "Forint", Code: CurrencyCode("HUF"), CurrencyType: CurrencyTypeFiat, Divisibility: 2},
		"IDR": {Name: "Rupiah", Code: CurrencyCode("IDR"), CurrencyType: CurrencyTypeFiat, Divisibility: 2},
		"ILS": {Name: "New Israeli Sheqel", Code: CurrencyCode("ILS"), CurrencyType: CurrencyTypeFiat, Divisibility: 2},
		"INR": {Name: "Indian Rupee", Code: CurrencyCode("INR"), CurrencyType: CurrencyTypeFiat, Divisibility: 2},
		"IQD": {Name: "Iraqi Dinar", Code: CurrencyCode("IQD"), CurrencyType: CurrencyTypeFiat, Divisibility: 2},
		"IRR": {Name: "Iranian Rial", Code: CurrencyCode("IRR"), CurrencyType: CurrencyTypeFiat, Divisibility: 2},
		"ISK": {Name: "Iceland Krona", Code: CurrencyCode("ISK"), CurrencyType: CurrencyTypeFiat, Divisibility: 2},
		"JMD": {Name: "Jamaican Dollar", Code: CurrencyCode("JMD"), CurrencyType: CurrencyTypeFiat, Divisibility: 2},
		"JOD": {Name: "Jordanian Dinar", Code: CurrencyCode("JOD"), CurrencyType: CurrencyTypeFiat, Divisibility: 2},
		"JPY": {Name: "Yen", Code: CurrencyCode("JPY"), CurrencyType: CurrencyTypeFiat, Divisibility: 2},
		"KES": {Name: "Kenyan Shilling", Code: CurrencyCode("KES"), CurrencyType: CurrencyTypeFiat, Divisibility: 2},
		"KGS": {Name: "Som", Code: CurrencyCode("KGS"), CurrencyType: CurrencyTypeFiat, Divisibility: 2},
		"KHR": {Name: "Riel", Code: CurrencyCode("KHR"), CurrencyType: CurrencyTypeFiat, Divisibility: 2},
		"KMF": {Name: "Comoro Franc", Code: CurrencyCode("KMF"), CurrencyType: CurrencyTypeFiat, Divisibility: 2},
		"KPW": {Name: "North Korean Won", Code: CurrencyCode("KPW"), CurrencyType: CurrencyTypeFiat, Divisibility: 2},
		"KRW": {Name: "Won", Code: CurrencyCode("KRW"), CurrencyType: CurrencyTypeFiat, Divisibility: 2},
		"KWD": {Name: "Kuwaiti Dinar", Code: CurrencyCode("KWD"), CurrencyType: CurrencyTypeFiat, Divisibility: 2},
		"KYD": {Name: "Cayman Islands Dollar", Code: CurrencyCode("KYD"), CurrencyType: CurrencyTypeFiat, Divisibility: 2},
		"KZT": {Name: "Tenge", Code: CurrencyCode("KZT"), CurrencyType: CurrencyTypeFiat, Divisibility: 2},
		"LAK": {Name: "Kip", Code: CurrencyCode("LAK"), CurrencyType: CurrencyTypeFiat, Divisibility: 2},
		"LBP": {Name: "Lebanese Pound", Code: CurrencyCode("LBP"), CurrencyType: CurrencyTypeFiat, Divisibility: 2},
		"LKR": {Name: "Sri Lanka Rupee", Code: CurrencyCode("LKR"), CurrencyType: CurrencyTypeFiat, Divisibility: 2},
		"LRD": {Name: "Liberian Dollar", Code: CurrencyCode("LRD"), CurrencyType: CurrencyTypeFiat, Divisibility: 2},
		"LSL": {Name: "Loti", Code: CurrencyCode("LSL"), CurrencyType: CurrencyTypeFiat, Divisibility: 2},
		"LYD": {Name: "Libyan Dinar", Code: CurrencyCode("LYD"), CurrencyType: CurrencyTypeFiat, Divisibility: 2},
		"MAD": {Name: "Moroccan Dirham", Code: CurrencyCode("MAD"), CurrencyType: CurrencyTypeFiat, Divisibility: 2},
		"MDL": {Name: "Moldovan Leu", Code: CurrencyCode("MDL"), CurrencyType: CurrencyTypeFiat, Divisibility: 2},
		"MGA": {Name: "Malagasy Ariary", Code: CurrencyCode("MGA"), CurrencyType: CurrencyTypeFiat, Divisibility: 2},
		"MKD": {Name: "Denar", Code: CurrencyCode("MKD"), CurrencyType: CurrencyTypeFiat, Divisibility: 2},
		"MMK": {Name: "Kyat", Code: CurrencyCode("MMK"), CurrencyType: CurrencyTypeFiat, Divisibility: 2},
		"MNT": {Name: "Tugrik", Code: CurrencyCode("MNT"), CurrencyType: CurrencyTypeFiat, Divisibility: 2},
		"MOP": {Name: "Pataca", Code: CurrencyCode("MOP"), CurrencyType: CurrencyTypeFiat, Divisibility: 2},
		"MRO": {Name: "Ouguiya", Code: CurrencyCode("MRO"), CurrencyType: CurrencyTypeFiat, Divisibility: 2},
		"MUR": {Name: "Mauritius Rupee", Code: CurrencyCode("MUR"), CurrencyType: CurrencyTypeFiat, Divisibility: 2},
		"MVR": {Name: "Rufiyaa", Code: CurrencyCode("MVR"), CurrencyType: CurrencyTypeFiat, Divisibility: 2},
		"MWK": {Name: "Kwacha", Code: CurrencyCode("MWK"), CurrencyType: CurrencyTypeFiat, Divisibility: 2},
		"MXN": {Name: "Mexican Peso", Code: CurrencyCode("MXN"), CurrencyType: CurrencyTypeFiat, Divisibility: 2},
		"MYR": {Name: "Malaysian Ringgit", Code: CurrencyCode("MYR"), CurrencyType: CurrencyTypeFiat, Divisibility: 2},
		"MZN": {Name: "Mozambique Metical", Code: CurrencyCode("MZN"), CurrencyType: CurrencyTypeFiat, Divisibility: 2},
		"NAD": {Name: "Namibia Dollar", Code: CurrencyCode("NAD"), CurrencyType: CurrencyTypeFiat, Divisibility: 2},
		"NGN": {Name: "Naira", Code: CurrencyCode("NGN"), CurrencyType: CurrencyTypeFiat, Divisibility: 2},
		"NIO": {Name: "Cordoba Oro", Code: CurrencyCode("NIO"), CurrencyType: CurrencyTypeFiat, Divisibility: 2},
		"NOK": {Name: "Norwegian Krone", Code: CurrencyCode("NOK"), CurrencyType: CurrencyTypeFiat, Divisibility: 2},
		"NPR": {Name: "Nepalese Rupee", Code: CurrencyCode("NPR"), CurrencyType: CurrencyTypeFiat, Divisibility: 2},
		"NZD": {Name: "New Zealand Dollar", Code: CurrencyCode("NZD"), CurrencyType: CurrencyTypeFiat, Divisibility: 2},
		"OMR": {Name: "Rial Omani", Code: CurrencyCode("OMR"), CurrencyType: CurrencyTypeFiat, Divisibility: 2},
		"PAB": {Name: "Balboa", Code: CurrencyCode("PAB"), CurrencyType: CurrencyTypeFiat, Divisibility: 2},
		"PEN": {Name: "Nuevo Sol", Code: CurrencyCode("PEN"), CurrencyType: CurrencyTypeFiat, Divisibility: 2},
		"PGK": {Name: "Kina", Code: CurrencyCode("PGK"), CurrencyType: CurrencyTypeFiat, Divisibility: 2},
		"PHP": {Name: "Philippine Peso", Code: CurrencyCode("PHP"), CurrencyType: CurrencyTypeFiat, Divisibility: 2},
		"PKR": {Name: "Pakistan Rupee", Code: CurrencyCode("PKR"), CurrencyType: CurrencyTypeFiat, Divisibility: 2},
		"PLN": {Name: "Zloty", Code: CurrencyCode("PLN"), CurrencyType: CurrencyTypeFiat, Divisibility: 2},
		"PYG": {Name: "Guarani", Code: CurrencyCode("PYG"), CurrencyType: CurrencyTypeFiat, Divisibility: 2},
		"QAR": {Name: "Qatari Rial", Code: CurrencyCode("QAR"), CurrencyType: CurrencyTypeFiat, Divisibility: 2},
		"RON": {Name: "Romanian Leu", Code: CurrencyCode("RON"), CurrencyType: CurrencyTypeFiat, Divisibility: 2},
		"RSD": {Name: "Serbian Dinar", Code: CurrencyCode("RSD"), CurrencyType: CurrencyTypeFiat, Divisibility: 2},
		"RUB": {Name: "Russian Ruble", Code: CurrencyCode("RUB"), CurrencyType: CurrencyTypeFiat, Divisibility: 2},
		"RWF": {Name: "Rwanda Franc", Code: CurrencyCode("RWF"), CurrencyType: CurrencyTypeFiat, Divisibility: 2},
		"SAR": {Name: "Saudi Riyal", Code: CurrencyCode("SAR"), CurrencyType: CurrencyTypeFiat, Divisibility: 2},
		"SBD": {Name: "Solomon Islands Dollar", Code: CurrencyCode("SBD"), CurrencyType: CurrencyTypeFiat, Divisibility: 2},
		"SCR": {Name: "Seychelles Rupee", Code: CurrencyCode("SCR"), CurrencyType: CurrencyTypeFiat, Divisibility: 2},
		"SDG": {Name: "Sudanese Pound", Code: CurrencyCode("SDG"), CurrencyType: CurrencyTypeFiat, Divisibility: 2},
		"SEK": {Name: "Swedish Krona", Code: CurrencyCode("SEK"), CurrencyType: CurrencyTypeFiat, Divisibility: 2},
		"SGD": {Name: "Singapore Dollar", Code: CurrencyCode("SGD"), CurrencyType: CurrencyTypeFiat, Divisibility: 2},
		"SHP": {Name: "Saint Helena Pound", Code: CurrencyCode("SHP"), CurrencyType: CurrencyTypeFiat, Divisibility: 2},
		"SLL": {Name: "Leone", Code: CurrencyCode("SLL"), CurrencyType: CurrencyTypeFiat, Divisibility: 2},
		"SOS": {Name: "Somali Shilling", Code: CurrencyCode("SOS"), CurrencyType: CurrencyTypeFiat, Divisibility: 2},
		"SRD": {Name: "Surinam Dollar", Code: CurrencyCode("SRD"), CurrencyType: CurrencyTypeFiat, Divisibility: 2},
		"SSP": {Name: "South Sudanese Pound", Code: CurrencyCode("SSP"), CurrencyType: CurrencyTypeFiat, Divisibility: 2},
		"STD": {Name: "Dobra", Code: CurrencyCode("STD"), CurrencyType: CurrencyTypeFiat, Divisibility: 2},
		"SVC": {Name: "El Salvador Colon", Code: CurrencyCode("SVC"), CurrencyType: CurrencyTypeFiat, Divisibility: 2},
		"SYP": {Name: "Syrian Pound", Code: CurrencyCode("SYP"), CurrencyType: CurrencyTypeFiat, Divisibility: 2},
		"SZL": {Name: "Lilangeni", Code: CurrencyCode("SZL"), CurrencyType: CurrencyTypeFiat, Divisibility: 2},
		"THB": {Name: "Baht", Code: CurrencyCode("THB"), CurrencyType: CurrencyTypeFiat, Divisibility: 2},
		"TJS": {Name: "Somoni", Code: CurrencyCode("TJS"), CurrencyType: CurrencyTypeFiat, Divisibility: 2},
		"TMT": {Name: "Turkmenistan New Manat", Code: CurrencyCode("TMT"), CurrencyType: CurrencyTypeFiat, Divisibility: 2},
		"TND": {Name: "Tunisian Dinar", Code: CurrencyCode("TND"), CurrencyType: CurrencyTypeFiat, Divisibility: 2},
		"TOP": {Name: "Paanga", Code: CurrencyCode("TOP"), CurrencyType: CurrencyTypeFiat, Divisibility: 2},
		"TRY": {Name: "Turkish Lira", Code: CurrencyCode("TRY"), CurrencyType: CurrencyTypeFiat, Divisibility: 2},
		"TTD": {Name: "Trinidad and Tobago Dollar", Code: CurrencyCode("TTD"), CurrencyType: CurrencyTypeFiat, Divisibility: 2},
		"TWD": {Name: "New Taiwan Dollar", Code: CurrencyCode("TWD"), CurrencyType: CurrencyTypeFiat, Divisibility: 2},
		"TZS": {Name: "Tanzanian Shilling", Code: CurrencyCode("TZS"), CurrencyType: CurrencyTypeFiat, Divisibility: 2},
		"UAH": {Name: "Hryvnia", Code: CurrencyCode("UAH"), CurrencyType: CurrencyTypeFiat, Divisibility: 2},
		"UGX": {Name: "Uganda Shilling", Code: CurrencyCode("UGX"), CurrencyType: CurrencyTypeFiat, Divisibility: 2},
		"USD": {Name: "United States Dollar", Code: CurrencyCode("USD"), CurrencyType: CurrencyTypeFiat, Divisibility: 2},
		"UYU": {Name: "Peso Uruguayo", Code: CurrencyCode("UYU"), CurrencyType: CurrencyTypeFiat, Divisibility: 2},
		"UZS": {Name: "Uzbekistan Sum", Code: CurrencyCode("UZS"), CurrencyType: CurrencyTypeFiat, Divisibility: 2},
		"VEF": {Name: "Bolivar", Code: CurrencyCode("VEF"), CurrencyType: CurrencyTypeFiat, Divisibility: 2},
		"VND": {Name: "Dong", Code: CurrencyCode("VND"), CurrencyType: CurrencyTypeFiat, Divisibility: 2},
		"VUV": {Name: "Vatu", Code: CurrencyCode("VUV"), CurrencyType: CurrencyTypeFiat, Divisibility: 2},
		"WST": {Name: "Tala", Code: CurrencyCode("WST"), CurrencyType: CurrencyTypeFiat, Divisibility: 2},
		"XAF": {Name: "CFA Franc BEAC", Code: CurrencyCode("XAF"), CurrencyType: CurrencyTypeFiat, Divisibility: 2},
		"XCD": {Name: "East Caribbean Dollar", Code: CurrencyCode("XCD"), CurrencyType: CurrencyTypeFiat, Divisibility: 2},
		"XOF": {Name: "CFA Franc BCEAO", Code: CurrencyCode("XOF"), CurrencyType: CurrencyTypeFiat, Divisibility: 2},
		"XPF": {Name: "CFP Franc", Code: CurrencyCode("XPF"), CurrencyType: CurrencyTypeFiat, Divisibility: 2},
		"XSU": {Name: "Sucre", Code: CurrencyCode("XSU"), CurrencyType: CurrencyTypeFiat, Divisibility: 2},
		"YER": {Name: "Yemeni Rial", Code: CurrencyCode("YER"), CurrencyType: CurrencyTypeFiat, Divisibility: 2},
		"ZAR": {Name: "Rand", Code: CurrencyCode("ZAR"), CurrencyType: CurrencyTypeFiat, Divisibility: 2},
		"ZMW": {Name: "Zambian Kwacha", Code: CurrencyCode("ZMW"), CurrencyType: CurrencyTypeFiat, Divisibility: 2},
		"ZWL": {Name: "Zimbabwe Dollar", Code: CurrencyCode("ZWL"), CurrencyType: CurrencyTypeFiat, Divisibility: 2},
	}
)

// CurrencyDictionary is a map that can be used to look up a currency
// given its string code.
type CurrencyDictionary map[string]*Currency

// LookupCurrencyDefinition returns the CurrencyDefinition out of the loaded dictionary.
// Lookup normalizes the code before lookup and recommends using CurrencyDefinition.Code
// from the response as a normalized code.
func (c CurrencyDictionary) Lookup(code string) (*Currency, error) {
	var (
		upcase    = strings.ToUpper(code)
		isTestnet = strings.HasPrefix(upcase, "T")

		def *Currency
		ok  bool
	)
	if isTestnet {
		def, ok = c[strings.TrimPrefix(upcase, "T")]
	} else {
		def, ok = c[upcase]
	}
	if !ok {
		return nil, ErrCurrencyDefinitionUndefined
	}
	if isTestnet {
		return convertToTestnet(def), nil
	}
	return def, nil
}

// convertToTestnet converts a Currency object to its
// testnet counterpart.
func convertToTestnet(def *Currency) *Currency {
	return &Currency{
		Name:         def.Name,
		Code:         CurrencyCode(fmt.Sprintf("T%s", def.Code)),
		Divisibility: def.Divisibility,
		CurrencyType: def.CurrencyType,
	}
}

// CurrencyType represents a type of current. Currently either
// crypto or fiat.
type CurrencyType string

const (
	// CurrencyTypeCrypto represents a cryptocurrency
	CurrencyTypeCrypto CurrencyType = "crypto"

	// CurrencyTypeFiat represents a fiat currency.
	CurrencyTypeFiat CurrencyType = "fiat"
)

// String returns a readable representation of CurrencyType
func (c CurrencyType) String() string {
	return string(c)
}

// CurrencyCode is a string-based currency symbol
type CurrencyCode string

// String returns a readable representation of CurrencyCode
func (c CurrencyCode) String() string {
	return strings.ToUpper(string(c))
}

// Currency defines the characteristics of a currency
type Currency struct {
	Name          string        `json:"name"`
	Code          CurrencyCode  `json:"code"`
	Divisibility  uint          `json:"divisibility"`
	CurrencyType  CurrencyType  `json:"currencyType"`
	Bip44Code     uint          `json:"-"`
	BlockInterval time.Duration `json:"blockInterval"`
}

// String returns a readable representation of CurrencyDefinition
func (c *Currency) String() string {
	if c == nil {
		log.Errorf("returning nil CurrencyCode, please report this bug")
		debug.PrintStack()
		return "nil"
	}
	return c.Code.String()
}

// CurrencyCode returns the CurrencyCode of the definition
func (c *Currency) CurrencyCode() *CurrencyCode {
	return &c.Code
}

// Equal indicates if the receiver and other have the same code
// and divisibility
func (c *Currency) Equal(other *Currency) bool {
	if c == nil || other == nil {
		return false
	}
	code := strings.TrimPrefix(c.Code.String(), "T")
	otherCode := strings.TrimPrefix(other.Code.String(), "T")
	if code != otherCode {
		return false
	}
	if c.Divisibility != other.Divisibility {
		return false
	}
	if c.CurrencyType != other.CurrencyType {
		return false
	}
	return true
}

// CurrencyValue represents the amount and variety of currency
type CurrencyValue struct {
	Amount   iwallet.Amount
	Currency *Currency
}

func (cv *CurrencyValue) MarshalJSON() ([]byte, error) {
	type currencyJSON struct {
		Amount   string   `json:"amount"`
		Currency Currency `json:"currency"`
	}

	c0 := currencyJSON{
		Amount:   "0",
		Currency: Currency{},
	}

	c0.Amount = cv.Amount.String()

	if cv.Currency != nil {
		c0.Currency = Currency{
			Code:         cv.Currency.Code,
			Divisibility: cv.Currency.Divisibility,
			Name:         cv.Currency.Name,
			CurrencyType: cv.Currency.CurrencyType,
		}
	}

	return json.Marshal(c0)
}

func (cv *CurrencyValue) UnmarshalJSON(b []byte) error {
	type currencyJSON struct {
		Amount   string   `json:"amount"`
		Currency Currency `json:"currency"`
	}

	var c0 currencyJSON

	err := json.Unmarshal(b, &c0)
	if err == nil {
		cv.Amount = iwallet.NewAmount(c0.Amount)
		cv.Currency = &c0.Currency
	}

	return err
}

// NewCurrencyValueFromInt is a convenience function which converts an int64
// into a string and passes the arguments to NewCurrencyValue
func NewCurrencyValueFromInt(amount int64, currency *Currency) *CurrencyValue {
	return &CurrencyValue{iwallet.NewAmount(amount), currency}
}

// NewCurrencyValueFromUint is a convenience function which converts an int64
// into a string and passes the arguments to NewCurrencyValue
func NewCurrencyValueFromUint(amount uint64, currency *Currency) *CurrencyValue {
	return &CurrencyValue{iwallet.NewAmount(amount), currency}
}

// NewCurrencyValue accepts string amounts and currency codes, and creates
// a valid CurrencyValue
func NewCurrencyValue(amount string, currency *Currency) *CurrencyValue {
	return &CurrencyValue{Amount: iwallet.NewAmount(amount), Currency: currency}
}

// AmountInt64 returns a valid int64 or an error
func (cv *CurrencyValue) AmountInt64() (int64, error) {
	if !cv.Amount.IsInt64() {
		return 0, ErrCurrencyValueInsufficientPrecision
	}
	return cv.Amount.Int64(), nil
}

// AmountUint64 returns a valid int64 or an error
func (cv *CurrencyValue) AmountUint64() (uint64, error) {
	if !cv.Amount.IsUint64() {
		return 0, ErrCurrencyValueInsufficientPrecision
	}
	return cv.Amount.Uint64(), nil
}

// String returns a string representation of a CurrencyValue
func (cv *CurrencyValue) String() string {
	return fmt.Sprintf("%s %s", cv.Amount.String(), cv.Currency.String())
}

// Equal indicates if the amount and variety of currency is equivalent
func (cv *CurrencyValue) Equal(other *CurrencyValue) bool {
	if cv == nil || other == nil {
		return false
	}
	if !cv.Currency.Equal(other.Currency) {
		return false
	}
	return cv.Amount.Cmp(other.Amount) == 0
}

// ConvertTo will perform the following math:
// v.Amount * exchangeRate * (final.Currency.Divisibility/v.Currency.Divisibility)
// where v is the receiver, exchangeRate is the ratio of (1 final.Currency/v.Currency)
// v and final must both be Valid() and exchangeRate must not be zero.
func (cv *CurrencyValue) ConvertTo(final *Currency, exchangeRate float64) (*CurrencyValue, error) {
	if final == nil || cv.Currency == nil {
		return nil, fmt.Errorf("cannot convert invalid value")
	}
	if exchangeRate <= 0 {
		return nil, ErrCurrencyValueNegativeRate
	}

	var (
		j                = new(big.Float)
		currencyRate     = new(big.Float)
		divisibilityRate = new(big.Float)

		divRateFloat = math.Pow10(int(final.Divisibility)) / math.Pow10(int(cv.Currency.Divisibility))
	)

	currencyRate.SetFloat64(exchangeRate)
	divisibilityRate.SetFloat64(divRateFloat)

	x := big.Int(cv.Amount)
	j.SetInt(&x)
	j.Mul(j, currencyRate)
	j.Mul(j, divisibilityRate)
	result, _ := j.Int(nil)
	return &CurrencyValue{Amount: iwallet.NewAmount(result), Currency: final}, nil
}
