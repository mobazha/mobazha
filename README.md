# Mobazha Community Edition

Mobazha Community Edition is a self-hostable, peer-to-peer marketplace node. It provides store management, listings, orders, fulfillment, disputes, ratings, notifications, wallet integration, and a browser-based administration experience from one node.

Originally developed by [fengzie](https://github.com/fengzie) and maintained by
the Mobazha contributors. The canonical source repository is
[mobazha/mobazha3.0](https://github.com/mobazha/mobazha3.0).

> **Status:** This repository is a pre-release Community Edition candidate. Security, dependency-license, packaging, and release-process reviews are still in progress.

## Included payment capabilities

The initial Community Edition runtime enables:

- Bitcoin (BTC)
- Bitcoin Cash (BCH)
- Litecoin (LTC)

The authoritative allowlist is [`config/editions/community.json`](config/editions/community.json). Runtime availability is the intersection of the edition allowlist and seller configuration. Clients may narrow this set, but they must never widen it.

Identifiers or adapters for other chains may remain in the repository for historical-data migration and protocol compatibility. Their presence does not enable those chains and is not a compatibility or support commitment. Bundled fiat payment providers are not enabled in the Community Edition runtime.

## Architecture

The node is organized around explicit application services for marketplace, order, payment, and settlement workflows. The Community Edition policy remains authoritative for capability registration, API projection, validation, and wallet startup.

Additional payment capabilities are planned as independently versioned,
out-of-process plugins. Core keeps policy enforcement, order state,
verification, audit, and key custody. Plugins must not receive raw seed phrases
or private keys.

See:

- [Community Edition scope](docs/community/COMMUNITY_EDITION.md)
- [OEM and VPS distribution](docs/community/OEM_DISTRIBUTION.md)
- [Public repository history](docs/community/PUBLIC_HISTORY.md)
- [Payment plugin architecture](docs/plugins/PAYMENT_PLUGIN_ARCHITECTURE.md)
- [ADR-015: payment plugin boundary](docs/adr/015-payment-plugin-boundary.md)
- [ADR-016: in-process first-party distribution composition](docs/adr/016-in-process-distribution-composition.md)
- [ADR-017: Community v0.3 payment chain scope](docs/adr/017-community-v0.3-chain-scope.md)
- [Supply-chain audit baseline](docs/security/SUPPLY_CHAIN_AUDIT.md)

## Requirements

- Go 1.26.4
- Git
- A supported macOS or Linux development environment

The default test configuration uses the pure-Go `goolm` implementation. Native `libolm` is optional and is only required for the dedicated native-libolm build or test path.

## Build from source

```bash
git clone https://github.com/mobazha/mobazha3.0.git
cd mobazha3.0
go build -tags goolm -o mobazha .
```

Start the node and open the embedded Web UI:

```bash
./mobazha start --open
```

The first start initializes the default data directory automatically. By default, the Web UI and HTTP API listen on `http://127.0.0.1:5102`, with API routes under `/v1/`.

To initialize a custom data directory explicitly:

```bash
./mobazha init --datadir /path/to/mobazha-data
./mobazha start --datadir /path/to/mobazha-data --open
```

Use testnet while evaluating payment flows:

```bash
./mobazha init --testnet
./mobazha start --testnet --open
```

## Operations

Install and manage the node as a background service on Linux or macOS:

```bash
./mobazha service install
./mobazha service status
./mobazha service stop
./mobazha service start
```

Run environment and node diagnostics:

```bash
./mobazha doctor
./mobazha doctor --json
```

Create a compressed backup of the node data directory:

```bash
./mobazha backup --output mobazha-backup.tar.gz
```

Standalone Docker deployment files are available under [`deploy/standalone`](deploy/standalone).

## Testing and release checks

Run the full Go test suite with the default pure-Go crypto implementation:

```bash
make test
```

If native `libolm` is installed, run the native path with:

```bash
make test-libolm
```

Validate the Community Edition boundary:

```bash
scripts/community/check-capabilities.sh
scripts/community/audit-public-history.sh
scripts/community/check-oem-distribution.sh --source
scripts/community/check-vulnerabilities.sh
```

The vulnerability check requires `govulncheck` on `PATH`.

Apply reviewed license conclusions to a fresh Syft SPDX JSON SBOM and collect
the license texts required by the default Go release target:

```bash
python3 scripts/community/apply-license-conclusions.py \
  mobazha-community.spdx.json \
  mobazha-community-reviewed.spdx.json
python3 scripts/community/collect-go-module-licenses.py \
  third-party-licenses
```

## Licensing

Mobazha-authored source in this repository, including retained Mobazha history, is licensed under the [Mozilla Public License 2.0](LICENSE).

Portions derived from OpenBazaar remain available under the [OpenBazaar MIT License](LICENSES/MIT-OpenBazaar.txt). Third-party dependencies and assets remain subject to their respective licenses. See [NOTICE](NOTICE) for attribution and scope.

See [Attribution and source identity](docs/community/ATTRIBUTION.md) for the
canonical project origin, source-header policy, and fork requirements.

## Contributing and security

Contributions are welcome. Before opening a pull request, read [CONTRIBUTING.md](CONTRIBUTING.md) and sign off commits under the [Developer Certificate of Origin](DCO.md). Report security issues privately as described in [SECURITY.md](SECURITY.md).

The source-code licenses do not grant rights to use Mobazha names or logos. See [TRADEMARKS.md](TRADEMARKS.md).
