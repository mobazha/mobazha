package net

import (
	"crypto/rand"
	"errors"
	"fmt"

	"github.com/libp2p/go-libp2p/core/crypto"
	"google.golang.org/protobuf/proto"

	extra "github.com/teserakt-io/golang-ed25519/extra25519"

	"golang.org/x/crypto/nacl/box"
)

const (
	// Length of nacl nonce
	NonceBytes = 24

	// Length of nacl ephemeral public key
	EphemeralPublicKeyBytes = 32
)

// ErrBoxDecryption Nacl box decryption failed
var ErrBoxDecryption = errors.New("failed to decrypt curve25519")

// Encrypt encrypt a message with the public key. Currently only ed25519
// is supported.
func Encrypt(pubKey crypto.PubKey, message proto.Message) ([]byte, error) {
	ser, err := proto.Marshal(message)
	if err != nil {
		return nil, err
	}
	ed25519Pubkey, ok := pubKey.(*crypto.Ed25519PublicKey)
	if ok {
		return encryptCurve25519(ed25519Pubkey, ser)
	}
	return nil, errors.New("could not determine key type")
}

func encryptCurve25519(pubKey *crypto.Ed25519PublicKey, plaintext []byte) ([]byte, error) {
	// Generated ephemeral key pair
	ephemPub, ephemPriv, err := box.GenerateKey(rand.Reader)
	if err != nil {
		return nil, err
	}
	// Convert recipient's key into curve25519
	rawBytes, err := pubKey.Raw()
	if err != nil {
		return nil, err
	}
	var raw [32]byte
	copy(raw[:], rawBytes)
	pk, err := pubkeyToCurve25519(raw)
	if err != nil {
		return nil, err
	}

	// Encrypt with nacl
	var (
		ciphertext []byte
		nonce      [24]byte
		n          = make([]byte, 24)
	)
	_, err = rand.Read(n)
	if err != nil {
		return nil, err
	}
	copy(nonce[:], n)
	ciphertext = box.Seal(ciphertext, plaintext, &nonce, pk, ephemPriv)

	// Prepend the ephemeral public key
	ciphertext = append(ephemPub[:], ciphertext...)

	// Prepend nonce
	ciphertext = append(nonce[:], ciphertext...)
	return ciphertext, nil
}

// Decrypt a message using a private key. Currently only ed5519 is supported.
func Decrypt(privKey crypto.PrivKey, ciphertext []byte, message proto.Message) error {
	ed25519Privkey, ok := privKey.(*crypto.Ed25519PrivateKey)
	if ok {
		plaintext, err := decryptCurve25519(ed25519Privkey, ciphertext)
		if err != nil {
			return err
		}

		return proto.Unmarshal(plaintext, message)
	}
	return errors.New("could not determine key type")
}

func decryptCurve25519(privKey *crypto.Ed25519PrivateKey, ciphertext []byte) ([]byte, error) {
	rawBytes, err := privKey.Raw()
	if err != nil {
		return nil, err
	}
	var raw [64]byte
	copy(raw[:], rawBytes)
	curve25519Privkey := privkeyToCurve25519(raw)
	var plaintext []byte

	n := ciphertext[:NonceBytes]
	ephemPubkeyBytes := ciphertext[NonceBytes : NonceBytes+EphemeralPublicKeyBytes]
	ct := ciphertext[NonceBytes+EphemeralPublicKeyBytes:]

	var ephemPubkey [32]byte
	copy(ephemPubkey[:], ephemPubkeyBytes)

	var nonce [24]byte
	copy(nonce[:], n)

	plaintext, success := box.Open(plaintext, ct, &nonce, &ephemPubkey, curve25519Privkey)
	if !success {
		return nil, ErrBoxDecryption
	}
	return plaintext, nil
}

func privkeyToCurve25519(sk [64]byte) *[32]byte {
	var skNew [32]byte
	extra.PrivateKeyToCurve25519(&skNew, &sk)
	return &skNew
}

func pubkeyToCurve25519(pk [32]byte) (*[32]byte, error) {
	var pkNew [32]byte
	success := extra.PublicKeyToCurve25519(&pkNew, &pk)
	if !success {
		return nil, fmt.Errorf("error converting ed25519 pubkey to curve25519 pubkey")
	}
	return &pkNew, nil
}
