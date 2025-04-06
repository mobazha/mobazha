package repo

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path"
	"reflect"
	"testing"

	config "github.com/ipfs/kubo/config"
	"github.com/mobazha/mobazha3.0/pkg/database"
	"github.com/mobazha/mobazha3.0/pkg/models"
)

func TestRepo_cleanIdentityFromConfig(t *testing.T) {
	var (
		dir            = path.Join(os.TempDir(), "mobazha", "cleantest")
		configFilePath = path.Join(dir, "config")
	)

	if err := os.MkdirAll(dir, os.ModePerm); err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	if err := os.WriteFile(configFilePath, []byte(`{"Identity": "abc"}`), os.ModePerm); err != nil {
		t.Fatal(err)
	}

	if err := cleanIdentityFromConfig(dir); err != nil {
		t.Fatal(err)
	}

	cfg, err := os.ReadFile(configFilePath)
	if err != nil {
		t.Fatal(err)
	}

	if !bytes.Equal(cfg, []byte("{}")) {
		t.Error("Failed to properly clean config file")
	}
}

var expectedConfigJSON = `{"Identity":{"PeerID":"QmRVbfRMxkTFUttgmEL7yxeLFMrbexzyKK6PLrkS5azHwP","PrivKey":"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"},"Datastore":{"StorageMax":"10GB","StorageGCWatermark":90,"GCPeriod":"1h","Spec":{"mounts":[{"child":{"path":"blocks","shardFunc":"/repo/flatfs/shard/v1/next-to-last/2","sync":true,"type":"flatfs"},"mountpoint":"/blocks","prefix":"flatfs.datastore","type":"measure"},{"child":{"compression":"none","path":"datastore","type":"levelds"},"mountpoint":"/","prefix":"leveldb.datastore","type":"measure"}],"type":"mount"},"HashOnRead":false,"BloomFilterSize":0},"Addresses":{"Swarm":["/ip4/0.0.0.0/tcp/4001","/ip6/::/tcp/4001","/ip4/0.0.0.0/tcp/9005/ws","/ip6/::/tcp/9005/ws"],"Announce":null,"AppendAnnounce":null,"NoAnnounce":null,"API":null,"Gateway":"/ip4/127.0.0.1/tcp/4002"},"Mounts":{"IPFS":"/ipfs","IPNS":"/ipns","FuseAllowOther":false},"Discovery":{"MDNS":{"Enabled":false}},"Routing":{"Type":"dht","Routers":null},"Ipns":{"RepublishPeriod":"","RecordLifetime":"720h","ResolveCacheSize":128},"Bootstrap":[],"Gateway":{"HTTPHeaders":{"Access-Control-Allow-Headers":["X-Requested-With","Range","User-Agent"],"Access-Control-Allow-Methods":["GET"],"Access-Control-Allow-Origin":["*"]},"RootRedirect":"","Writable":false,"PathPrefixes":[],"APICommands":[],"NoFetch":false,"NoDNSLink":false,"PublicGateways":null},"API":{"HTTPHeaders":{}},"Swarm":{"AddrFilters":null,"DisableBandwidthMetrics":false,"DisableNatPortMap":false,"RelayClient":{"Enabled":true},"RelayService":{},"Transports":{"Network":{},"Security":{},"Multiplexers":{}},"ConnMgr":{"Type":"basic","LowWater":600,"HighWater":900,"GracePeriod":"20s"},"ResourceMgr":{}},"AutoNAT":{},"Pubsub":{"Router":"","DisableSigning":false},"Peering":{"Peers":null},"DNS":{"Resolvers":{}},"Migration":{"DownloadSources":[],"Keep":""},"Provider":{"Strategy":""},"Reprovider":{"Interval":"12h","Strategy":"all"},"Experimental":{"FilestoreEnabled":false,"UrlstoreEnabled":false,"GraphsyncEnabled":false,"Libp2pStreamMounting":false,"P2pHttpProxy":false,"StrategicProviding":false,"AcceleratedDHTClient":false},"Plugins":{"Plugins":null},"Pinning":{"RemoteServices":{}},"Internal":{}}`

func TestRepo_mustDefaultConfig(t *testing.T) {
	cfg := mustDefaultConfig(true)
	// The Strings type screws up the json serialization
	// so we'll just set it to nil here to pass the test.
	// It should always be nil anyway.
	cfg.Addresses.API = nil

	expectedConfig := &config.Config{}
	if err := json.Unmarshal([]byte(expectedConfigJSON), expectedConfig); err != nil {
		t.Fatal(err)
	}
	expectedConfig.Identity = cfg.Identity
	expectedConfig.Addresses.API = nil
	expectedConfig.DNS.Resolvers = make(map[string]string)
	expectedConfig.Pinning.RemoteServices = make(map[string]config.RemotePinningService)
	expectedConfig.Migration.DownloadSources = []string{}
	expectedConfig.Reprovider = config.Reprovider{}
	expectedConfig.Swarm.ConnMgr = config.ConnMgr{}

	fmt.Printf("expectedConfig: %v\ncfg:%v\n", expectedConfig, cfg)

	if !reflect.DeepEqual(expectedConfig, cfg) {
		t.Error("Returned incorrect config file")
	}
}

func TestNewRepo(t *testing.T) {
	var dir = path.Join(os.TempDir(), "mobazha", "newRepoTest")
	r, err := NewRepo(dir, true)
	if err != nil {
		t.Fatal(err)
	}
	defer r.DestroyRepo()

	if r.DB() == nil {
		t.Error("Failed to initialize the database")
	}
}

func TestNewRepoWithCustomMnemonicSeed(t *testing.T) {
	var (
		dir      = path.Join(os.TempDir(), "mobazha", "newRepoTest")
		mnemonic = "abc"
	)
	r, err := NewRepoWithCustomMnemonicSeed(dir, mnemonic, true)
	if err != nil {
		t.Fatal(err)
	}
	defer r.DestroyRepo()

	var dbSeed models.Key
	err = r.db.View(func(tx database.Tx) error {
		return tx.Read().Where("name = ?", "mnemonic").First(&dbSeed).Error
	})
	if err != nil {
		t.Fatal(err)
	}

	if string(dbSeed.Value) != mnemonic {
		t.Errorf("Failed to set correct mnemonic. Expected %s, got %s", mnemonic, string(dbSeed.Value))
	}
}
