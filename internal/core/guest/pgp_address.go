package guest

import (
	"crypto/sha1" // #nosec G505 -- OpenPGP v4 fingerprints are defined as SHA-1.
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"io"
	"strings"

	"golang.org/x/crypto/openpgp/armor"
	"golang.org/x/crypto/openpgp/packet"
)

func addressPublicKeyFingerprint(publicKeyArmor string) (string, error) {
	key, err := parseAddressPublicKey(publicKeyArmor)
	if err != nil {
		return "", err
	}
	return key.primaryFingerprint, nil
}

type addressPublicKey struct {
	primaryFingerprint string
	recipientKeyIDs    map[uint64]struct{}
}

func parseAddressPublicKey(publicKeyArmor string) (addressPublicKey, error) {
	block, err := armor.Decode(strings.NewReader(strings.TrimSpace(publicKeyArmor)))
	if err != nil || block == nil || block.Type != "PGP PUBLIC KEY BLOCK" {
		return addressPublicKey{}, fmt.Errorf("seller address encryption key is invalid")
	}

	parsed := addressPublicKey{recipientKeyIDs: make(map[uint64]struct{})}
	packets := packet.NewOpaqueReader(block.Body)
	for {
		next, err := packets.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return addressPublicKey{}, fmt.Errorf("seller address encryption key is invalid")
		}
		if next.Tag != 6 && next.Tag != 14 { // Public-Key and Public-Subkey packets.
			continue
		}
		fingerprint, keyID, err := fingerprintAddressKeyPacket(next.Contents)
		if err != nil {
			return addressPublicKey{}, err
		}
		parsed.recipientKeyIDs[keyID] = struct{}{}
		if next.Tag == 6 && parsed.primaryFingerprint == "" {
			parsed.primaryFingerprint = strings.ToUpper(hex.EncodeToString(fingerprint))
		}
	}
	if parsed.primaryFingerprint == "" || len(parsed.recipientKeyIDs) == 0 {
		return addressPublicKey{}, fmt.Errorf("seller address encryption key is invalid")
	}
	return parsed, nil
}

func fingerprintAddressKeyPacket(contents []byte) ([]byte, uint64, error) {
	if len(contents) < 6 {
		return nil, 0, fmt.Errorf("seller address encryption key is invalid")
	}

	var framed []byte
	switch contents[0] {
	case 4:
		if len(contents) > 0xffff {
			return nil, 0, fmt.Errorf("seller address encryption key is invalid")
		}
		framed = make([]byte, 3+len(contents))
		framed[0] = 0x99
		binary.BigEndian.PutUint16(framed[1:3], uint16(len(contents)))
		copy(framed[3:], contents)
		sum := sha1.Sum(framed) // #nosec G401 -- required by the OpenPGP v4 format.
		fingerprint := sum[:]
		return fingerprint, binary.BigEndian.Uint64(fingerprint[len(fingerprint)-8:]), nil
	case 5, 6:
		framed = make([]byte, 5+len(contents))
		if contents[0] == 5 {
			framed[0] = 0x9a
		} else {
			framed[0] = 0x9b
		}
		binary.BigEndian.PutUint32(framed[1:5], uint32(len(contents)))
		copy(framed[5:], contents)
		sum := sha256.Sum256(framed)
		fingerprint := sum[:]
		if contents[0] == 6 {
			return fingerprint, binary.BigEndian.Uint64(fingerprint[:8]), nil
		}
		return fingerprint, binary.BigEndian.Uint64(fingerprint[len(fingerprint)-8:]), nil
	default:
		return nil, 0, fmt.Errorf("seller address encryption key version is unsupported")
	}
}

// validateAddressCiphertext verifies that the request contains a structurally
// valid OpenPGP message encrypted to the currently configured seller key. It
// intentionally does not decrypt the message on the server.
func validateAddressCiphertext(ciphertext, publicKeyArmor string) error {
	publicKey, err := parseAddressPublicKey(publicKeyArmor)
	if err != nil {
		return err
	}

	block, err := armor.Decode(strings.NewReader(strings.TrimSpace(ciphertext)))
	if err != nil || block == nil || block.Type != "PGP MESSAGE" {
		return fmt.Errorf("shipping address must be a valid armored OpenPGP message")
	}

	packets := packet.NewOpaqueReader(block.Body)
	recipientMatched := false
	for {
		next, err := packets.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("shipping address OpenPGP message is malformed: %w", err)
		}
		if next.Tag == 1 && len(next.Contents) >= 10 { // Public-Key Encrypted Session Key.
			keyID := binary.BigEndian.Uint64(next.Contents[1:9])
			if _, matches := publicKey.recipientKeyIDs[keyID]; matches {
				recipientMatched = true
			}
			continue
		}
		if (next.Tag == 9 || next.Tag == 18 || next.Tag == 20) && recipientMatched {
			return nil
		}
	}

	if recipientMatched {
		return fmt.Errorf("shipping address OpenPGP message has no encrypted payload")
	}
	return fmt.Errorf("shipping address was not encrypted to the active seller key")
}
