// SPDX-License-Identifier: MPL-2.0
// Copyright (c) 2026 fengzie and the respective contributors.

package core

import (
	"encoding/hex"
	"testing"

	"github.com/btcsuite/btcd/btcutil/hdkeychain"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/mobazha/mobazha/internal/repo"
	"github.com/mobazha/mobazha/pkg/contracts"
)

func testMultiwallet(t *testing.T, masterKey *hdkeychain.ExtendedKey) contracts.WalletOperator {
	t.Helper()
	cfg := &repo.Config{LogLevel: "error"}
	mw, err := loadTestMultiwallet(masterKey, cfg, nil, false, t.TempDir())
	if err != nil {
		t.Fatalf("testMultiwallet: %v", err)
	}
	return &mw
}

func testMasterKey(t *testing.T) *hdkeychain.ExtendedKey {
	t.Helper()
	seed, err := hex.DecodeString(
		"000102030405060708090a0b0c0d0e0f" +
			"101112131415161718191a1b1c1d1e1f")
	if err != nil {
		t.Fatal(err)
	}
	masterKey, err := hdkeychain.NewMaster(seed, &chaincfg.MainNetParams)
	if err != nil {
		t.Fatal(err)
	}
	purposeKey, err := masterKey.Derive(hdkeychain.HardenedKeyStart + 44)
	if err != nil {
		t.Fatal(err)
	}
	return purposeKey
}
