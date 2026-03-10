package repo

import (
	"bytes"
	"encoding/hex"
	"testing"

	"github.com/tyler-smith/go-bip39"
)

var keyHex = "08011240499228645d120d15b5008b1da0b9dba898df328001ea03c0be84a64c41d205ff1b8339a303cd8cf2945b66c89ac29fa90e79731d67000694284791af404eeb1f"

func TestPrivKeyAndPeerIDFromKey(t *testing.T) {
	keyBytes, err := hex.DecodeString(keyHex)
	if err != nil {
		t.Fatal(err)
	}
	privKey, peerID, err := PrivKeyAndPeerIDFromKey(keyBytes)
	if err != nil {
		t.Fatal(err)
	}
	expected := "12D3KooWBfmETW1ZbkdZbKKPpE3jpjyQ5WBXoDF8y9oE8vMQPKLi"
	if peerID.String() != expected {
		t.Errorf("Incorrect peer ID. Wanted %s, got %s", expected, peerID.String())
	}
	if privKey == nil {
		t.Error("Private key should not be nil")
	}
}

func TestIdentityKeyFromSeed(t *testing.T) {
	seed := bip39.NewSeed("mule track design catch stairs remain produce evidence cannon opera hamster burst", "Secret Passphrase")
	key, err := IdentityKeyFromSeed(seed, 4096)
	if err != nil {
		t.Error(err)
	}
	keyBytes, err := hex.DecodeString(keyHex)
	if err != nil {
		t.Error(err)
	}
	if !bytes.Equal(key, keyBytes) {
		t.Error("Failed to extract correct private key from seed")
	}
}
