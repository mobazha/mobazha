package assetid

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/btcsuite/btcd/btcutil/base58"
	"github.com/ethereum/go-ethereum/common"
	"github.com/gagliardetto/solana-go"
)

type Namespace string

const (
	NamespaceEIP155      Namespace = "eip155"
	NamespaceBIP122      Namespace = "bip122"
	NamespaceTRON        Namespace = "tron"
	NamespaceSolana      Namespace = "solana"
	NamespaceBitcoinCash Namespace = "bitcoincash"
	NamespaceZCash       Namespace = "zcash"
	NamespaceExternalPayment      Namespace = "external_payment"
)

type Standard string

const (
	StandardNative Standard = "native"
	StandardERC20  Standard = "erc20"
	StandardTRC20  Standard = "trc20"
	StandardSPL    Standard = "spl"
)

type ID struct {
	Namespace Namespace
	ChainRef  string
	Standard  Standard
	AssetRef  string // Empty for native assets.
}

func (id ID) IsNative() bool {
	return id.Standard == StandardNative
}

func (id ID) String() string {
	if id.IsNative() {
		return fmt.Sprintf("crypto:%s:%s:native", id.Namespace, id.ChainRef)
	}
	return fmt.Sprintf("crypto:%s:%s:%s:%s", id.Namespace, id.ChainRef, id.Standard, id.AssetRef)
}

func Parse(raw string) (ID, error) {
	normalized, err := Normalize(raw)
	if err != nil {
		return ID{}, err
	}

	parts := strings.Split(normalized, ":")
	ns := Namespace(parts[1])
	chainRef := parts[2]
	if len(parts) == 4 {
		return ID{
			Namespace: ns,
			ChainRef:  chainRef,
			Standard:  StandardNative,
		}, nil
	}

	return ID{
		Namespace: ns,
		ChainRef:  chainRef,
		Standard:  Standard(parts[3]),
		AssetRef:  parts[4],
	}, nil
}

// IsCanonical reports whether raw is already in canonical form.
func IsCanonical(raw string) bool {
	normalized, err := Normalize(raw)
	if err != nil {
		return false
	}
	return strings.TrimSpace(raw) == normalized
}

var bip122ChainRefRE = regexp.MustCompile("^[0-9a-f]{32}$")

var (
	solanaChainRefs = map[string]struct{}{"mainnet": {}, "devnet": {}, "testnet": {}}
	tronChainRefs   = map[string]struct{}{"mainnet": {}, "shasta": {}, "nile": {}}
	bchChainRefs    = map[string]struct{}{"mainnet": {}, "testnet": {}}
	zecChainRefs    = map[string]struct{}{"mainnet": {}, "testnet": {}}
	external_paymentChainRefs = map[string]struct{}{"mainnet": {}, "stagenet": {}, "testnet": {}}
)

func validateChainRefWhitelist(ns Namespace, chainRef string, allowed map[string]struct{}) (string, error) {
	normalized := strings.ToLower(chainRef)
	if _, ok := allowed[normalized]; !ok {
		return "", newError(ErrCodeInvalidChainRef, fmt.Sprintf("unsupported %s chain_ref %q", ns, chainRef))
	}
	return normalized, nil
}

func Normalize(raw string) (string, error) {
	input := strings.TrimSpace(raw)
	if input == "" {
		return "", newError(ErrCodeEmpty, "asset id is empty")
	}

	parts := strings.Split(input, ":")
	if len(parts) != 4 && len(parts) != 5 {
		return "", newError(ErrCodeInvalidSegmentCount, fmt.Sprintf("got %d segments", len(parts)))
	}

	if !strings.EqualFold(parts[0], "crypto") {
		return "", newError(ErrCodeInvalidPrefix, fmt.Sprintf("prefix %q", parts[0]))
	}

	ns := Namespace(strings.ToLower(parts[1]))
	chainRefRaw := parts[2]
	chainRef, err := normalizeChainRef(ns, chainRefRaw)
	if err != nil {
		return "", err
	}

	if len(parts) == 4 {
		if !strings.EqualFold(parts[3], string(StandardNative)) {
			return "", newError(ErrCodeInvalidNativeFormat, "native form must end with :native")
		}
		if err := validateStandardForNamespace(ns, StandardNative); err != nil {
			return "", err
		}
		return fmt.Sprintf("crypto:%s:%s:native", ns, chainRef), nil
	}

	standard := Standard(strings.ToLower(parts[3]))
	assetRefRaw := parts[4]
	if assetRefRaw == "" {
		return "", newError(ErrCodeInvalidAssetRef, "asset_ref is empty")
	}
	if standard == StandardNative {
		return "", newError(ErrCodeInvalidNativeFormat, "native assets must use 4-segment form")
	}

	if err := validateStandardForNamespace(ns, standard); err != nil {
		return "", err
	}

	assetRef, err := normalizeAssetRef(ns, standard, assetRefRaw)
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("crypto:%s:%s:%s:%s", ns, chainRef, standard, assetRef), nil
}

