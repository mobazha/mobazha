package utils

import (
	btcec "github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/btcutil/hdkeychain"
	"github.com/btcsuite/btcd/chaincfg"
)

// GenerateRatingPublicKeys uses the chaincode from the order to deterministically generate public keys
// from our rating public key. This allows us to recover the private key to sign the rating with later on
// as long as we know the chaincode.
func GenerateRatingPublicKeys(ratingPubKey *btcec.PublicKey, numKeys int, chaincode []byte) ([][]byte, error) {
	hdKey := hdkeychain.NewExtendedKey(
		chaincfg.MainNetParams.HDPublicKeyID[:],
		ratingPubKey.SerializeCompressed(),
		chaincode,
		[]byte{0x00, 0x00, 0x00, 0x00},
		0,
		0,
		false)

	keyBytes := make([][]byte, 0, numKeys)
	iKeys, err := generateHdKeys(hdKey, numKeys, false)
	if err != nil {
		return nil, err
	}
	for _, key := range iKeys {
		keyBytes = append(keyBytes, key.([]byte))
	}
	return keyBytes, nil
}

// GenerateRatingPrivateKeys does the same thing as it's public key counterpart except it
// returns the private keys.
func GenerateRatingPrivateKeys(ratingPrivKey *btcec.PrivateKey, numKeys int, chaincode []byte) ([]*btcec.PrivateKey, error) {
	hdKey := hdkeychain.NewExtendedKey(
		chaincfg.MainNetParams.HDPrivateKeyID[:],
		ratingPrivKey.Serialize(),
		chaincode,
		[]byte{0x00, 0x00, 0x00, 0x00},
		0,
		0,
		true)

	keys := make([]*btcec.PrivateKey, 0, numKeys)
	iKeys, err := generateHdKeys(hdKey, numKeys, true)
	if err != nil {
		return nil, err
	}
	for _, key := range iKeys {
		keys = append(keys, key.(*btcec.PrivateKey))
	}
	return keys, nil
}

// GenerateEscrowPublicKey uses the chaincode from the order to deterministically generate public keys
// from our escrow public key. This allows us to recover the private key to sign the transaction with later
// on as long as we know the chaincode.
func GenerateEscrowPublicKey(escrowPubKey *btcec.PublicKey, chaincode []byte) (*btcec.PublicKey, error) {
	hdKey := hdkeychain.NewExtendedKey(
		chaincfg.MainNetParams.HDPublicKeyID[:],
		escrowPubKey.SerializeCompressed(),
		chaincode,
		[]byte{0x00, 0x00, 0x00, 0x00},
		0,
		0,
		false)

	key, err := generateChild(hdKey)
	if err != nil {
		return nil, err
	}
	return key.ECPubKey()
}

// GenerateEscrowPrivateKey does the same thing as it's public key counterpart except it
// returns the private keys.
func GenerateEscrowPrivateKey(escrowPrivKey *btcec.PrivateKey, chaincode []byte) (*btcec.PrivateKey, error) {
	hdKey := hdkeychain.NewExtendedKey(
		chaincfg.MainNetParams.HDPrivateKeyID[:],
		escrowPrivKey.Serialize(),
		chaincode,
		[]byte{0x00, 0x00, 0x00, 0x00},
		0,
		0,
		true)

	key, err := generateChild(hdKey)
	if err != nil {
		return nil, err
	}
	return key.ECPrivKey()
}

// generateHdKey returns a single child key from the provided hd key.
func generateChild(hdKey *hdkeychain.ExtendedKey) (*hdkeychain.ExtendedKey, error) {
	i := 0
	for {
		childKey, err := hdKey.Derive(uint32(i))
		if err != nil {
			// Small chance this can fail due to weird curve stuff.
			// Bip32 spec calls for skipping to next key.
			i++
			continue
		}
		return childKey, nil
	}
}

// generateHdKeys is a helper function that can generate from either public or private
// keys.
func generateHdKeys(hdKey *hdkeychain.ExtendedKey, numKeys int, priv bool) ([]interface{}, error) {
	keys := make([]interface{}, 0, numKeys)
	i := 0
	for len(keys) < numKeys {
		childKey, err := hdKey.Derive(uint32(i))
		if err != nil {
			// Small chance this can fail due to weird curve stuff.
			// Bip32 spec calls for skipping to next key.
			i++
			continue
		}
		if priv {
			priv, err := childKey.ECPrivKey()
			if err != nil {
				return nil, err
			}
			keys = append(keys, priv)
		} else {
			pub, err := childKey.ECPubKey()
			if err != nil {
				return nil, err
			}
			keys = append(keys, pub.SerializeCompressed())
		}

		i++
	}
	return keys, nil
}

func GenerateEthPrivateKey(bip44Masterkey *hdkeychain.ExtendedKey) (*btcec.PrivateKey, error) {
	coinTypeKey, err := bip44Masterkey.Derive(hdkeychain.HardenedKeyStart + 60)
	if err != nil {
		return nil, err
	}

	accountKey, err := coinTypeKey.Derive(hdkeychain.HardenedKeyStart + 0)
	if err != nil {
		return nil, err
	}

	changeKey, err := accountKey.Derive(0)
	if err != nil {
		return nil, err
	}
	addressKey, err := changeKey.Derive(0)
	if err != nil {
		return nil, err
	}

	return addressKey.ECPrivKey()
}

// GenerateTRONPrivateKey derives a TRON master key using BIP-44 path m/44'/195'/0'/0/0.
// Coin type 195 is the registered BIP-44 coin type for TRON.
// Independent HD path ensures key isolation from EVM keys and prevents cross-chain replay.
func GenerateTRONPrivateKey(bip44Masterkey *hdkeychain.ExtendedKey) (*btcec.PrivateKey, error) {
	coinTypeKey, err := bip44Masterkey.Derive(hdkeychain.HardenedKeyStart + 195)
	if err != nil {
		return nil, err
	}

	accountKey, err := coinTypeKey.Derive(hdkeychain.HardenedKeyStart + 0)
	if err != nil {
		return nil, err
	}

	changeKey, err := accountKey.Derive(0)
	if err != nil {
		return nil, err
	}
	addressKey, err := changeKey.Derive(0)
	if err != nil {
		return nil, err
	}

	return addressKey.ECPrivKey()
}
