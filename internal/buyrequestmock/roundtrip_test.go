// SPDX-License-Identifier: MPL-2.0
// Copyright (c) 2026 fengzie and the respective contributors.

package buyrequestmock_test

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/mobazha/mobazha/internal/buyrequestmock"
	"github.com/mobazha/mobazha/pkg/database/netdb"
	"github.com/mobazha/mobazha/pkg/orders"
	opb "github.com/mobazha/mobazha/pkg/orders/mbzpb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// TestNodePublishesToTheIndex is the end-to-end check the record contract asks
// for: a node signs a request with its own identity key and pushes it through
// the real search-service client, and the index accepts it and serves it back to
// a reader. Both halves therefore agree on the bytes, the signature and the
// listing rules, not just on the shape of a struct.
func TestNodePublishesToTheIndex(t *testing.T) {
	privateKey, _, err := crypto.GenerateEd25519Key(rand.Reader)
	if err != nil {
		t.Fatalf("generate node key: %s", err)
	}
	peerID, err := peer.IDFromPrivateKey(privateKey)
	if err != nil {
		t.Fatalf("derive peer ID: %s", err)
	}

	server := httptest.NewServer(buyrequestmock.NewIndex(buyrequestmock.Config{}).Handler())
	defer server.Close()

	client, err := netdb.NewNetDB(server.URL+"/search/v1/netdb", peerID.String(), privateKey)
	if err != nil {
		t.Fatalf("build search-service client: %s", err)
	}

	now := time.Now().UTC().Truncate(time.Second)
	record := &opb.BuyerRequest{
		RequestID: "card-projection-check",
		Revision:  1,
		PeerID:    peerID.String(),
		Title:     "Card projection check",
		Quantity:  2,
		Budget: &opb.CurrencyValue{
			Currency: &opb.Currency{Code: "BTC", Divisibility: 8},
			Amount:   "150000",
		},
		AcceptedCurrencies: []string{"BTC", "LTC"},
		ShipTo:             "NL",
		ProductType:        "video-projectors",
		Description:        "Two projectors, please.",
		Images: []*opb.Image{{
			Filename: "front.jpg",
			Original: "QmOriginal",
			Tiny:     "QmTiny",
		}},
		Status:    orders.BuyerRequestStatusOpen,
		CreatedAt: timestamppb.New(now),
		ExpiresAt: timestamppb.New(now.Add(7 * 24 * time.Hour)),
	}
	if err := orders.ValidateBuyerRequest(record, now); err != nil {
		t.Fatalf("the node must not sign a record the index will refuse: %s", err)
	}

	if err := client.SetOwnBuyRequest(record); err != nil {
		t.Fatalf("publish: %s", err)
	}

	page := fetchBoard(t, server.URL, "")
	if page.Meta.Total != 1 || len(page.Data) != 1 {
		t.Fatalf("a published request must appear on the board, got %+v", page.Meta)
	}
	if page.Data[0].Type != "buyer_request" {
		t.Fatalf("a board hit must be typed as a request, got %q", page.Data[0].Type)
	}
	listed := decodeRecord(t, page.Data[0].Data)
	if listed.GetRequestID() != record.GetRequestID() || listed.GetPeerID() != peerID.String() {
		t.Fatalf("the board served %s owned by %s", listed.GetRequestID(), listed.GetPeerID())
	}
	if listed.GetBudget().GetAmount() != "150000" ||
		listed.GetBudget().GetCurrency().GetCode() != "BTC" {
		t.Fatalf("the budget did not survive the round trip: %+v", listed.GetBudget())
	}
	if len(listed.GetImages()) != 1 || listed.GetImages()[0].GetOriginal() != "QmOriginal" {
		t.Fatalf("the images did not survive the round trip: %+v", listed.GetImages())
	}

	detailURL := fmt.Sprintf("%s/search/v1/buyer-requests/%s/%s", server.URL,
		peerID.String(), record.GetRequestID())
	detail := fetchRecord(t, detailURL)
	if detail.GetRequestID() != record.GetRequestID() {
		t.Fatalf("detail served %s", detail.GetRequestID())
	}

	if err := client.DeleteOwnBuyRequest(record.GetRequestID(), record.GetRevision()); err != nil {
		t.Fatalf("withdraw: %s", err)
	}
	if page := fetchBoard(t, server.URL, ""); page.Meta.Total != 0 {
		t.Fatalf("a withdrawn request must leave the board, total is %d", page.Meta.Total)
	}
}

