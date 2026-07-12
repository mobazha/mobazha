package mempool

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	pkgutxo "github.com/mobazha/mobazha/pkg/utxo"
)

func TestSourceGetTransaction_HTTPNotFoundIsAuthoritative(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.NotFound(w, nil)
	}))
	t.Cleanup(server.Close)

	source := &Source{baseURL: server.URL, httpClient: server.Client()}
	tx, err := source.GetTransaction(context.Background(), "missing")
	if tx != nil {
		t.Fatalf("expected no transaction, got %s", tx.ID)
	}
	if !errors.Is(err, pkgutxo.ErrTransactionNotFound) {
		t.Fatalf("expected ErrTransactionNotFound, got %v", err)
	}
	if !source.IsHealthy() {
		t.Fatal("an authoritative HTTP 404 must not mark the source unhealthy")
	}
}
