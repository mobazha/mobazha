module github.com/cpacia/openbazaar3.0

go 1.12

require github.com/jessevdk/go-flags v1.4.0

require (
	github.com/btcsuite/btcd v0.0.0-20190427004231-96897255fd17
	github.com/btcsuite/btcutil v0.0.0-20190425235716-9e5f4b9a998d
	github.com/dgraph-io/badger/v2 v2.0.0-rc2 // indirect
	github.com/gogo/protobuf v1.2.1
	github.com/golang/protobuf v1.3.1
	github.com/ipfs/go-bitswap v0.0.7
	github.com/ipfs/go-cid v0.0.2
	github.com/ipfs/go-datastore v0.0.5
	github.com/ipfs/go-ipfs v0.4.21
	github.com/ipfs/go-ipfs-config v0.0.3
	github.com/ipfs/go-ipfs-files v0.0.3
	github.com/ipfs/go-ipns v0.0.1
	github.com/ipfs/go-merkledag v0.0.3
	github.com/ipfs/go-path v0.0.4
	github.com/ipfs/interface-go-ipfs-core v0.0.8
	github.com/jbenet/go-context v0.0.0-20150711004518-d14ea06fba99
	github.com/jinzhu/gorm v1.9.9
	github.com/libp2p/go-libp2p v0.0.28
	github.com/libp2p/go-libp2p-core v0.0.1 // indirect
	github.com/libp2p/go-libp2p-crypto v0.0.2
	github.com/libp2p/go-libp2p-host v0.0.3
	github.com/libp2p/go-libp2p-kad-dht v0.0.13
	github.com/libp2p/go-libp2p-net v0.0.2
	github.com/libp2p/go-libp2p-peer v0.1.1
	github.com/libp2p/go-libp2p-peerstore v0.0.6
	github.com/libp2p/go-libp2p-protocol v0.0.1
	github.com/libp2p/go-libp2p-record v0.0.1
	github.com/libp2p/go-libp2p-routing v0.0.1
	github.com/natefinch/lumberjack v2.0.0+incompatible
	github.com/op/go-logging v0.0.0-20160315200505-970db520ece7
	github.com/tyler-smith/go-bip39 v1.0.0
	github.com/whyrusleeping/go-logging v0.0.0-20170515211332-0457bb6b88fc
)

replace github.com/dgraph-io/badger => github.com/dgraph-io/badger/v2 v2.0.0-rc.2+incompatible
