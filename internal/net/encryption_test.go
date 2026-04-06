package net

import (
	"testing"

	crypto "github.com/libp2p/go-libp2p/core/crypto"
	pb "github.com/mobazha/mobazha3.0/pkg/net/mbzpb"
)

func TestEncryptCurve25519(t *testing.T) {
	priv, pub, err := crypto.GenerateKeyPair(crypto.Ed25519, 0)
	if err != nil {
		t.Fatal(err)
	}

	plaintext := "Hello World!!!"
	ciphertext, err := Encrypt(pub, &pb.AckMessage{AckedMessageID: plaintext})
	if err != nil {
		t.Fatal(err)
	}
	decrypted := new(pb.AckMessage)
	err = Decrypt(priv, ciphertext, decrypted)
	if err != nil {
		t.Fatal(err)
	}
	if decrypted.AckedMessageID != plaintext {
		t.Errorf("Expected plaintext of %s, got %s", plaintext, decrypted.AckedMessageID)
	}
}
