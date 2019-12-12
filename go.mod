module github.com/cpacia/openbazaar3.0

go 1.13

require (
	github.com/OpenBazaar/jsonpb v0.0.0-20171123000858-37d32ddf4eef
	github.com/agl/ed25519 v0.0.0-20170116200512-5312a6153412
	github.com/btcsuite/btcd v0.20.1-beta
	github.com/btcsuite/btcutil v0.0.0-20190425235716-9e5f4b9a998d
	github.com/coreos/go-semver v0.3.0 // indirect
	github.com/cpacia/go-store-and-forward v0.0.0-20191212030550-66dbb96ead2a
	github.com/cpacia/multiwallet v0.0.0-20191212004439-8a5a7dc8bfec
	github.com/cpacia/proxyclient v0.1.0
	github.com/cpacia/wallet-interface v0.0.0-20191211190928-d7ad7fbaf4ec
	github.com/fatih/color v1.7.0
	github.com/gogo/protobuf v1.2.1
	github.com/golang/protobuf v1.3.2
	github.com/gorilla/mux v1.7.3
	github.com/gorilla/websocket v1.4.1
	github.com/gosimple/slug v1.6.0
	github.com/ipfs/go-bitswap v0.0.8-0.20190704155249-cbb485998356
	github.com/ipfs/go-cid v0.0.2
	github.com/ipfs/go-datastore v0.0.5
	github.com/ipfs/go-ipfs v0.4.22
	github.com/ipfs/go-ipfs-config v0.0.3
	github.com/ipfs/go-ipfs-files v0.0.3
	github.com/ipfs/go-ipns v0.0.1
	github.com/ipfs/go-log v0.0.1
	github.com/ipfs/go-merkledag v0.0.3
	github.com/ipfs/go-path v0.0.4
	github.com/ipfs/interface-go-ipfs-core v0.0.8
	github.com/jarcoal/httpmock v1.0.4
	github.com/jbenet/go-context v0.0.0-20150711004518-d14ea06fba99
	github.com/jessevdk/go-flags v1.4.0
	github.com/jinzhu/gorm v1.9.11
	github.com/libp2p/go-libp2p v0.0.28
	github.com/libp2p/go-libp2p-crypto v0.0.2
	github.com/libp2p/go-libp2p-host v0.0.3
	github.com/libp2p/go-libp2p-kad-dht v0.0.13
	github.com/libp2p/go-libp2p-net v0.0.2
	github.com/libp2p/go-libp2p-peer v0.1.1
	github.com/libp2p/go-libp2p-peerstore v0.0.6
	github.com/libp2p/go-libp2p-protocol v0.0.1
	github.com/libp2p/go-libp2p-record v0.0.1
	github.com/libp2p/go-libp2p-routing v0.0.1
	github.com/libp2p/go-msgio v0.0.4 // indirect
	github.com/libp2p/go-testutil v0.0.1
	github.com/microcosm-cc/bluemonday v1.0.2
	github.com/multiformats/go-multiaddr v0.0.4
	github.com/multiformats/go-multiaddr-net v0.0.1
	github.com/multiformats/go-multihash v0.0.6
	github.com/natefinch/lumberjack v2.0.0+incompatible
	github.com/op/go-logging v0.0.0-20160315200505-970db520ece7
	github.com/pkg/errors v0.8.1
	github.com/rainycape/unidecode v0.0.0-20150907023854-cb7f23ec59be // indirect
	github.com/tyler-smith/go-bip39 v1.0.2
	github.com/yawning/bulb v0.0.0-20170405033506-85d80d893c3d
	golang.org/x/crypto v0.0.0-20190923035154-9ee001bba392
	golang.org/x/net v0.0.0-20191209160850-c0dbc17a3553
	golang.org/x/xerrors v0.0.0-20191011141410-1b5146add898 // indirect
)

replace (
	github.com/go-critic/go-critic => github.com/go-critic/go-critic v0.4.0
	github.com/golangci/golangci-lint => github.com/golangci/golangci-lint v1.21.0
)
