package utils

import (
	"encoding/hex"
	"testing"

	btcec "github.com/btcsuite/btcd/btcec/v2"
)

const (
	privKeyHex   = "c560021782de34f597ef1b2bd415d20c7febe7f111e6c1da349990323e082c74"
	chaincodeHex = "7b3e0d9be17127f289f9ef52076f89ccc9c4cccb35c234bc8038372b015e68b4"
)

func TestGenerateEscrowPrivateKey(t *testing.T) {
	privKeyBytes, err := hex.DecodeString(privKeyHex)
	if err != nil {
		t.Fatal(err)
	}

	priv, _ := btcec.PrivKeyFromBytes(privKeyBytes)

	chaincode, err := hex.DecodeString(chaincodeHex)
	if err != nil {
		t.Fatal(err)
	}

	key, err := GenerateEscrowPrivateKey(priv, chaincode)
	if err != nil {
		t.Fatal(err)
	}

	expected := "6dd4cfd740ea853a94e61364a95ac17c5c2b246d638d88c8f3d71e323029d567"
	if hex.EncodeToString(key.Serialize()) != expected {
		t.Errorf("Generated invalid key. Expected %s, got %s", expected, hex.EncodeToString(key.Serialize()))
	}
}

func TestGenerateEscrowPublicKey(t *testing.T) {
	privKeyBytes, err := hex.DecodeString(privKeyHex)
	if err != nil {
		t.Fatal(err)
	}

	_, pub := btcec.PrivKeyFromBytes(privKeyBytes)

	chaincode, err := hex.DecodeString(chaincodeHex)
	if err != nil {
		t.Fatal(err)
	}

	key, err := GenerateEscrowPublicKey(pub, chaincode)
	if err != nil {
		t.Fatal(err)
	}

	expected := "0325af7d595399660af29434e183934a298f691ce29514b19de71f9529619939bb"
	if hex.EncodeToString(key.SerializeCompressed()) != expected {
		t.Errorf("Generated invalid key. Expected %s, got %s", expected, hex.EncodeToString(key.SerializeCompressed()))
	}
}

func TestGenerateRatingPrivateKeys(t *testing.T) {
	privKeyBytes, err := hex.DecodeString(privKeyHex)
	if err != nil {
		t.Fatal(err)
	}

	priv, _ := btcec.PrivKeyFromBytes(privKeyBytes)

	chaincode, err := hex.DecodeString(chaincodeHex)
	if err != nil {
		t.Fatal(err)
	}

	keys, err := GenerateRatingPrivateKeys(priv, 3, chaincode)
	if err != nil {
		t.Fatal(err)
	}

	if len(keys) != 3 {
		t.Fatalf("Expected 3 keys got %d", len(keys))
	}

	expectedKey0 := "6dd4cfd740ea853a94e61364a95ac17c5c2b246d638d88c8f3d71e323029d567"
	if hex.EncodeToString(keys[0].Serialize()) != expectedKey0 {
		t.Errorf("Key 0 incorrect. Expected %s got %s", expectedKey0, hex.EncodeToString(keys[0].Serialize()))
	}

	expectedKey1 := "e603c4db531ee2e9a940c16c88eb855aeb124c33dc96ceff876160028c62448f"
	if hex.EncodeToString(keys[1].Serialize()) != expectedKey1 {
		t.Errorf("Key 1 incorrect. Expected %s got %s", expectedKey1, hex.EncodeToString(keys[1].Serialize()))
	}

	expectedKey2 := "5b794d95d5ebaca15c89e5549c94417e9393ecbf10376443299adbae2eace511"
	if hex.EncodeToString(keys[2].Serialize()) != expectedKey2 {
		t.Errorf("Key 2 incorrect. Expected %s got %s", expectedKey2, hex.EncodeToString(keys[2].Serialize()))
	}
}

func TestGenerateRatingPublicKeys(t *testing.T) {
	privKeyBytes, err := hex.DecodeString(privKeyHex)
	if err != nil {
		t.Fatal(err)
	}

	_, pub := btcec.PrivKeyFromBytes(privKeyBytes)

	chaincode, err := hex.DecodeString(chaincodeHex)
	if err != nil {
		t.Fatal(err)
	}

	keys, err := GenerateRatingPublicKeys(pub, 3, chaincode)
	if err != nil {
		t.Fatal(err)
	}

	if len(keys) != 3 {
		t.Fatalf("Expected 3 keys got %d", len(keys))
	}

	expectedKey0 := "0325af7d595399660af29434e183934a298f691ce29514b19de71f9529619939bb"
	if hex.EncodeToString(keys[0]) != expectedKey0 {
		t.Errorf("Key 0 incorrect. Expected %s got %s", expectedKey0, hex.EncodeToString(keys[0]))
	}

	expectedKey1 := "03e4059762b4b89b63f72b3165ffebd75046419143105c5e336eddcf7a23990670"
	if hex.EncodeToString(keys[1]) != expectedKey1 {
		t.Errorf("Key 1 incorrect. Expected %s got %s", expectedKey1, hex.EncodeToString(keys[1]))
	}

	expectedKey2 := "03ad8aa0c01928338e144181eeb50da08240aa0d07195f602d655e88ee192ca46a"
	if hex.EncodeToString(keys[2]) != expectedKey2 {
		t.Errorf("Key 2 incorrect. Expected %s got %s", expectedKey2, hex.EncodeToString(keys[2]))
	}
}
