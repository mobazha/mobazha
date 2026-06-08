package utxoaddress

import "strings"

// SameUTXOAddress compares UTXO addresses while tolerating URI/network prefixes
// such as "bitcoincash:" that different chain sources may include or omit.
func SameUTXOAddress(a, b string) bool {
	a = normalizeForCompare(a)
	b = normalizeForCompare(b)
	return a != "" && strings.EqualFold(a, b)
}

func normalizeForCompare(address string) string {
	address = strings.TrimSpace(address)
	if i := strings.LastIndex(address, ":"); i >= 0 {
		address = address[i+1:]
	}
	return address
}
