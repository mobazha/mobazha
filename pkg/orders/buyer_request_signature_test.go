package orders

import (
	"encoding/base64"
	"testing"

	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The published test vector from the index side. Ed25519 signatures are
// deterministic, so a byte-for-byte match here is what proves this node and the
// hosted index agree on what a signature covers — not merely that each of them
// can verify its own work.
//
// The private key is a public test key that belongs to no node.
const (
	hostedTestPrivateKeyBase64 = "CAESQAEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBiojj3XQJ8ZX9UtstPLpdcspnCb8dlBIb83SIAbQPb1w="
	hostedTestPeerID           = "12D3KooWK99VoVxNE7XzyBwXEzW7xhK7Gpv85r9F3V3fyKSUKPH5"
	hostedTestPublishSignature = "LOmbRB+75iAKY7FkcJWjVJij30+jv8G1G5eoaBP6hV/Go5kdjDAneJXe9g5VLRGwWRX0C1T6ERnhnJmjw3PrAg=="
	hostedTestWithdrawRevision = 2
	hostedTestWithdrawSig      = "4d3bgP/Rg5IOKHwtkSzv5eyHme1R2tQcTurrrdca2bf2b4YNLmhQfyI5rr6dlA9MVRtd9ozsOhzoqKDx63iUDA=="
)

// hostedTestRecord is the exact byte sequence the index signed: compact JSON,
// with revision and quantity written as strings.
const hostedTestRecord = `{"requestID":"card-projection-check-354c1a57","revision":"1","peerID":"12D3KooWK99VoVxNE7XzyBwXEzW7xhK7Gpv85r9F3V3fyKSUKPH5","title":"Card projection check","quantity":"2","budget":{"currency":{"code":"BTC","divisibility":8},"amount":"150000"},"acceptedCurrencies":["BTC","LTC"],"shipTo":"NL","productType":"video-projectors","status":"open","createdAt":"2026-10-11T05:19:01.164Z","expiresAt":"2026-10-18T05:19:01.155Z"}`

func hostedTestKey(t *testing.T) crypto.PrivKey {
	t.Helper()
	raw, err := base64.StdEncoding.DecodeString(hostedTestPrivateKeyBase64)
	require.NoError(t, err)
	key, err := crypto.UnmarshalPrivateKey(raw)
	require.NoError(t, err)
	return key
}

func TestBuyerRequestSignatureMatchesTheHostedIndexVector(t *testing.T) {
	key := hostedTestKey(t)
	serialized := []byte(hostedTestRecord)
	require.Equal(t, 420, len(serialized), "the vector's record is 420 bytes")

	record, err := ParseBuyerRequest(serialized)
	require.NoError(t, err)
	assert.Equal(t, hostedTestPeerID, record.GetPeerID())
	assert.Equal(t, uint32(1), record.GetRevision(), "a revision may arrive as a string")
	assert.Equal(t, uint32(2), record.GetQuantity(), "a quantity may arrive as a string")

	payload, err := BuyerRequestSignaturePayload(record, serialized)
	require.NoError(t, err)
	assert.Equal(t, "mobazha:buyer-request:v1\n52:"+hostedTestPeerID+"\n420:"+hostedTestRecord,
		string(payload), "the publication payload is the vector's, byte for byte")

	signature, err := key.Sign(payload)
	require.NoError(t, err)
	assert.Equal(t, hostedTestPublishSignature, base64.StdEncoding.EncodeToString(signature))

	// The record the vector carries is one this node would have signed itself,
	// so the rules on both sides agree too.
	assert.NoError(t, ValidateBuyerRequest(record, record.GetCreatedAt().AsTime()))
	assert.NoError(t, VerifyBuyerRequestSignature(hostedTestPeerID, payload, signature))
}

func TestBuyerRequestWithdrawSignatureMatchesTheHostedIndexVector(t *testing.T) {
	key := hostedTestKey(t)

	payload := BuyerRequestWithdrawPayload(hostedTestPeerID, "card-projection-check-354c1a57",
		hostedTestWithdrawRevision)
	assert.Equal(t, "mobazha:buyer-request-withdraw:v1\n52:"+hostedTestPeerID+
		"\n30:card-projection-check-354c1a57\n1:2\n", string(payload),
		"the withdrawal payload is the vector's, byte for byte")

	signature, err := key.Sign(payload)
	require.NoError(t, err)
	assert.Equal(t, hostedTestWithdrawSig, base64.StdEncoding.EncodeToString(signature))
}
