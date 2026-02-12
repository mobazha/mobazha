module github.com/mobazha/mobazha3.0

go 1.24.0

toolchain go1.24.5

replace (
	// Use local mobazha-core for development
	github.com/mobazha/mobazha-core => /Users/mingfeng/dev/mobazha/core
	// github.com/ipfs/kubo => /Users/mingfeng/go/src/github.com/mobazha/kubo

	// github.com/multiformats/go-multiaddr => /Users/mingfeng/go/src/github.com/multiformats/go-multiaddr
	github.com/nanmu42/etherscan-api => /Users/mingfeng/go/src/github.com/mobazha/etherscan-api
	github.com/wlynxg/anet => /Users/mingfeng/go/src/github.com/wlynxg/anet
)

require (
	github.com/OpenBazaar/golang-socketio v0.0.0-20200109001351-4147b5f0d294
	github.com/btcsuite/btcd v0.25.0
	github.com/btcsuite/btcd/btcec/v2 v2.3.5
	github.com/btcsuite/btcd/btcutil v1.1.5
	github.com/btcsuite/btcd/chaincfg/chainhash v1.1.0
	github.com/btcsuite/btcwallet/wallet/txauthor v1.3.3
	github.com/btcsuite/btcwallet/wallet/txrules v1.2.0
	github.com/btcsuite/btcwallet/wallet/txsizes v1.2.3
	github.com/cenkalti/backoff/v4 v4.3.0
	github.com/cretz/bine v0.2.0
	github.com/disintegration/imaging v1.6.2
	github.com/ethereum/go-ethereum v1.16.8
	github.com/evanphx/json-patch/v5 v5.6.0
	github.com/fatih/color v1.16.0
	github.com/gagliardetto/binary v0.8.0
	github.com/gagliardetto/solana-go v1.12.0
	github.com/gcash/bchd v0.19.0
	github.com/gcash/bchutil v0.0.0-20210113190856-6ea28dff4000
	github.com/gcash/bchwallet v0.10.0
	github.com/go-resty/resty/v2 v2.15.3
	github.com/gogo/protobuf v1.3.2
	github.com/golang/protobuf v1.5.4
	github.com/google/uuid v1.6.0
	github.com/gorilla/mux v1.8.1
	github.com/gorilla/websocket v1.5.3
	github.com/gosimple/slug v1.13.1
	github.com/h2non/filetype v1.1.3
	github.com/ipfs/boxo v0.33.1
	github.com/ipfs/fs-repo-migrations/tools v0.0.0-20230628085149-3fc3a1aa00e4
	github.com/ipfs/go-cid v0.5.0
	github.com/ipfs/go-datastore v0.8.2
	github.com/ipfs/go-ipld-format v0.6.2
	github.com/ipfs/go-log v1.0.5
	github.com/ipfs/kubo v0.36.0
	github.com/ipsn/go-libtor v1.0.380
	github.com/jarcoal/httpmock v1.2.0
	github.com/jbenet/go-context v0.0.0-20150711004518-d14ea06fba99
	github.com/jessevdk/go-flags v1.5.0
	github.com/libp2p/go-libp2p v0.42.1
	github.com/libp2p/go-libp2p-kad-dht v0.33.1
	github.com/libp2p/go-libp2p-pubsub v0.14.2
	github.com/libp2p/go-libp2p-testing v0.12.0
	github.com/libp2p/go-msgio v0.3.0
	github.com/ltcsuite/ltcd v0.22.1-beta
	github.com/ltcsuite/ltcd/btcec/v2 v2.1.0
	github.com/ltcsuite/ltcd/ltcutil v1.1.0
	github.com/ltcsuite/ltcwallet/wallet/txauthor v1.1.0
	github.com/ltcsuite/ltcwallet/wallet/txrules v1.2.0
	github.com/martinboehm/btcd v0.0.0-20221101112928-408689e15809
	github.com/martinboehm/btcutil v0.0.0-20211010173611-6ef1889c1819
	github.com/microcosm-cc/bluemonday v1.0.27
	github.com/minio/blake2b-simd v0.0.0-20160723061019-3f5f724cb5b1
	github.com/mobazha/ethereum-watcher v0.0.0-20240109073302-d1cf31cdeef6
	github.com/mobazha/mobazha-core v0.0.0-00010101000000-000000000000
	github.com/multiformats/go-base32 v0.1.0
	github.com/multiformats/go-multiaddr v0.16.0
	github.com/multiformats/go-multiaddr-dns v0.4.1
	github.com/multiformats/go-multiaddr-fmt v0.1.0
	github.com/multiformats/go-multihash v0.2.3
	github.com/natefinch/lumberjack v2.0.0+incompatible
	github.com/op/go-logging v0.0.0-20160315200505-970db520ece7
	github.com/pkg/errors v0.9.1
	github.com/stretchr/testify v1.11.1
	github.com/stripe/stripe-go/v82 v82.0.0
	github.com/teserakt-io/golang-ed25519 v0.0.0-20210104091850-3888c087a4c8
	github.com/tyler-smith/go-bip39 v1.1.0
	github.com/xuri/excelize/v2 v2.10.0
	go.uber.org/fx v1.24.0
	golang.org/x/crypto v0.47.0
	golang.org/x/net v0.48.0
	google.golang.org/grpc v1.73.0
	google.golang.org/protobuf v1.36.6
	gorm.io/driver/sqlite v1.4.4
	gorm.io/gorm v1.24.5
)

