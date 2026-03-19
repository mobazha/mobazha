package tron

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/mobazha/mobazha3.0/pkg/payment"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func mockTronServer(txInfo *TronTransactionInfo, blockNumber int64) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/wallet/gettransactioninfobyid":
			json.NewEncoder(w).Encode(txInfo)
		case "/wallet/getnowblock":
			resp := fmt.Sprintf(`{"block_header":{"raw_data":{"number":%d,"timestamp":1700000000000}}}`, blockNumber)
			w.Write([]byte(resp))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
}

func testEscrowHash() [32]byte {
	var h [32]byte
	b, _ := hex.DecodeString("aabbccddaabbccddaabbccddaabbccddaabbccddaabbccddaabbccddaabbccdd")
	copy(h[:], b)
	return h
}

func padAmount(amount int64) string {
	b := big.NewInt(amount).Bytes()
	padded := make([]byte, 32)
	copy(padded[32-len(b):], b)
	return hex.EncodeToString(padded)
}

func TestVerifyDeposit_Success(t *testing.T) {
	escrowHash := testEscrowHash()
	contractAddr := "41a614f803b6fd780986a42c78ec9c7f77e6ded13c"

	info := &TronTransactionInfo{
		ID:          "txhash123",
		BlockNumber: 1000,
		Receipt:     TronReceipt{Result: "SUCCESS"},
		Log: []TronEventLog{{
			Address: contractAddr,
			Topics: []string{
				fundedEventTopic,
				hex.EncodeToString(escrowHash[:]),
				"0000000000000000000000001234567890abcdef1234567890abcdef12345678",
			},
			Data: padAmount(1000000),
		}},
	}

	srv := mockTronServer(info, 1020)
	defer srv.Close()

	client := newTestClient([]string{srv.URL})
	err := VerifyDeposit(context.Background(), client, DepositVerification{
		TxHash:           "txhash123",
		EscrowHash:       escrowHash,
		ExpectedContract: contractAddr,
		MinAmount:        big.NewInt(1000000),
	})
	require.NoError(t, err)
}

func TestVerifyDeposit_Reverted(t *testing.T) {
	info := &TronTransactionInfo{
		ID:      "txhash123",
		Receipt: TronReceipt{Result: "OUT_OF_ENERGY"},
	}

	srv := mockTronServer(info, 1020)
	defer srv.Close()

	client := newTestClient([]string{srv.URL})
	err := VerifyDeposit(context.Background(), client, DepositVerification{
		TxHash:           "txhash123",
		EscrowHash:       testEscrowHash(),
		ExpectedContract: "41a614f803b6fd780986a42c78ec9c7f77e6ded13c",
	})
	assert.ErrorIs(t, err, payment.ErrDepositReverted)
}

func TestVerifyDeposit_EventNotFound(t *testing.T) {
	info := &TronTransactionInfo{
		ID:          "txhash123",
		BlockNumber: 1000,
		Receipt:     TronReceipt{Result: "SUCCESS"},
		Log:         []TronEventLog{},
	}

	srv := mockTronServer(info, 1020)
	defer srv.Close()

	client := newTestClient([]string{srv.URL})
	err := VerifyDeposit(context.Background(), client, DepositVerification{
		TxHash:           "txhash123",
		EscrowHash:       testEscrowHash(),
		ExpectedContract: "41a614f803b6fd780986a42c78ec9c7f77e6ded13c",
	})
	assert.ErrorIs(t, err, payment.ErrDepositEventNotFound)
}

func TestVerifyDeposit_InsufficientConfirmations(t *testing.T) {
	escrowHash := testEscrowHash()
	contractAddr := "41a614f803b6fd780986a42c78ec9c7f77e6ded13c"

	info := &TronTransactionInfo{
		ID:          "txhash123",
		BlockNumber: 1000,
		Receipt:     TronReceipt{Result: "SUCCESS"},
		Log: []TronEventLog{{
			Address: contractAddr,
			Topics: []string{
				fundedEventTopic,
				hex.EncodeToString(escrowHash[:]),
			},
			Data: padAmount(1000000),
		}},
	}

	srv := mockTronServer(info, 1010) // only 10 confirmations
	defer srv.Close()

	client := newTestClient([]string{srv.URL})
	err := VerifyDeposit(context.Background(), client, DepositVerification{
		TxHash:           "txhash123",
		EscrowHash:       escrowHash,
		ExpectedContract: contractAddr,
		MinAmount:        big.NewInt(1000000),
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "insufficient confirmations")
}

func TestVerifyDeposit_InsufficientAmount(t *testing.T) {
	escrowHash := testEscrowHash()
	contractAddr := "41a614f803b6fd780986a42c78ec9c7f77e6ded13c"

	info := &TronTransactionInfo{
		ID:          "txhash123",
		BlockNumber: 1000,
		Receipt:     TronReceipt{Result: "SUCCESS"},
		Log: []TronEventLog{{
			Address: contractAddr,
			Topics: []string{
				fundedEventTopic,
				hex.EncodeToString(escrowHash[:]),
			},
			Data: padAmount(500000), // less than required
		}},
	}

	srv := mockTronServer(info, 1020)
	defer srv.Close()

	client := newTestClient([]string{srv.URL})
	err := VerifyDeposit(context.Background(), client, DepositVerification{
		TxHash:           "txhash123",
		EscrowHash:       escrowHash,
		ExpectedContract: contractAddr,
		MinAmount:        big.NewInt(1000000),
	})
	assert.ErrorIs(t, err, payment.ErrDepositEventNotFound)
}

func TestVerifyDeposit_WrongEscrowHash(t *testing.T) {
	contractAddr := "41a614f803b6fd780986a42c78ec9c7f77e6ded13c"

	var wrongHash [32]byte
	copy(wrongHash[:], []byte("wrong-hash-does-not-match!!!!!!"))

	info := &TronTransactionInfo{
		ID:          "txhash123",
		BlockNumber: 1000,
		Receipt:     TronReceipt{Result: "SUCCESS"},
		Log: []TronEventLog{{
			Address: contractAddr,
			Topics: []string{
				fundedEventTopic,
				hex.EncodeToString(wrongHash[:]),
			},
			Data: padAmount(1000000),
		}},
	}

	srv := mockTronServer(info, 1020)
	defer srv.Close()

	client := newTestClient([]string{srv.URL})
	err := VerifyDeposit(context.Background(), client, DepositVerification{
		TxHash:           "txhash123",
		EscrowHash:       testEscrowHash(),
		ExpectedContract: contractAddr,
		MinAmount:        big.NewInt(1000000),
	})
	assert.ErrorIs(t, err, payment.ErrDepositEventNotFound)
}

func TestVerifyDeposit_Failed(t *testing.T) {
	info := &TronTransactionInfo{
		ID:     "txhash123",
		Result: "FAILED",
	}

	srv := mockTronServer(info, 1020)
	defer srv.Close()

	client := newTestClient([]string{srv.URL})
	err := VerifyDeposit(context.Background(), client, DepositVerification{
		TxHash: "txhash123",
	})
	assert.ErrorIs(t, err, payment.ErrDepositReverted)
}
