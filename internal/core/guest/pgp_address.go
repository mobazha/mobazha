package guest

import (
	"fmt"
	"io"
	"strings"

	"golang.org/x/crypto/openpgp"
	"golang.org/x/crypto/openpgp/armor"
	"golang.org/x/crypto/openpgp/packet"
)

// validateAddressCiphertext verifies that the request contains a structurally
// valid OpenPGP message encrypted to the currently configured seller key. It
// intentionally does not decrypt the message on the server.
func validateAddressCiphertext(ciphertext, publicKeyArmor string) error {
	entities, err := openpgp.ReadArmoredKeyRing(strings.NewReader(publicKeyArmor))
	if err != nil || len(entities) != 1 || entities[0].PrimaryKey == nil {
		return fmt.Errorf("seller address encryption key is invalid")
	}

	recipientKeyIDs := map[uint64]struct{}{
		entities[0].PrimaryKey.KeyId: {},
	}
	for _, subkey := range entities[0].Subkeys {
		if subkey.PublicKey != nil {
			recipientKeyIDs[subkey.PublicKey.KeyId] = struct{}{}
		}
	}

	block, err := armor.Decode(strings.NewReader(strings.TrimSpace(ciphertext)))
	if err != nil || block == nil || block.Type != "PGP MESSAGE" {
		return fmt.Errorf("shipping address must be a valid armored OpenPGP message")
	}

	packets := packet.NewReader(block.Body)
	recipientMatched := false
	for {
		next, err := packets.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("shipping address OpenPGP message is malformed: %w", err)
		}
		if encryptedKey, ok := next.(*packet.EncryptedKey); ok {
			if _, matches := recipientKeyIDs[encryptedKey.KeyId]; matches {
				recipientMatched = true
			}
			continue
		}
		if _, ok := next.(*packet.SymmetricallyEncrypted); ok && recipientMatched {
			return nil
		}
	}

	if recipientMatched {
		return fmt.Errorf("shipping address OpenPGP message has no encrypted payload")
	}
	return fmt.Errorf("shipping address was not encrypted to the active seller key")
}