func normalizeChainRef(ns Namespace, chainRef string) (string, error) {
	if chainRef == "" {
		return "", newError(ErrCodeInvalidChainRef, "chain_ref is empty")
	}

	switch ns {
	case NamespaceEIP155:
		v, err := strconv.ParseUint(chainRef, 10, 64)
		if err != nil || v == 0 {
			return "", newError(ErrCodeInvalidChainRef, fmt.Sprintf("invalid eip155 chain_ref %q", chainRef))
		}
		return strconv.FormatUint(v, 10), nil
	case NamespaceBIP122:
		n := strings.ToLower(chainRef)
		if !bip122ChainRefRE.MatchString(n) {
			return "", newError(ErrCodeInvalidChainRef, fmt.Sprintf("invalid bip122 chain_ref %q", chainRef))
		}
		return n, nil
	case NamespaceSolana:
		return validateChainRefWhitelist(ns, chainRef, solanaChainRefs)
	case NamespaceTRON:
		return validateChainRefWhitelist(ns, chainRef, tronChainRefs)
	case NamespaceBitcoinCash:
		return validateChainRefWhitelist(ns, chainRef, bchChainRefs)
	case NamespaceZCash:
		return validateChainRefWhitelist(ns, chainRef, zecChainRefs)
	case NamespaceExternalPayment:
		return validateChainRefWhitelist(ns, chainRef, external_paymentChainRefs)
	default:
		return "", newError(ErrCodeInvalidNamespace, fmt.Sprintf("unsupported namespace %q", ns))
	}
}

func validateStandardForNamespace(ns Namespace, standard Standard) error {
	switch ns {
	case NamespaceEIP155:
		if standard != StandardNative && standard != StandardERC20 {
			return newError(ErrCodeInvalidStandard, fmt.Sprintf("namespace %s only supports native/erc20", ns))
		}
		return nil
	case NamespaceBIP122:
		if standard != StandardNative {
			return newError(ErrCodeInvalidStandard, fmt.Sprintf("namespace %s only supports native", ns))
		}
		return nil
	case NamespaceTRON:
		if standard != StandardNative && standard != StandardTRC20 {
			return newError(ErrCodeInvalidStandard, fmt.Sprintf("namespace %s only supports native/trc20", ns))
		}
		return nil
	case NamespaceSolana:
		if standard != StandardNative && standard != StandardSPL {
			return newError(ErrCodeInvalidStandard, fmt.Sprintf("namespace %s only supports native/spl", ns))
		}
		return nil
	case NamespaceBitcoinCash, NamespaceZCash, NamespaceExternalPayment:
		if standard != StandardNative {
			return newError(ErrCodeInvalidStandard, fmt.Sprintf("namespace %s only supports native", ns))
		}
		return nil
	default:
		return newError(ErrCodeInvalidNamespace, fmt.Sprintf("unsupported namespace %q", ns))
	}
}

func normalizeAssetRef(ns Namespace, standard Standard, assetRef string) (string, error) {
	switch ns {
	case NamespaceEIP155:
		if standard != StandardERC20 {
			return "", newError(ErrCodeInvalidStandard, fmt.Sprintf("namespace %s token standard must be erc20", ns))
		}
		if !common.IsHexAddress(assetRef) {
			return "", newError(ErrCodeInvalidEVMAddress, fmt.Sprintf("invalid eip155 address %q", assetRef))
		}
		return common.HexToAddress(assetRef).Hex(), nil

	case NamespaceTRON:
		if standard != StandardTRC20 {
			return "", newError(ErrCodeInvalidStandard, fmt.Sprintf("namespace %s token standard must be trc20", ns))
		}
		canonical, err := normalizeTRONAddress(assetRef)
		if err != nil {
			return "", newError(ErrCodeInvalidTRONAddress, fmt.Sprintf("invalid tron address %q", assetRef))
		}
		return canonical, nil

	case NamespaceSolana:
		if standard != StandardSPL {
			return "", newError(ErrCodeInvalidStandard, fmt.Sprintf("namespace %s token standard must be spl", ns))
		}
		pk, err := solana.PublicKeyFromBase58(assetRef)
		if err != nil {
			return "", newError(ErrCodeInvalidSolanaMint, fmt.Sprintf("invalid solana mint %q", assetRef))
		}
		return pk.String(), nil

	case NamespaceBIP122, NamespaceBitcoinCash, NamespaceZCash, NamespaceExternalPayment:
		return "", newError(ErrCodeInvalidStandard, fmt.Sprintf("namespace %s does not support token asset_ref", ns))

	default:
		return "", newError(ErrCodeInvalidNamespace, fmt.Sprintf("unsupported namespace %q", ns))
	}
}

func normalizeTRONAddress(address string) (string, error) {
	decoded := base58.Decode(address)
	if len(decoded) != 25 {
		return "", fmt.Errorf("invalid base58 length")
	}

	payload := decoded[:21]
	checksum := decoded[21:]
	expected := tronChecksum(payload)
	if !bytes.Equal(checksum, expected) {
		return "", fmt.Errorf("checksum mismatch")
	}

	// 0x41 is TRON mainnet address prefix.
	if payload[0] != 0x41 {
		return "", fmt.Errorf("invalid tron prefix")
	}

	return base58.Encode(append(payload, expected...)), nil
}

func tronChecksum(payload []byte) []byte {
	h1 := sha256.Sum256(payload)
	h2 := sha256.Sum256(h1[:])
	return h2[:4]
}
