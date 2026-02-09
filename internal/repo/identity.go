package repo

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"

	"github.com/mobazha/mobazha-core/identity"

	config "github.com/ipfs/kubo/config"
	crypto "github.com/libp2p/go-libp2p/core/crypto"
)

// IdentityFromKey creates an IPFS config.Identity from marshaled private key bytes.
// Uses mobazha-core for PeerID derivation, ensuring consistency across node and cloud.
func IdentityFromKey(privkey []byte) (config.Identity, error) {
	ident := config.Identity{}

	// Use mobazha-core to reconstruct the key pair and derive PeerID
	keyPair, err := identity.KeyPairFromMarshaledPrivateKey(privkey)
	if err != nil {
		return ident, err
	}

	// Marshal for IPFS config (base64-encoded libp2p format)
	marshaledKey, err := keyPair.MarshalPrivateKey()
	if err != nil {
		return ident, err
	}
	ident.PrivKey = base64.StdEncoding.EncodeToString(marshaledKey)

	// Derive PeerID via mobazha-core (same libp2p derivation under the hood)
	peerID, err := identity.PeerIDFromPublicKey(keyPair.PubKey)
	if err != nil {
		return ident, err
	}
	ident.PeerID = peerID.String()

	return ident, nil
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