require (
	bazil.org/fuse v0.0.0-20200407214033-5883e5a4b512 // indirect
	contrib.go.opencensus.io/exporter/prometheus v0.4.2 // indirect
	filippo.io/edwards25519 v1.0.0-rc.1 // indirect
	github.com/AndreasBriese/bbloom v0.0.0-20190825152654-46b345b51c96 // indirect
	github.com/DataDog/zstd v1.5.6-0.20230824185856-869dae002e5e // indirect
	github.com/Groestlcoin/go-groestl-hash v0.1.0 // indirect
	github.com/Jorropo/jsync v1.0.1 // indirect
	github.com/Microsoft/go-winio v0.6.2 // indirect
	github.com/ProjectZKM/Ziren/crates/go-runtime/zkvm_runtime v0.0.0-20251001021608-1fe7b43fc4d6 // indirect
	github.com/aead/siphash v1.0.1 // indirect
	github.com/alecthomas/units v0.0.0-20240927000941-0f3dac36c52b // indirect
	github.com/alexbrainman/goissue34681 v0.0.0-20191006012335-3fc7a47baff5 // indirect
	github.com/andres-erbsen/clock v0.0.0-20160526145045-9e14626cd129 // indirect
	github.com/anmitsu/go-shlex v0.0.0-20161002113705-648efa622239 // indirect
	github.com/aymerick/douceur v0.2.0 // indirect
	github.com/benbjohnson/clock v1.3.5 // indirect
	github.com/beorn7/perks v1.0.1 // indirect
	github.com/bits-and-blooms/bitset v1.20.0 // indirect
	github.com/blang/semver/v4 v4.0.0 // indirect
	github.com/blendle/zapdriver v1.3.1 // indirect
	github.com/btcsuite/btclog v0.0.0-20170628155309-84c8d2346e9f // indirect
	github.com/btcsuite/go-socks v0.0.0-20170105172521-4720035b7bfd // indirect
	github.com/buger/jsonparser v1.1.1 // indirect
	github.com/caddyserver/certmagic v0.21.6 // indirect
	github.com/caddyserver/zerossl v0.1.3 // indirect
	github.com/cenkalti/backoff/v5 v5.0.2 // indirect
	github.com/ceramicnetwork/go-dag-jose v0.1.1 // indirect
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/cheggaaa/pb v1.0.29 // indirect
	github.com/cockroachdb/crlib v0.0.0-20241015224233-894974b3ad94 // indirect
	github.com/cockroachdb/errors v1.11.3 // indirect
	github.com/cockroachdb/fifo v0.0.0-20240606204812-0bbfbd93a7ce // indirect
	github.com/cockroachdb/logtags v0.0.0-20230118201751-21c54148d20b // indirect
	github.com/cockroachdb/pebble/v2 v2.0.6 // indirect
	github.com/cockroachdb/redact v1.1.5 // indirect
	github.com/cockroachdb/swiss v0.0.0-20250624142022-d6e517c1d961 // indirect
	github.com/cockroachdb/tokenbucket v0.0.0-20230807174530-cc333fc44b06 // indirect
	github.com/consensys/gnark-crypto v0.18.0 // indirect
	github.com/crackcomm/go-gitignore v0.0.0-20241020182519-7843d2ba8fdf // indirect
	github.com/crate-crypto/go-eth-kzg v1.4.0 // indirect
	github.com/crate-crypto/go-ipa v0.0.0-20240724233137-53bbb0ceb27a // indirect
	github.com/cskr/pubsub v1.0.2 // indirect
	github.com/davecgh/go-spew v1.1.2-0.20180830191138-d8f796af33cc // indirect
	github.com/davidlazar/go-crypto v0.0.0-20200604182044-b73af7476f6c // indirect
	github.com/dchest/blake256 v1.1.0 // indirect
	github.com/dchest/siphash v1.2.3 // indirect
	github.com/deckarep/golang-set/v2 v2.6.0 // indirect
	github.com/decred/dcrd/crypto/blake256 v1.1.0 // indirect
	github.com/decred/dcrd/dcrec/secp256k1/v4 v4.4.0 // indirect
	github.com/dgraph-io/badger v1.6.2 // indirect
	github.com/dgraph-io/ristretto v0.1.1 // indirect
	github.com/dustin/go-humanize v1.0.1 // indirect
	github.com/elgris/jsondiff v0.0.0-20160530203242-765b5c24c302 // indirect
	github.com/ethereum/c-kzg-4844/v2 v2.1.5 // indirect
	github.com/ethereum/go-verkle v0.2.2 // indirect
	github.com/facebookgo/atomicfile v0.0.0-20151019160806-2de1f203e7d5 // indirect
	github.com/felixge/httpsnoop v1.0.4 // indirect
	github.com/filecoin-project/go-clock v0.1.0 // indirect
	github.com/flynn/noise v1.1.0 // indirect
	github.com/francoispqt/gojay v1.2.13 // indirect
	github.com/fsnotify/fsnotify v1.7.0 // indirect
	github.com/gabriel-vasile/mimetype v1.4.9 // indirect
	github.com/gagliardetto/treeout v0.1.4 // indirect
	github.com/gammazero/chanqueue v1.1.1 // indirect
	github.com/gammazero/deque v1.0.0 // indirect
	github.com/gcash/bchlog v0.0.0-20180913005452-b4f036f92fa6 // indirect
	github.com/getsentry/sentry-go v0.27.0 // indirect
	github.com/go-jose/go-jose/v4 v4.0.5 // indirect
	github.com/go-kit/log v0.2.1 // indirect
	github.com/go-logfmt/logfmt v0.6.0 // indirect
	github.com/go-logr/logr v1.4.3 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/go-ole/go-ole v1.3.0 // indirect
	github.com/go-task/slim-sprig/v3 v3.0.0 // indirect
	github.com/golang/glog v1.2.4 // indirect
	github.com/golang/groupcache v0.0.0-20210331224755-41bb18bfe9da // indirect
	github.com/golang/snappy v1.0.0 // indirect
	github.com/google/gopacket v1.1.19 // indirect
	github.com/google/pprof v0.0.0-20250607225305-033d6d78b36a // indirect
	github.com/gorilla/css v1.0.1 // indirect
	github.com/gorilla/rpc v1.2.0 // indirect
	github.com/gosimple/unidecode v1.0.1 // indirect
	github.com/grpc-ecosystem/grpc-gateway/v2 v2.27.1 // indirect
	github.com/hashicorp/go-version v1.7.0 // indirect
	github.com/hashicorp/golang-lru v1.0.2 // indirect
	github.com/hashicorp/golang-lru/v2 v2.0.7 // indirect
	github.com/holiman/uint256 v1.3.2 // indirect
	github.com/huin/goupnp v1.3.0 // indirect
	github.com/ipfs-shipyard/nopfs v0.0.14 // indirect
	github.com/ipfs-shipyard/nopfs/ipfs v0.25.0 // indirect
	github.com/ipfs/bbloom v0.0.4 // indirect
	github.com/ipfs/go-bitfield v1.1.0 // indirect
	github.com/ipfs/go-block-format v0.2.2 // indirect
	github.com/ipfs/go-cidutil v0.1.0 // indirect
	github.com/ipfs/go-ds-badger v0.3.4 // indirect
	github.com/ipfs/go-ds-flatfs v0.5.5 // indirect
	github.com/ipfs/go-ds-leveldb v0.5.2 // indirect
	github.com/ipfs/go-ds-measure v0.2.2 // indirect
	github.com/ipfs/go-ds-pebble v0.5.1 // indirect
	github.com/ipfs/go-fs-lock v0.1.1 // indirect
	github.com/ipfs/go-ipfs-cmds v0.15.0 // indirect
	github.com/ipfs/go-ipfs-delay v0.0.1 // indirect
	github.com/ipfs/go-ipfs-ds-help v1.1.1 // indirect
	github.com/ipfs/go-ipfs-pq v0.0.3 // indirect
	github.com/ipfs/go-ipfs-redirects-file v0.1.2 // indirect
	github.com/ipfs/go-ipld-cbor v0.2.0 // indirect
	github.com/ipfs/go-ipld-git v0.1.1 // indirect
	github.com/ipfs/go-ipld-legacy v0.2.2 // indirect
	github.com/ipfs/go-log/v2 v2.6.0 // indirect
	github.com/ipfs/go-metrics-interface v0.3.0 // indirect
	github.com/ipfs/go-peertaskqueue v0.8.2 // indirect
	github.com/ipfs/go-unixfsnode v1.10.1 // indirect
	github.com/ipld/go-car/v2 v2.14.3 // indirect
	github.com/ipld/go-codec-dagpb v1.7.0 // indirect
	github.com/ipld/go-ipld-prime v0.21.0 // indirect
	github.com/ipshipyard/p2p-forge v0.6.0 // indirect
	github.com/jackpal/go-nat-pmp v1.0.2 // indirect
	github.com/jbenet/go-temp-err-catcher v0.1.0 // indirect
	github.com/jbenet/goprocess v0.1.4 // indirect
	github.com/jinzhu/inflection v1.0.0 // indirect
	github.com/jinzhu/now v1.1.5 // indirect
	github.com/json-iterator/go v1.1.12 // indirect
	github.com/kkdai/bstream v1.0.0 // indirect
	github.com/klauspost/compress v1.18.0 // indirect
	github.com/klauspost/cpuid/v2 v2.2.10 // indirect
	github.com/koron/go-ssdp v0.0.6 // indirect
	github.com/kr/pretty v0.3.1 // indirect
	github.com/kr/text v0.2.0 // indirect
	github.com/libdns/libdns v0.2.2 // indirect
	github.com/libp2p/go-buffer-pool v0.1.0 // indirect
	github.com/libp2p/go-cidranger v1.1.0 // indirect
	github.com/libp2p/go-doh-resolver v0.5.0 // indirect
	github.com/libp2p/go-flow-metrics v0.3.0 // indirect
	github.com/libp2p/go-libp2p-asn-util v0.4.1 // indirect
	github.com/libp2p/go-libp2p-gostream v0.6.0 // indirect
	github.com/libp2p/go-libp2p-http v0.5.0 // indirect
	github.com/libp2p/go-libp2p-kbucket v0.7.0 // indirect
	github.com/libp2p/go-libp2p-pubsub-router v0.6.0 // indirect
	github.com/libp2p/go-libp2p-record v0.3.1 // indirect
	github.com/libp2p/go-libp2p-routing-helpers v0.7.5 // indirect
	github.com/libp2p/go-libp2p-xor v0.1.0 // indirect
	github.com/libp2p/go-netroute v0.2.2 // indirect
	github.com/libp2p/go-reuseport v0.4.0 // indirect
	github.com/libp2p/go-yamux/v5 v5.0.1 // indirect
	github.com/libp2p/zeroconf/v2 v2.2.0 // indirect
	github.com/logrusorgru/aurora v2.0.3+incompatible // indirect
	github.com/ltcsuite/ltcwallet/wallet/txsizes v1.1.0 // indirect
	github.com/marten-seemann/tcp v0.0.0-20210406111302-dfbc87cc63fd // indirect
	github.com/mattn/go-colorable v0.1.13 // indirect
	github.com/mattn/go-isatty v0.0.20 // indirect
	github.com/mattn/go-runewidth v0.0.15 // indirect
	github.com/mattn/go-sqlite3 v1.14.16 // indirect
	github.com/mgutz/ansi v0.0.0-20170206155736-9520e82c474b // indirect
	github.com/mholt/acmez/v3 v3.0.0 // indirect
	github.com/miekg/dns v1.1.67 // indirect
	github.com/mikioh/tcpinfo v0.0.0-20190314235526-30a79bb1804b // indirect
	github.com/mikioh/tcpopt v0.0.0-20190314235656-172688c1accc // indirect
	github.com/minio/sha256-simd v1.0.1 // indirect
	github.com/mitchellh/go-testing-interface v1.14.1 // indirect
	github.com/mitchellh/mapstructure v1.5.0 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.2 // indirect
	github.com/mostynb/zstdpool-freelist v0.0.0-20201229113212-927304c0c3b1 // indirect
	github.com/mr-tron/base58 v1.2.0 // indirect
	github.com/multiformats/go-base36 v0.2.0 // indirect
	github.com/multiformats/go-multibase v0.2.0 // indirect
	github.com/multiformats/go-multicodec v0.9.2 // indirect
	github.com/multiformats/go-multistream v0.6.1 // indirect
	github.com/multiformats/go-varint v0.0.7 // indirect
	github.com/munnerz/goautoneg v0.0.0-20191010083416-a7dc8b61c822 // indirect
	github.com/onsi/ginkgo/v2 v2.23.4 // indirect
	github.com/opentracing/opentracing-go v1.2.0 // indirect
	github.com/openzipkin/zipkin-go v0.4.3 // indirect
	github.com/pbnjay/memory v0.0.0-20210728143218-7b4eea64cf58 // indirect
	github.com/petar/GoLLRB v0.0.0-20210522233825-ae3b015fd3e9 // indirect
	github.com/pion/datachannel v1.5.10 // indirect
	github.com/pion/dtls/v2 v2.2.12 // indirect
	github.com/pion/dtls/v3 v3.0.6 // indirect
	github.com/pion/ice/v4 v4.0.10 // indirect
	github.com/pion/interceptor v0.1.40 // indirect
	github.com/pion/logging v0.2.3 // indirect
	github.com/pion/mdns/v2 v2.0.7 // indirect
	github.com/pion/randutil v0.1.0 // indirect
	github.com/pion/rtcp v1.2.15 // indirect
	github.com/pion/rtp v1.8.19 // indirect
	github.com/pion/sctp v1.8.39 // indirect
	github.com/pion/sdp/v3 v3.0.13 // indirect
	github.com/pion/srtp/v3 v3.0.6 // indirect
	github.com/pion/stun v0.6.1 // indirect
	github.com/pion/stun/v3 v3.0.0 // indirect
	github.com/pion/transport/v2 v2.2.10 // indirect
	github.com/pion/transport/v3 v3.0.7 // indirect
	github.com/pion/turn/v4 v4.0.2 // indirect
	github.com/pion/webrtc/v4 v4.1.2 // indirect
	github.com/pmezard/go-difflib v1.0.1-0.20181226105442-5d4384ee4fb2 // indirect
	github.com/polydawn/refmt v0.89.0 // indirect
	github.com/prometheus/client_golang v1.22.0 // indirect
	github.com/prometheus/client_model v0.6.2 // indirect
	github.com/prometheus/common v0.64.0 // indirect
	github.com/prometheus/procfs v0.16.1 // indirect
	github.com/prometheus/statsd_exporter v0.27.1 // indirect
	github.com/quic-go/qpack v0.5.1 // indirect
	github.com/quic-go/quic-go v0.52.0 // indirect
	github.com/quic-go/webtransport-go v0.8.1-0.20241018022711-4ac2c9250e66 // indirect
	github.com/richardlehane/mscfb v1.0.4 // indirect
	github.com/richardlehane/msoleps v1.0.4 // indirect
	github.com/rivo/uniseg v0.4.4 // indirect
	github.com/rogpeppe/go-internal v1.13.1 // indirect
	github.com/rs/cors v1.11.1 // indirect
	github.com/shirou/gopsutil v3.21.11+incompatible // indirect
	github.com/slok/go-http-metrics v0.13.0 // indirect
	github.com/spaolacci/murmur3 v1.1.0 // indirect
	github.com/streamingfast/logging v0.0.0-20230608130331-f22c91403091 // indirect
	github.com/stretchr/objx v0.5.2 // indirect
	github.com/supranational/blst v0.3.16-0.20250831170142-f48500c1fdbe // indirect
	github.com/syndtr/goleveldb v1.0.1-0.20220721030215-126854af5e6d // indirect
	github.com/tiendc/go-deepcopy v1.7.1 // indirect
	github.com/tklauser/go-sysconf v0.3.13 // indirect
	github.com/tklauser/numcpus v0.7.0 // indirect
	github.com/ucarion/urlpath v0.0.0-20200424170820-7ccc79b76bbb // indirect
	github.com/whyrusleeping/base32 v0.0.0-20170828182744-c30ac30633cc // indirect
	github.com/whyrusleeping/cbor v0.0.0-20171005072247-63513f603b11 // indirect
	github.com/whyrusleeping/cbor-gen v0.1.2 // indirect
	github.com/whyrusleeping/chunker v0.0.0-20181014151217-fe64bd25879f // indirect
	github.com/whyrusleeping/go-keyspace v0.0.0-20160322163242-5b898ac5add1 // indirect
	github.com/whyrusleeping/go-sysinfo v0.0.0-20190219211824-4a357d4b90b1 // indirect
	github.com/whyrusleeping/multiaddr-filter v0.0.0-20160516205228-e903e4adabd7 // indirect
	github.com/wlynxg/anet v0.0.5 // indirect
	github.com/xuri/efp v0.0.1 // indirect
	github.com/xuri/nfp v0.0.2-0.20250530014748-2ddeb826f9a9 // indirect
	github.com/yusufpapurcu/wmi v1.2.3 // indirect
	github.com/zeebo/blake3 v0.2.4 // indirect
	github.com/zquestz/grab v0.0.0-20190224022517-abcee96e61b1 // indirect
	go.mongodb.org/mongo-driver v1.12.2 // indirect
	go.opencensus.io v0.24.0 // indirect
	go.opentelemetry.io/auto/sdk v1.1.0 // indirect
	go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp v0.62.0 // indirect
	go.opentelemetry.io/otel v1.37.0 // indirect
	go.opentelemetry.io/otel/exporters/otlp/otlptrace v1.37.0 // indirect
	go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc v1.37.0 // indirect
	go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp v1.37.0 // indirect
	go.opentelemetry.io/otel/exporters/stdout/stdouttrace v1.37.0 // indirect
	go.opentelemetry.io/otel/exporters/zipkin v1.37.0 // indirect
	go.opentelemetry.io/otel/metric v1.37.0 // indirect
	go.opentelemetry.io/otel/sdk v1.37.0 // indirect
	go.opentelemetry.io/otel/trace v1.37.0 // indirect
	go.opentelemetry.io/proto/otlp v1.7.0 // indirect
	go.uber.org/automaxprocs v1.6.0 // indirect
	go.uber.org/dig v1.19.0 // indirect
	go.uber.org/mock v0.5.2 // indirect
	go.uber.org/multierr v1.11.0 // indirect
	go.uber.org/ratelimit v0.2.0 // indirect
	go.uber.org/zap v1.27.0 // indirect
	go.uber.org/zap/exp v0.3.0 // indirect
	go4.org v0.0.0-20230225012048-214862532bf5 // indirect
	golang.org/x/exp v0.0.0-20250718183923-645b1fa84792 // indirect
	golang.org/x/image v0.25.0 // indirect
	golang.org/x/mod v0.31.0 // indirect
	golang.org/x/oauth2 v0.30.0 // indirect
	golang.org/x/sync v0.19.0 // indirect
	golang.org/x/sys v0.40.0 // indirect
	golang.org/x/telemetry v0.0.0-20251203150158-8fff8a5912fc // indirect
	golang.org/x/term v0.39.0 // indirect
	golang.org/x/text v0.33.0 // indirect
	golang.org/x/time v0.12.0 // indirect
	golang.org/x/tools v0.40.0 // indirect
	golang.org/x/xerrors v0.0.0-20240903120638-7835f813f4da // indirect
	gonum.org/v1/gonum v0.16.0 // indirect
	google.golang.org/genproto v0.0.0-20230410155749-daa745c078e1 // indirect
	gopkg.in/yaml.v2 v2.4.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
	lukechampine.com/blake3 v1.4.1 // indirect
)
