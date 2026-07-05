package wallet

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/mobazha/mobazha/pkg/models"
	iwallet "github.com/mobazha/mobazha/pkg/wallet-interface"
)

func makeRemoteRateJSON(rates map[string]string) []byte {
	return makeRemoteRateJSONAt(rates, time.Now().UTC())
}

func makeRemoteRateJSONAt(rates map[string]string, updatedAt time.Time) []byte {
	resp := map[string]interface{}{
		"data": map[string]interface{}{
			"rates":     rates,
			"reserve":   "USD",
			"updatedAt": updatedAt.UTC().Format(time.RFC3339),
		},
	}
	b, _ := json.Marshal(resp)
	return b
}

func TestRemoteProvider_RejectsExpiredSourceSnapshot(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(makeRemoteRateJSONAt(map[string]string{
			"USD": "100",
			"ETH": "500000000000000",
		}, time.Now().Add(-remoteRateMaxSourceAge-time.Minute)))
	}))
	defer srv.Close()

	rp := NewRemoteProvider(srv.URL, srv.Client(), 30*time.Second)
	_, err := rp.fetchRates("USD")
	if err == nil {
		t.Fatal("expected an expired source snapshot to fail closed")
	}
}

func TestRemoteProvider_RejectsExpiredStaleCache(t *testing.T) {
	rp := NewRemoteProvider("http://127.0.0.1:1", &http.Client{Timeout: 10 * time.Millisecond}, time.Millisecond)
	rp.cached = map[models.CurrencyCode]iwallet.Amount{
		"USD": iwallet.NewAmount(100),
	}
	rp.cachedReserve = "USD"
	rp.cachedUpdated = time.Now().Add(-remoteRateMaxStaleAge - time.Minute)
	rp.lastFetch = time.Now().Add(-time.Minute)

	_, err := rp.fetchRates("USD")
	if err == nil {
		t.Fatal("expected an expired stale cache to fail closed")
	}
}

func TestRemoteProvider_FetchRates_Success(t *testing.T) {
	sourceUpdated := time.Now().UTC().Add(-time.Minute).Truncate(time.Second)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/platform/v1/exchange-rates" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(makeRemoteRateJSONAt(map[string]string{
			"BTC": "6500000000000",
			"ETH": "350000000000",
			"LTC": "8700000000",
			"USD": "100000000",
		}, sourceUpdated))
	}))
	defer srv.Close()

	rp := NewRemoteProvider(srv.URL, srv.Client(), 30*time.Second)
	rates, err := rp.fetchRates("USD")
	if err != nil {
		t.Fatalf("fetchRates failed: %v", err)
	}

	if len(rates) != 4 {
		t.Fatalf("expected 4 rates, got %d", len(rates))
	}

	btcRate := rates[models.CurrencyCode("BTC")]
	if btcRate.String() != "6500000000000" {
		t.Errorf("BTC rate = %s, want 6500000000000", btcRate.String())
	}
	if !rp.cachedUpdated.Equal(sourceUpdated) {
		t.Errorf("cached source timestamp = %s, want %s", rp.cachedUpdated, sourceUpdated)
	}
}

func TestRemoteProvider_RebasesReserveSnapshotForRequestedBase(t *testing.T) {
	var callCount int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&callCount, 1)
		w.Header().Set("Content-Type", "application/json")
		w.Write(makeRemoteRateJSON(map[string]string{
			"USD": "100",
			"ETH": "500000000000000",
		}))
	}))
	defer srv.Close()

	rp := NewRemoteProvider(srv.URL, srv.Client(), 30*time.Second)
	usdRates, err := rp.fetchRates("USD")
	if err != nil {
		t.Fatalf("fetch USD rates: %v", err)
	}
	if got := usdRates[models.CurrencyCode("ETH")].String(); got != "500000000000000" {
		t.Fatalf("USD/ETH rate = %s, want 500000000000000", got)
	}

	ethRates, err := rp.fetchRates("ETH")
	if err != nil {
		t.Fatalf("fetch ETH rates: %v", err)
	}
	if got := ethRates[models.CurrencyCode("USD")].String(); got != "200000" {
		t.Fatalf("ETH/USD rate = %s, want 200000", got)
	}
	if got := ethRates[models.CurrencyCode("ETH")].String(); got != "1000000000000000000" {
		t.Fatalf("ETH/ETH rate = %s, want 1000000000000000000", got)
	}
	if got := atomic.LoadInt32(&callCount); got != 1 {
		t.Fatalf("remote calls = %d, want one shared reserve snapshot", got)
	}
}

