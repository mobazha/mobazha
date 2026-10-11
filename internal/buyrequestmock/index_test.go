// SPDX-License-Identifier: MPL-2.0
// Copyright (c) 2026 fengzie and the respective contributors.

package buyrequestmock

import (
	"bytes"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	libp2pcrypto "github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/mobazha/mobazha/pkg/orders"
	opb "github.com/mobazha/mobazha/pkg/orders/mbzpb"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// node is one publishing node in a test: its identity key and peer ID.
type node struct {
	key    libp2pcrypto.PrivKey
	peerID string
}

func newNode(t *testing.T) node {
	t.Helper()
	key, _, err := libp2pcrypto.GenerateEd25519Key(rand.Reader)
	require.NoError(t, err)
	id, err := peer.IDFromPrivateKey(key)
	require.NoError(t, err)
	return node{key: key, peerID: id.String()}
}

// record builds a valid record for a node.
func (n node) record(t *testing.T, requestID string, revision uint32, now time.Time) *opb.BuyerRequest {
	t.Helper()
	return &opb.BuyerRequest{
		RequestID: requestID,
		Revision:  revision,
		PeerID:    n.peerID,
		Title:     "Vintage camera body, working meter",
		Quantity:  1,
		Budget: &opb.CurrencyValue{
			Currency: &opb.Currency{Code: "BTC", Divisibility: 8},
			Amount:   "150000",
		},
		AcceptedCurrencies: []string{"BTC", "LTC"},
		ShipTo:             "NL",
		ProductType:        "photography",
		Description:        "Body only; dents are fine, a dead meter is not.",
		Status:             orders.BuyerRequestStatusOpen,
		CreatedAt:          timestamppb.New(now),
		ExpiresAt:          timestamppb.New(now.Add(7 * 24 * time.Hour)),
	}
}

func (n node) publish(t *testing.T, server *httptest.Server, request *opb.BuyerRequest) *http.Response {
	t.Helper()
	serialized, signature, err := orders.SignBuyerRequest(n.key, request)
	require.NoError(t, err)
	body, err := json.Marshal(publication{
		PeerID:            n.peerID,
		SerializedRequest: serialized,
		Signature:         signature,
	})
	require.NoError(t, err)

	resp, err := http.Post(server.URL+"/search/v1/netdb/buyer-requests", "application/json", bytes.NewReader(body))
	require.NoError(t, err)
	t.Cleanup(func() { _ = resp.Body.Close() })
	return resp
}

func (n node) withdraw(t *testing.T, server *httptest.Server, requestID string, revision uint32) *http.Response {
	t.Helper()
	payload := orders.BuyerRequestWithdrawPayload(n.peerID, requestID, revision)
	signature, err := n.key.Sign(payload)
	require.NoError(t, err)
	body, err := json.Marshal(withdrawal{PeerID: n.peerID, Revision: revision, Signature: signature})
	require.NoError(t, err)

	req, err := http.NewRequest(http.MethodDelete, server.URL+"/search/v1/netdb/buyer-requests/"+requestID, bytes.NewReader(body))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	t.Cleanup(func() { _ = resp.Body.Close() })
	return resp
}

// listItems reads a board page and returns the parsed records plus the meta.
func listItems(t *testing.T, server *httptest.Server, query string) ([]*opb.BuyerRequest, map[string]any) {
	t.Helper()
	resp, err := http.Get(server.URL + "/search/v1/buyer-requests" + query)
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var payload struct {
		Data []struct {
			Data json.RawMessage `json:"data"`
			Type string          `json:"type"`
		} `json:"data"`
		Meta map[string]any `json:"meta"`
	}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&payload))

	records := make([]*opb.BuyerRequest, 0, len(payload.Data))
	for _, item := range payload.Data {
		assert.Equal(t, "buyer_request", item.Type)
		record, err := orders.ParseBuyerRequest(item.Data)
		require.NoError(t, err)
		records = append(records, record)
	}
	return records, payload.Meta
}

func TestPublishListSearchAndDetail(t *testing.T) {
	now := time.Date(2026, 10, 11, 8, 0, 0, 0, time.UTC)
	index := NewIndex(Config{Now: func() time.Time { return now }})
	server := httptest.NewServer(index.Handler())
	defer server.Close()

	publisher := newNode(t)
	record := publisher.record(t, "6f1c2a9e-4d1b-4f5e-9a51-2b8f0c7d3e10", 1, now)

	resp := publisher.publish(t, server, record)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	// The board lists it, and each hit is wrapped like a listing hit.
	records, meta := listItems(t, server, "")
	require.Len(t, records, 1)
	assert.Equal(t, record.GetRequestID(), records[0].GetRequestID())
	assert.Equal(t, float64(1), meta["total"])
	assert.Equal(t, false, meta["hasMore"])

	// Search and filters.
	byTitle, _ := listItems(t, server, "?q=working+meter")
	assert.Len(t, byTitle, 1)
	byMiss, _ := listItems(t, server, "?q=nonexistent")
	assert.Empty(t, byMiss)
	byType, _ := listItems(t, server, "?productType=photography")
	assert.Len(t, byType, 1)
	byOtherType, _ := listItems(t, server, "?productType=wigs")
	assert.Empty(t, byOtherType)
	byCountry, _ := listItems(t, server, "?shipTo=nl")
	assert.Len(t, byCountry, 1)
	byCurrency, _ := listItems(t, server, "?currency=LTC")
	assert.Len(t, byCurrency, 1)
	byBudget, _ := listItems(t, server, "?minBudget=100000&maxBudget=200000&budgetCurrency=BTC")
	assert.Len(t, byBudget, 1)
	byBudgetMiss, _ := listItems(t, server, "?minBudget=200000&budgetCurrency=BTC")
	assert.Empty(t, byBudgetMiss)

	// Detail is public under the owner and the identifier.
	detail, err := http.Get(fmt.Sprintf("%s/search/v1/buyer-requests/%s/%s", server.URL, publisher.peerID, record.GetRequestID()))
	require.NoError(t, err)
	defer func() { _ = detail.Body.Close() }()
	assert.Equal(t, http.StatusOK, detail.StatusCode)
}

