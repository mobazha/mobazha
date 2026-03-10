package repo

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"

	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/mobazha/mobazha-core/identity"

	crypto "github.com/libp2p/go-libp2p/core/crypto"
)

// PrivKeyAndPeerIDFromKey parses marshaled identity key bytes and returns
// the libp2p private key and derived peer ID.
func PrivKeyAndPeerIDFromKey(privkeyBytes []byte) (crypto.PrivKey, peer.ID, error) {
	keyPair, err := identity.KeyPairFromMarshaledPrivateKey(privkeyBytes)
	if err != nil {
		return nil, "", err
	}

	peerIDStr, err := identity.PeerIDFromPublicKey(keyPair.PubKey)
	if err != nil {
		return nil, "", err
	}

	pid, err := peer.Decode(string(peerIDStr))
	if err != nil {
		return nil, "", err
	}

	return keyPair.PrivKey, pid, nil
}

// IdentityKeyFromSeed derives an identity key from a BIP39 seed.
// Uses HMAC-SHA256 with "OpenBazaar seed" as the key for deterministic derivation,
// then generates an Ed25519 key pair via libp2p.
func IdentityKeyFromSeed(seed []byte, bits int) ([]byte, error) {
	// Derive deterministic seed using HMAC-SHA256 (Mobazha-specific derivation)
	hm := hmac.New(sha256.New, []byte("OpenBazaar seed"))
	hm.Write(seed)
	reader := bytes.NewReader(hm.Sum(nil))

	// Generate Ed25519 key pair from the derived seed
	sk, _, err := crypto.GenerateKeyPairWithReader(crypto.Ed25519, bits, reader)
	if err != nil {
		return nil, err
	}
	encodedKey, err := crypto.MarshalPrivateKey(sk)
	if err != nil {
		return nil, err
	}
	return encodedKey, nil
}
