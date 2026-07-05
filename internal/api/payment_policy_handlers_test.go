package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/mobazha/mobazha/internal/repo"
	"github.com/mobazha/mobazha/pkg/contracts"
	"github.com/mobazha/mobazha/pkg/core/coreiface"
	"github.com/mobazha/mobazha/pkg/database"
	"github.com/mobazha/mobazha/pkg/models"
)

type paymentPolicyCoreNode struct {
	*mockNode
}

func (n *paymentPolicyCoreNode) PeerHost() host.Host { return nil }

func newPaymentPolicyRouter(t *testing.T, db database.Database) http.Handler {
	return newPaymentPolicyRouterWithNode(t, db, true)
}

func newPaymentPolicyRouterWithNode(t *testing.T, db database.Database, asCoreIface bool) http.Handler {
	t.Helper()
	node := &paymentPolicyCoreNode{
		mockNode: &mockNode{dbFunc: func() database.Database { return db }},
	}
	gateway := &Gateway{config: &GatewayConfig{}}
	outer := chi.NewMux()
	outer.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			var ctx context.Context
			if asCoreIface {
				var ci coreiface.CoreIface = node
				ctx = context.WithValue(req.Context(), nodeContextKey, ci)
			} else {
				ctx = context.WithValue(req.Context(), nodeContextKey, contracts.NodeService(node))
			}
			ctx = WithAuthIdentity(ctx, &AuthIdentity{UserID: "test-admin", IsAdmin: true})
			next.ServeHTTP(w, req.WithContext(ctx))
		})
	})
	outer.Mount("/", mustNewV1Router(t, gateway, false, false))
	return outer
}

func TestGETStorePaymentPolicy_DefaultsToChainConfirmed(t *testing.T) {
	r, err := repo.NewRepo("", filepath.Join(t.TempDir(), "policy-get"), true)
	if err != nil {
		t.Fatal(err)
	}
	defer r.DestroyRepo()

	router := newPaymentPolicyRouter(t, r.DB())
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/settings/payment-policy", nil)
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rr.Code, rr.Body.String())
	}
	var body struct {
		Data struct {
			UtxoConfirmationPolicy string `json:"utxoConfirmationPolicy"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Data.UtxoConfirmationPolicy != models.PaymentConfirmationPolicyChainConfirmed {
		t.Fatalf("policy = %q, want chain_confirmed", body.Data.UtxoConfirmationPolicy)
	}
}

func TestPUTStorePaymentPolicy_PersistsMempoolAccepted(t *testing.T) {
	r, err := repo.NewRepo("", filepath.Join(t.TempDir(), "policy-put"), true)
	if err != nil {
		t.Fatal(err)
	}
	defer r.DestroyRepo()

	router := newPaymentPolicyRouter(t, r.DB())
	payload := `{"utxoConfirmationPolicy":"mempool_accepted"}`
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/v1/settings/payment-policy", bytes.NewReader([]byte(payload)))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("PUT status = %d body=%s", rr.Code, rr.Body.String())
	}

	rr = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/v1/settings/payment-policy", nil)
	router.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("GET status = %d body=%s", rr.Code, rr.Body.String())
	}
	var body struct {
		Data struct {
			UtxoConfirmationPolicy string `json:"utxoConfirmationPolicy"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Data.UtxoConfirmationPolicy != models.PaymentConfirmationPolicyMempoolAccepted {
		t.Fatalf("policy = %q, want mempool_accepted", body.Data.UtxoConfirmationPolicy)
	}
}

func TestGETStorePaymentPolicy_WorksWithNodeServiceOnly(t *testing.T) {
	r, err := repo.NewRepo("", filepath.Join(t.TempDir(), "policy-saas"), true)
	if err != nil {
		t.Fatal(err)
	}
	defer r.DestroyRepo()

	router := newPaymentPolicyRouterWithNode(t, r.DB(), false)
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/settings/payment-policy", nil)
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rr.Code, rr.Body.String())
	}
}

func TestPUTStorePaymentPolicy_RejectsInvalidPolicy(t *testing.T) {
	r, err := repo.NewRepo("", filepath.Join(t.TempDir(), "policy-invalid"), true)
	if err != nil {
		t.Fatal(err)
	}
	defer r.DestroyRepo()

	router := newPaymentPolicyRouter(t, r.DB())
	payload := `{"utxoConfirmationPolicy":"instant"}`
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/v1/settings/payment-policy", bytes.NewReader([]byte(payload)))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("PUT status = %d body=%s, want 400", rr.Code, rr.Body.String())
	}
}
