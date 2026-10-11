// SPDX-License-Identifier: MPL-2.0
// Copyright (c) 2026 fengzie and the respective contributors.

// Command buyrequest-mock-index serves the local stand-in for the hosted
// index's buyer request endpoints.
//
// Run it while developing the node or the web client, and point the node at it
// with --netdbendpoint so development never publishes to the production index:
//
//	go run ./cmd/buyrequest-mock-index -addr 127.0.0.1:4310
//
// The mock keeps everything in memory, applies the rules from RFC-0017 (signature
// checks, increasing revisions, the 30-day ceiling, the per-node quota, the ban
// list), and prints one line per accepted or refused call so a run is
// reviewable.
package main

import (
	"flag"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/mobazha/mobazha/internal/buyrequestmock"
)

func main() {
	addr := flag.String("addr", "127.0.0.1:4310", "address to listen on")
	banned := flag.String("ban", "", "comma-separated peer IDs to refuse")
	flag.Parse()

	var bannedPeers []string
	for _, peerID := range strings.Split(*banned, ",") {
		if trimmed := strings.TrimSpace(peerID); trimmed != "" {
			bannedPeers = append(bannedPeers, trimmed)
		}
	}

	index := buyrequestmock.NewIndex(buyrequestmock.Config{
		BannedPeers: bannedPeers,
		Logger: func(format string, args ...any) {
			log.Printf(format, args...)
		},
	})

	server := &http.Server{
		Addr:              *addr,
		Handler:           index.Handler(),
		ReadHeaderTimeout: 10 * time.Second,
	}
	log.Printf("buy request mock index listening on http://%s/search/v1", *addr)
	if err := server.ListenAndServe(); err != nil {
		log.Fatalf("mock index stopped: %s", err)
	}
}
