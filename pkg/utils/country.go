// Package utils provides utility functions for the Mobazha application.
package utils

import "strings"

// ISO 3166-1 alpha-2 country codes
var validISO31661Alpha2 = map[string]bool{
	"AD": true, "AE": true, "AF": true, "AG": true, "AI": true, "AL": true, "AM": true, "AO": true,
	"AQ": true, "AR": true, "AS": true, "AT": true, "AU": true, "AW": true, "AX": true, "AZ": true,
	"BA": true, "BB": true, "BD": true, "BE": true, "BF": true, "BG": true, "BH": true, "BI": true,
	"BJ": true, "BL": true, "BM": true, "BN": true, "BO": true, "BQ": true, "BR": true, "BS": true,
	"BT": true, "BV": true, "BW": true, "BY": true, "BZ": true, "CA": true, "CC": true, "CD": true,
	"CF": true, "CG": true, "CH": true, "CI": true, "CK": true, "CL": true, "CM": true, "CN": true,
	"CO": true, "CR": true, "CU": true, "CV": true, "CW": true, "CX": true, "CY": true, "CZ": true,
	"DE": true, "DJ": true, "DK": true, "DM": true, "DO": true, "DZ": true, "EC": true, "EE": true,
	"EG": true, "EH": true, "ER": true, "ES": true, "ET": true, "FI": true, "FJ": true, "FK": true,
	"FM": true, "FO": true, "FR": true, "GA": true, "GB": true, "GD": true, "GE": true, "GF": true,
	"GG": true, "GH": true, "GI": true, "GL": true, "GM": true, "GN": true, "GP": true, "GQ": true,
	"GR": true, "GS": true, "GT": true, "GU": true, "GW": true, "GY": true, "HK": true, "HM": true,
	"HN": true, "HR": true, "HT": true, "HU": true, "ID": true, "IE": true, "IL": true, "IM": true,
	"IN": true, "IO": true, "IQ": true, "IR": true, "IS": true, "IT": true, "JE": true, "JM": true,
	"JO": true, "JP": true, "KE": true, "KG": true, "KH": true, "KI": true, "KM": true, "KN": true,
	"KP": true, "KR": true, "KW": true, "KY": true, "KZ": true, "LA": true, "LB": true, "LC": true,
	"LI": true, "LK": true, "LR": true, "LS": true, "LT": true, "LU": true, "LV": true, "LY": true,
	"MA": true, "MC": true, "MD": true, "ME": true, "MF": true, "MG": true, "MH": true, "MK": true,
	"ML": true, "MM": true, "MN": true, "MO": true, "MP": true, "MQ": true, "MR": true, "MS": true,
	"MT": true, "MU": true, "MV": true, "MW": true, "MX": true, "MY": true, "MZ": true, "NA": true,
	"NC": true, "NE": true, "NF": true, "NG": true, "NI": true, "NL": true, "NO": true, "NP": true,
	"NR": true, "NU": true, "NZ": true, "OM": true, "PA": true, "PE": true, "PF": true, "PG": true,
	"PH": true, "PK": true, "PL": true, "PM": true, "PN": true, "PR": true, "PS": true, "PT": true,
	"PW": true, "PY": true, "QA": true, "RE": true, "RO": true, "RS": true, "RU": true, "RW": true,
	"SA": true, "SB": true, "SC": true, "SD": true, "SE": true, "SG": true, "SH": true, "SI": true,
	"SJ": true, "SK": true, "SL": true, "SM": true, "SN": true, "SO": true, "SR": true, "SS": true,
	"ST": true, "SV": true, "SX": true, "SY": true, "SZ": true, "TC": true, "TD": true, "TF": true,
	"TG": true, "TH": true, "TJ": true, "TK": true, "TL": true, "TM": true, "TN": true, "TO": true,
	"TR": true, "TT": true, "TV": true, "TW": true, "TZ": true, "UA": true, "UG": true, "UM": true,
	"US": true, "UY": true, "UZ": true, "VA": true, "VC": true, "VE": true, "VG": true, "VI": true,
	"VN": true, "VU": true, "WF": true, "WS": true, "YE": true, "YT": true, "ZA": true, "ZM": true,
	"ZW": true,
}

// Special region codes for shipping
var specialRegionCodes = map[string]bool{
	"ALL":             true, // Worldwide / Global
	"ASIA":            true, // Asia region
	"EUROPE":          true, // Europe region
	"NORTH_AMERICA":   true, // North America region
	"SOUTH_AMERICA":   true, // South America region
	"AFRICA":          true, // Africa region
	"OCEANIA":         true, // Oceania region
	"MIDDLE_EAST":     true, // Middle East region
	"CENTRAL_AMERICA": true, // Central America region
}

// IsValidISOCountryCode checks if the given code is a valid ISO 3166-1 alpha-2 country code.
func IsValidISOCountryCode(code string) bool {
	return validISO31661Alpha2[strings.ToUpper(code)]
}

// IsValidSpecialRegionCode checks if the given code is a valid special region code.
func IsValidSpecialRegionCode(code string) bool {
	return specialRegionCodes[strings.ToUpper(code)]
}

// IsValidShippingRegion checks if the given code is a valid shipping region
// (either an ISO country code or a special region code).
func IsValidShippingRegion(code string) bool {
	upperCode := strings.ToUpper(code)
	return validISO31661Alpha2[upperCode] || specialRegionCodes[upperCode]
}

// NormalizeRegionCode normalizes a region code to uppercase.
func NormalizeRegionCode(code string) string {
	return strings.ToUpper(strings.TrimSpace(code))
}

// ValidateShippingRegions validates a list of shipping regions.
// Returns a list of invalid region codes, or nil if all are valid.
func ValidateShippingRegions(regions []string) []string {
	var invalid []string
	for _, region := range regions {
		if !IsValidShippingRegion(region) {
			invalid = append(invalid, region)
		}
	}
	return invalid
}

// GetAllISOCountryCodes returns all valid ISO 3166-1 alpha-2 country codes.
func GetAllISOCountryCodes() []string {
	codes := make([]string, 0, len(validISO31661Alpha2))
	for code := range validISO31661Alpha2 {
		codes = append(codes, code)
	}
	return codes
}

// GetAllSpecialRegionCodes returns all valid special region codes.
func GetAllSpecialRegionCodes() []string {
	codes := make([]string, 0, len(specialRegionCodes))
	for code := range specialRegionCodes {
		codes = append(codes, code)
	}
	return codes
}
