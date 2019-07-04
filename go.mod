module github.com/cpacia/openbazaar3.0

go 1.12

require github.com/jessevdk/go-flags v1.4.0

require (
	github.com/OpenBazaar/golang-socketio v0.0.0-20181127201421-909b73d947ae // indirect
	github.com/OpenBazaar/jsonpb v0.0.0-20171123000858-37d32ddf4eef
	github.com/OpenBazaar/multiwallet v0.0.0-20190704133557-52b6b512b299
	github.com/OpenBazaar/spvwallet v0.0.0-20190417151124-49419d61fdff // indirect
	github.com/OpenBazaar/wallet-interface v0.0.0-20190411204206-5b458c29c191 // indirect
	github.com/btcsuite/btcd v0.0.0-20190523000118-16327141da8c
	github.com/btcsuite/btcutil v0.0.0-20190425235716-9e5f4b9a998d
	github.com/btcsuite/btcwallet v0.0.0-20190628225330-4a9774585e57 // indirect
	github.com/cevaris/ordered_map v0.0.0-20190319150403-3adeae072e73 // indirect
	github.com/coreos/go-semver v0.3.0 // indirect
	github.com/cpacia/bchutil v0.0.0-20181003130114-b126f6a35b6c // indirect
	github.com/dchest/siphash v1.2.1 // indirect
	github.com/dgraph-io/badger/v2 v2.0.0-rc2 // indirect
	github.com/gcash/bchd v0.14.6 // indirect
	github.com/gcash/bchlog v0.0.0-20180913005452-b4f036f92fa6 // indirect
	github.com/gcash/bchutil v0.0.0-20190625002603-800e62fe9aff // indirect
	github.com/gogo/protobuf v1.2.1
	github.com/golang/protobuf v1.3.1
	github.com/gosimple/slug v1.5.0
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
	github.com/libp2p/go-libp2p-crypto v0.0.2
	github.com/libp2p/go-libp2p-host v0.0.3
	github.com/libp2p/go-libp2p-kad-dht v0.0.13
	github.com/libp2p/go-libp2p-net v0.0.2
	github.com/libp2p/go-libp2p-peer v0.1.1
	github.com/libp2p/go-libp2p-peerstore v0.0.6
	github.com/libp2p/go-libp2p-protocol v0.0.1
	github.com/libp2p/go-libp2p-record v0.0.1
	github.com/libp2p/go-libp2p-routing v0.0.1
	github.com/ltcsuite/ltcd v0.0.0-20190519120615-e27ee083f08f // indirect
	github.com/ltcsuite/ltcutil v0.0.0-20190507133322-23cdfa9fcc3d // indirect
	github.com/ltcsuite/ltcwallet v0.0.0-20190105125346-3fa612e326e5 // indirect
	github.com/microcosm-cc/bluemonday v1.0.2
	github.com/multiformats/go-multihash v0.0.5
	github.com/natefinch/lumberjack v2.0.0+incompatible
	github.com/op/go-logging v0.0.0-20160315200505-970db520ece7
	github.com/rainycape/unidecode v0.0.0-20150907023854-cb7f23ec59be // indirect
	github.com/tyler-smith/go-bip39 v1.0.0
	github.com/whyrusleeping/go-logging v0.0.0-20170515211332-0457bb6b88fc
	gopkg.in/natefinch/lumberjack.v2 v2.0.0 // indirect
)

replace github.com/dgraph-io/badger => github.com/dgraph-io/badger/v2 v2.0.0-rc.2+incompatible