func TestRemoteProvider_Cache_HitAndExpiry(t *testing.T) {
	var callCount int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&callCount, 1)
		w.Header().Set("Content-Type", "application/json")
		w.Write(makeRemoteRateJSON(map[string]string{
			"BTC": "6500000000000",
		}))
	}))
	defer srv.Close()

	rp := NewRemoteProvider(srv.URL, srv.Client(), 100*time.Millisecond)

	// First call fetches from remote
	_, err := rp.fetchRates("USD")
	if err != nil {
		t.Fatalf("first fetch failed: %v", err)
	}
	if atomic.LoadInt32(&callCount) != 1 {
		t.Fatalf("expected 1 server call, got %d", callCount)
	}

	// Second call within TTL should use cache
	_, err = rp.fetchRates("USD")
	if err != nil {
		t.Fatalf("second fetch failed: %v", err)
	}
	if atomic.LoadInt32(&callCount) != 1 {
		t.Fatalf("expected 1 server call (cached), got %d", callCount)
	}

	// Wait for TTL to expire
	time.Sleep(150 * time.Millisecond)

	// Third call after TTL should re-fetch
	_, err = rp.fetchRates("USD")
	if err != nil {
		t.Fatalf("third fetch failed: %v", err)
	}
	if atomic.LoadInt32(&callCount) != 2 {
		t.Fatalf("expected 2 server calls after TTL expiry, got %d", callCount)
	}
}

func TestRemoteProvider_StaleFallback(t *testing.T) {
	var callCount int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt32(&callCount, 1)
		if n == 1 {
			w.Header().Set("Content-Type", "application/json")
			w.Write(makeRemoteRateJSON(map[string]string{
				"BTC": "6500000000000",
			}))
			return
		}
		http.Error(w, "service unavailable", http.StatusServiceUnavailable)
	}))
	defer srv.Close()

	rp := NewRemoteProvider(srv.URL, srv.Client(), 50*time.Millisecond)

	// Populate cache
	rates, err := rp.fetchRates("USD")
	if err != nil {
		t.Fatalf("initial fetch failed: %v", err)
	}
	if len(rates) == 0 {
		t.Fatal("expected non-empty rates from initial fetch")
	}

	// Wait for cache to expire
	time.Sleep(100 * time.Millisecond)

	// Server now returns 503, but stale cache should be used
	staleRates, err := rp.fetchRates("USD")
	if err != nil {
		t.Fatalf("stale fallback should not error: %v", err)
	}
	if staleRates[models.CurrencyCode("BTC")].String() != "6500000000000" {
		t.Error("stale cache should return previous BTC rate")
	}
}

func TestRemoteProvider_NoCache_ErrorPropagated(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "service unavailable", http.StatusServiceUnavailable)
	}))
	defer srv.Close()

	rp := NewRemoteProvider(srv.URL, srv.Client(), 30*time.Second)

	_, err := rp.fetchRates("USD")
	if err == nil {
		t.Fatal("expected error when server is down and no cache")
	}
}

func TestRemoteProvider_EmptyRates_Error(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(makeRemoteRateJSON(map[string]string{}))
	}))
	defer srv.Close()

	rp := NewRemoteProvider(srv.URL, srv.Client(), 30*time.Second)
	_, err := rp.fetchRates("USD")
	if err == nil {
		t.Fatal("expected error for empty rates response")
	}
}

func TestRemoteProvider_InvalidJSON_Error(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{invalid json`))
	}))
	defer srv.Close()

	rp := NewRemoteProvider(srv.URL, srv.Client(), 30*time.Second)
	_, err := rp.fetchRates("USD")
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}