// TestStaleRevisionIsRefused covers the rule that makes an edit safe: the index
// keeps the newest copy, so a node that republishes an older revision is told
// its copy is stale instead of silently rolling the board back.
func TestStaleRevisionIsRefused(t *testing.T) {
	privateKey, _, err := crypto.GenerateEd25519Key(rand.Reader)
	if err != nil {
		t.Fatalf("generate node key: %s", err)
	}
	peerID, err := peer.IDFromPrivateKey(privateKey)
	if err != nil {
		t.Fatalf("derive peer ID: %s", err)
	}

	now := time.Now().UTC().Truncate(time.Second)
	index := buyrequestmock.NewIndex(buyrequestmock.Config{})
	server := httptest.NewServer(index.Handler())
	defer server.Close()

	client, err := netdb.NewNetDB(server.URL+"/search/v1/netdb", peerID.String(), privateKey)
	if err != nil {
		t.Fatalf("build search-service client: %s", err)
	}

	record := &opb.BuyerRequest{
		RequestID:          "stale-revision-check",
		Revision:           1,
		PeerID:             peerID.String(),
		Title:              "First wording",
		Quantity:           1,
		Budget:             &opb.CurrencyValue{Currency: &opb.Currency{Code: "BTC", Divisibility: 8}, Amount: "1000"},
		AcceptedCurrencies: []string{"BTC"},
		ShipTo:             "NL",
		ProductType:        "video-projectors",
		Status:             orders.BuyerRequestStatusOpen,
		CreatedAt:          timestamppb.New(now),
		ExpiresAt:          timestamppb.New(now.Add(24 * time.Hour)),
	}
	if err := client.SetOwnBuyRequest(record); err != nil {
		t.Fatalf("publish revision 1: %s", err)
	}

	record.Revision = 2
	record.Title = "Second wording"
	if err := client.SetOwnBuyRequest(record); err != nil {
		t.Fatalf("publish revision 2: %s", err)
	}

	record.Revision = 1
	record.Title = "First wording"
	if err := client.SetOwnBuyRequest(record); err == nil {
		t.Fatal("an older revision must be refused")
	}

	page := fetchBoard(t, server.URL, "")
	if len(page.Data) != 1 {
		t.Fatalf("expected one record on the board, got %d", len(page.Data))
	}
	listed := decodeRecord(t, page.Data[0].Data)
	if listed.GetRevision() != 2 || listed.GetTitle() != "Second wording" {
		t.Fatalf("the board rolled back to revision %d (%q)", listed.GetRevision(), listed.GetTitle())
	}
}

type boardHit struct {
	Data json.RawMessage `json:"data"`
	Type string          `json:"type"`
}

type boardPage struct {
	Data []boardHit `json:"data"`
	Meta struct {
		Total   int  `json:"total"`
		HasMore bool `json:"hasMore"`
	} `json:"meta"`
}

func fetchBoard(t *testing.T, baseURL, query string) boardPage {
	t.Helper()
	resp, err := http.Get(baseURL + "/search/v1/buyer-requests" + query)
	if err != nil {
		t.Fatalf("list the board: %s", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("list the board: %s", resp.Status)
	}
	var page boardPage
	if err := json.NewDecoder(resp.Body).Decode(&page); err != nil {
		t.Fatalf("decode the board: %s", err)
	}
	return page
}

func fetchRecord(t *testing.T, url string) *opb.BuyerRequest {
	t.Helper()
	resp, err := http.Get(url)
	if err != nil {
		t.Fatalf("read the record: %s", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("read the record: %s", resp.Status)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read the record body: %s", err)
	}
	var envelope struct {
		Data json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(body, &envelope); err != nil {
		t.Fatalf("decode the record envelope: %s", err)
	}
	return decodeRecord(t, envelope.Data)
}

// decodeRecord reads a record back with the node's own parser, so the test
// covers the decoder the node uses rather than a test-only copy of it.
func decodeRecord(t *testing.T, raw json.RawMessage) *opb.BuyerRequest {
	t.Helper()
	record, err := orders.ParseBuyerRequest(raw)
	if err != nil {
		t.Fatalf("decode the record: %s", err)
	}
	return record
}
