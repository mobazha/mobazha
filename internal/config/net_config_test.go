package config

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	iwallet "github.com/mobazha/mobazha3.0/pkg/wallet-interface"
)

func TestGenerateJson(t *testing.T) {
	config := &NetConfig{
		BootstrapAddrs: []string{
			// "/ip4/140.246.224.238/tcp/4001/p2p/12D3KooWD1GpGf11qVtcDhat8q8rB2du9nohFEFu2DgciUYWY2BC",
			// "/ip4/192.227.231.231/tcp/4001/p2p/12D3KooWSsoZBMiQjvPctdqckrAGukta3q7kAZS7cQRwfwbet7zG",
			// "/ip4/115.220.5.230/tcp/4001/p2p/12D3KooWLSei5eJ8o8mWoS8SsEj5ymL93kFYvNgHA4PpdVhhZyuu",
			// "/ip4/43.153.84.212/tcp/4001/p2p/12D3KooWC37TxYV9UGrcxwi3kmupGaDNC5YTo1BDL7TrWQHPfh5S",
		},
		StoreAndForwardServers: []string{
			// "12D3KooWD1GpGf11qVtcDhat8q8rB2du9nohFEFu2DgciUYWY2BC",
			// "12D3KooWSsoZBMiQjvPctdqckrAGukta3q7kAZS7cQRwfwbet7zG",
			// "12D3KooWLSei5eJ8o8mWoS8SsEj5ymL93kFYvNgHA4PpdVhhZyuu",
			// "12D3KooWC37TxYV9UGrcxwi3kmupGaDNC5YTo1BDL7TrWQHPfh5S",
		},
		ExchangeRateProviders: []string{
			// "https://info.mobazha.org/api/ticker",
		},
		PlatformAddrs: map[iwallet.ChainType]string{
			iwallet.ChainBitcoinCash: "ppaz03a9gc9r339wq9ctggf5st79zkjfxgle6qvuss",
		},
		Data: map[string]string{
			// "netDBEndpoint":       "",
			// "verifiedModEndpoint": "",
		},
	}
	val, _ := json.Marshal(config)
	t.Log(string(val))
}

func TestLoadNetConfig(t *testing.T) {
	fixture := NetConfig{
		BootstrapAddrs:         []string{"/ip4/127.0.0.1/tcp/4001/p2p/12D3KooWTestPeerID"},
		StoreAndForwardServers: []string{"12D3KooWTestPeerID"},
		PlatformAddrs: map[iwallet.ChainType]string{
			iwallet.ChainBitcoin: "bc1qtest",
		},
		Data: map[string]string{
			"commission": "0.01",
		},
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(&fixture)
	}))
	defer srv.Close()

	netConfig, err := LoadNetConfig(srv.URL)
	if err != nil {
		t.Fatal(err)
	}

	commission := netConfig.GetCommission()
	if commission != 0.01 {
		t.Errorf("expected commission 0.01, got %f", commission)
	}

	btcAddr := netConfig.GetPlatformAddr(iwallet.ChainBitcoin)
	if btcAddr != "bc1qtest" {
		t.Errorf("expected btcAddr bc1qtest, got %s", btcAddr)
	}

	if len(netConfig.BootstrapAddrs) != 1 {
		t.Errorf("expected 1 bootstrap addr, got %d", len(netConfig.BootstrapAddrs))
	}
}
