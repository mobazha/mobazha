package utxoaddress

import (
	"bytes"
	"strings"

	"github.com/btcsuite/btcd/btcutil/bech32"
)

// SameUTXOAddress compares UTXO addresses while tolerating URI/network prefixes
// such as "bitcoincash:" that different chain sources may include or omit.
func SameUTXOAddress(a, b string) bool {
	a = normalizeForCompare(a)
	b = normalizeForCompare(b)
	if a == "" || b == "" {
		return false
	}
	if strings.EqualFold(a, b) {
		return true
	}
	return sameBitcoinWitnessProgram(a, b)
}

func sameBitcoinWitnessProgram(a, b string) bool {
	aHRP, aData, aVersion, aErr := bech32.DecodeGeneric(a)
	bHRP, bData, bVersion, bErr := bech32.DecodeGeneric(b)
	if aErr != nil || bErr != nil || !bitcoinWitnessHRP(aHRP) || !bitcoinWitnessHRP(bHRP) {
		return false
	}
	if !sameBitcoinWitnessNetwork(aHRP, bHRP) {
		return false
	}
	return aVersion == bVersion && bytes.Equal(aData, bData)
}

func sameBitcoinWitnessNetwork(aHRP, bHRP string) bool {
	aHRP = strings.ToLower(strings.TrimSpace(aHRP))
	bHRP = strings.ToLower(strings.TrimSpace(bHRP))
	if aHRP == bHRP {
		return true
	}
	return (aHRP == "tb" && bHRP == "bcrt") || (aHRP == "bcrt" && bHRP == "tb")
}

func bitcoinWitnessHRP(hrp string) bool {
	switch strings.ToLower(strings.TrimSpace(hrp)) {
	case "bc", "tb", "bcrt":
		return true
	default:
		return false
	}
}

func normalizeForCompare(address string) string {
	address = strings.TrimSpace(address)
	if i := strings.LastIndex(address, ":"); i >= 0 {
		address = address[i+1:]
	}
	return address
}
