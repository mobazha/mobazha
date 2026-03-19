package tron

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestServer(handler http.HandlerFunc) *httptest.Server {
	return httptest.NewServer(handler)
}

func newTestClient(endpoints []string) *TronClient {
	return NewTronClient(endpoints, RetryConfig{
		MaxRetries:     2,
		InitialBackoff: 10 * time.Millisecond,
		MaxBackoff:     50 * time.Millisecond,
	})
}

func TestGetTransaction_Success(t *testing.T) {
	srv := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/wallet/gettransactionbyid", r.URL.Path)
		w.Write([]byte(`{"txID":"abc123","ret":[{"contractRet":"SUCCESS"}]}`))
	})
	defer srv.Close()

	client := newTestClient([]string{srv.URL})
	tx, err := client.GetTransaction(context.Background(), "abc123")
	require.NoError(t, err)
	assert.Equal(t, "abc123", tx.TxID)
	assert.Equal(t, "SUCCESS", tx.Ret[0].ContractRet)
}

func TestGetTransaction_NotFound(t *testing.T) {
	srv := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(TronTransaction{})
	})
	defer srv.Close()

	client := newTestClient([]string{srv.URL})
	_, err := client.GetTransaction(context.Background(), "missing")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestGetTransactionInfo_Success(t *testing.T) {
	srv := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/wallet/gettransactioninfobyid", r.URL.Path)
		json.NewEncoder(w).Encode(TronTransactionInfo{
			ID:          "abc123",
			BlockNumber: 12345,
			Receipt:     TronReceipt{Result: "SUCCESS"},
			Log: []TronEventLog{{
				Address: "41a614f803b6fd780986a42c78ec9c7f77e6ded13c",
				Topics:  []string{"topic0", "topic1"},
				Data:    "data",
			}},
		})
	})
	defer srv.Close()

	client := newTestClient([]string{srv.URL})
	info, err := client.GetTransactionInfo(context.Background(), "abc123")
	require.NoError(t, err)
	assert.Equal(t, "abc123", info.ID)
	assert.Equal(t, int64(12345), info.BlockNumber)
	assert.Equal(t, "SUCCESS", info.Receipt.Result)
	assert.Len(t, info.Log, 1)
}

func TestGetNowBlock_Success(t *testing.T) {
	srv := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/wallet/getnowblock", r.URL.Path)
		w.Write([]byte(`{"block_header":{"raw_data":{"number":99999,"timestamp":1700000000000}}}`))
	})
	defer srv.Close()

	client := newTestClient([]string{srv.URL})
	block, err := client.GetNowBlock(context.Background())
	require.NoError(t, err)
	assert.Equal(t, int64(99999), block.BlockHeader.RawData.Number)
}

func TestRetryOnServerError(t *testing.T) {
	var calls int32
	srv := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt32(&calls, 1)
		if n <= 2 {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte("internal error"))
			return
		}
		json.NewEncoder(w).Encode(TronTransaction{TxID: "retried"})
	})
	defer srv.Close()

	client := newTestClient([]string{srv.URL})
	tx, err := client.GetTransaction(context.Background(), "retried")
	require.NoError(t, err)
	assert.Equal(t, "retried", tx.TxID)
	assert.Equal(t, int32(3), atomic.LoadInt32(&calls))
}

func TestEndpointFailover(t *testing.T) {
	srv1 := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
		w.Write([]byte("down"))
	})
	defer srv1.Close()

	srv2 := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(TronTransaction{TxID: "from-srv2"})
	})
	defer srv2.Close()

	client := newTestClient([]string{srv1.URL, srv2.URL})
	tx, err := client.GetTransaction(context.Background(), "test")
	require.NoError(t, err)
	assert.Equal(t, "from-srv2", tx.TxID)
}

func TestAllEndpointsFail(t *testing.T) {
	srv := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
		w.Write([]byte("down"))
	})
	defer srv.Close()

	client := newTestClient([]string{srv.URL})
	_, err := client.GetTransaction(context.Background(), "test")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "all retries exhausted")
}

func TestContextCancellation(t *testing.T) {
	srv := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(200 * time.Millisecond)
		json.NewEncoder(w).Encode(TronTransaction{TxID: "slow"})
	})
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	client := newTestClient([]string{srv.URL})
	_, err := client.GetTransaction(ctx, "test")
	assert.Error(t, err)
}

func TestBroadcastTransaction_Success(t *testing.T) {
	srv := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/wallet/broadcasttransaction", r.URL.Path)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"result": true,
			"txid":   "broadcast-tx-id",
		})
	})
	defer srv.Close()

	client := newTestClient([]string{srv.URL})
	tx := json.RawMessage(`{"txID":"test"}`)
	err := client.BroadcastTransaction(context.Background(), tx)
	require.NoError(t, err)
}

func TestBroadcastTransaction_Failure(t *testing.T) {
	srv := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"result":  false,
			"code":    "SIGERROR",
			"message": "signature error",
		})
	})
	defer srv.Close()

	client := newTestClient([]string{srv.URL})
	tx := json.RawMessage(`{"txID":"test"}`)
	err := client.BroadcastTransaction(context.Background(), tx)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "SIGERROR")
}

func TestTriggerConstantContract(t *testing.T) {
	srv := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/wallet/triggerconstantcontract", r.URL.Path)
		json.NewEncoder(w).Encode(ConstantResult{
			Result: struct {
				Result  bool   `json:"result"`
				Message string `json:"message,omitempty"`
			}{Result: true},
			ConstantResult: []string{"0000000000000000000000000000000000000000000000000000000000000001"},
		})
	})
	defer srv.Close()

	client := newTestClient([]string{srv.URL})
	result, err := client.TriggerConstantContract(
		context.Background(),
		"41a614f803b6fd780986a42c78ec9c7f77e6ded13c",
		"41b614f803b6fd780986a42c78ec9c7f77e6ded13c",
		"transactions(bytes32)",
		"0000000000000000000000000000000000000000000000000000000000000001",
	)
	require.NoError(t, err)
	assert.True(t, result.Result.Result)
	assert.Len(t, result.ConstantResult, 1)
}
