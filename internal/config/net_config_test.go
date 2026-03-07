package config

import (
	"encoding/json"
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
	netConfig, err := LoadNetConfig("https://mobazha.info/search/v1/config")
	if err != nil {
		t.Fatal(err)
	}
	t.Log(netConfig)

	commission := netConfig.GetCommission()
	t.Logf("commission is %f", commission)

	btcAddr := netConfig.GetPlatformAddr(iwallet.ChainBitcoin)
	t.Logf("btcAddr is %s", btcAddr)

	t.Logf("GetExtraFeesPerByte: %s", netConfig.GetExtraFeesPerByte(iwallet.ChainBitcoinCash))
}

func TestNetConfig_GetExtraFeesPerByte(t *testing.T) {
	t.Log(DefaultNetConfig().GetExtraFeesPerByte(iwallet.ChainBitcoinCash))
}