func TestRevisionMustIncrease(t *testing.T) {
	now := time.Now().UTC()
	index := NewIndex(Config{Now: func() time.Time { return now }})
	server := httptest.NewServer(index.Handler())
	defer server.Close()

	publisher := newNode(t)
	require.Equal(t, http.StatusOK, publisher.publish(t, server, publisher.record(t, "req-1", 1, now)).StatusCode)
	require.Equal(t, http.StatusOK, publisher.publish(t, server, publisher.record(t, "req-1", 2, now)).StatusCode)

	// An older record must not overwrite a newer one: a replay of revision 1 is
	// refused, and the stored record stays at revision 2.
	replay := publisher.publish(t, server, publisher.record(t, "req-1", 1, now))
	assert.Equal(t, http.StatusConflict, replay.StatusCode)

	records, _ := listItems(t, server, "")
	require.Len(t, records, 1)
	assert.Equal(t, uint32(2), records[0].GetRevision())
}

func TestWithdrawRemovesTheRecordAndOnlyTheOwnerMayWithdraw(t *testing.T) {
	now := time.Now().UTC()
	index := NewIndex(Config{Now: func() time.Time { return now }})
	server := httptest.NewServer(index.Handler())
	defer server.Close()

	publisher := newNode(t)
	record := publisher.record(t, "req-2", 1, now)
	require.Equal(t, http.StatusOK, publisher.publish(t, server, record).StatusCode)

	// Somebody else cannot take it off the board, even with a signature of their
	// own.
	intruder := newNode(t)
	assert.Equal(t, http.StatusForbidden, intruder.withdraw(t, server, "req-2", 1).StatusCode)
	records, _ := listItems(t, server, "")
	assert.Len(t, records, 1)

	assert.Equal(t, http.StatusOK, publisher.withdraw(t, server, "req-2", 1).StatusCode)
	records, _ = listItems(t, server, "")
	assert.Empty(t, records)

	detail, err := http.Get(fmt.Sprintf("%s/search/v1/buyer-requests/%s/req-2", server.URL, publisher.peerID))
	require.NoError(t, err)
	defer func() { _ = detail.Body.Close() }()
	assert.Equal(t, http.StatusNotFound, detail.StatusCode)
}

func TestTamperedRecordIsRefused(t *testing.T) {
	now := time.Now().UTC()
	index := NewIndex(Config{Now: func() time.Time { return now }})
	server := httptest.NewServer(index.Handler())
	defer server.Close()

	publisher := newNode(t)
	record := publisher.record(t, "req-3", 1, now)
	serialized, signature, err := orders.SignBuyerRequest(publisher.key, record)
	require.NoError(t, err)

	// Change the title after signing: the signature no longer covers these bytes.
	tampered := bytes.Replace(serialized, []byte("Vintage camera body"), []byte("Vintage camera bodY"), 1)
	require.NotEqual(t, serialized, tampered)
	body, err := json.Marshal(publication{
		PeerID:            publisher.peerID,
		SerializedRequest: tampered,
		Signature:         signature,
	})
	require.NoError(t, err)
	resp, err := http.Post(server.URL+"/search/v1/netdb/buyer-requests", "application/json", bytes.NewReader(body))
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}

func TestExpiredRecordsStayOffTheBoard(t *testing.T) {
	published := time.Date(2026, 10, 11, 8, 0, 0, 0, time.UTC)
	clock := published
	index := NewIndex(Config{Now: func() time.Time { return clock }})
	server := httptest.NewServer(index.Handler())
	defer server.Close()

	publisher := newNode(t)
	record := publisher.record(t, "req-4", 1, published)
	record.ExpiresAt = timestamppb.New(published.Add(2 * time.Hour))
	require.Equal(t, http.StatusOK, publisher.publish(t, server, record).StatusCode)

	records, _ := listItems(t, server, "")
	require.Len(t, records, 1)

	// A node only sweeps expiry when something reads its own board, so a reader
	// must not show a record past its deadline even though nothing withdrew it.
	clock = published.Add(3 * time.Hour)
	records, _ = listItems(t, server, "")
	assert.Empty(t, records)
}

func TestQuotaAndBanList(t *testing.T) {
	now := time.Now().UTC()
	index := NewIndex(Config{Now: func() time.Time { return now }})
	server := httptest.NewServer(index.Handler())
	defer server.Close()

	publisher := newNode(t)
	last := http.StatusOK
	for i := 0; i < orders.BuyerRequestMaxOpenPerPeer+1; i++ {
		last = publisher.publish(t, server, publisher.record(t, fmt.Sprintf("req-quota-%d", i), 1, now)).StatusCode
	}
	assert.Equal(t, http.StatusConflict, last)

	banned := newNode(t)
	bannedIndex := NewIndex(Config{
		Now:         func() time.Time { return now },
		BannedPeers: []string{banned.peerID},
	})
	bannedServer := httptest.NewServer(bannedIndex.Handler())
	defer bannedServer.Close()
	assert.Equal(t, http.StatusForbidden, banned.publish(t, bannedServer, banned.record(t, "req-banned", 1, now)).StatusCode)
}
