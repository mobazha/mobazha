# Mobazha

**Open-source peer-to-peer commerce for storefronts, marketplaces, checkout,
and seller operations.**

Mobazha is an open-source commerce platform for self-hosted stores and hosted
deployments. This repository contains the core Go runtime, Mobazha Node, which
combines catalog and order management, fulfillment, disputes, ratings,
messaging, wallet-backed payment monitoring, APIs, and an embedded browser
interface.

[Quick Start](#quick-start) · [Operations](#operations) ·
[Frontend](https://github.com/mobazha/mobazha-unified) ·
[Contributing](./CONTRIBUTING.md)

> **Status:** v0.3 is a release candidate intended for evaluation and testnet
> use. Stable binaries and signed release artifacts have not been published
> yet.

## Why run a Mobazha node?

- **Own your store** — keep your catalog, orders, customer interactions, and
  operational data on infrastructure you control.
- **Run complete commerce workflows** — manage listings, checkout,
  fulfillment, refunds, disputes, and ratings from one node.
- **Accept wallet-backed payments** — monitor and verify supported
  cryptocurrency payments without a platform-controlled checkout.
- **Connect directly** — participate in peer-to-peer discovery, order
  messaging, and notifications.
- **Automate operations** — integrate through HTTP, WebSocket, and MCP APIs.
- **Operate with confidence** — use built-in service management, diagnostics,
  and compressed backups.

A local standalone store remains usable for administration, listings, data
export, and supported UTXO payment flows without requiring a Mobazha Hosting
account. Optional services can add discovery, search, routing, managed updates,
or support.

## What is included?

### Store operations

- Store profile and storefront configuration
- Product listings and shipping profiles
- Order, fulfillment, shipping, cancellation, and refund workflows
- Browser-based seller administration

### Trust and communication

- Buyer and seller order state management
- Payment monitoring and verification
- Disputes, evidence, resolution, and ratings
- Durable peer-to-peer messages and notifications

### Platform capabilities

- Embedded Web UI
- Versioned HTTP API and WebSocket events
- MCP endpoint for agent and automation integrations
- Background service installation and lifecycle management
- Health diagnostics and local backups

## Quick Start

### Requirements

- Go 1.26.4
- Git
- A supported macOS or Linux development environment

Clone and build the node with the default pure-Go crypto implementation:

```bash
git clone https://github.com/mobazha/mobazha.git
cd mobazha
go build -tags goolm -o mobazha .
```

Start the node and open the embedded Web UI:

```bash
./mobazha start --open
```

The first start initializes the default data directory automatically. The Web
UI and HTTP API listen on `http://127.0.0.1:5102` by default, with API routes
under `/v1/`.

Use testnet while evaluating payment flows:

```bash
./mobazha init --testnet
./mobazha start --testnet --open
```

To use a custom data directory:

```bash
./mobazha init --datadir /path/to/mobazha-data
./mobazha start --datadir /path/to/mobazha-data --open
```

## Payments and release scope

The first open-source release enables these payment methods by default:

- Bitcoin (BTC)
- Bitcoin Cash (BCH)
- Litecoin (LTC)

Runtime availability also depends on the seller configuration. Frontends may
narrow the methods reported by the node, but they cannot enable a method the
node did not advertise.

Additional payment extensions are being designed around a versioned plugin
boundary. The public protocol is under development and should not yet be
treated as a stable plugin runtime. Core remains responsible for policy, order
state, verification, audit, settlement gates, and key custody; plugins must not
receive raw seed phrases or private keys.

See [release scope](./docs/community/COMMUNITY_EDITION.md) and
[payment plugin architecture](./docs/plugins/PAYMENT_PLUGIN_ARCHITECTURE.md).

## APIs and integrations

Mobazha Node exposes its commerce capabilities through:

- HTTP APIs under `/v1/`
- WebSocket connections under `/ws`
- MCP Streamable HTTP under `/v1/mcp`
- The shared [Mobazha Unified](https://github.com/mobazha/mobazha-unified)
  storefront and seller interface

The node is authoritative for capabilities, order state, payment verification,
settlement, audit, and wallet operations. Clients render only the capabilities
reported by the connected node.

## Operations

Install and manage the node as a background service on Linux or macOS:

```bash
./mobazha service install
./mobazha service status
./mobazha service stop
./mobazha service start
```

Run diagnostics:

```bash
./mobazha doctor
./mobazha doctor --json
```

Create a compressed backup of the node data directory:

```bash
./mobazha backup --output mobazha-backup.tar.gz
```

Pre-release Docker, appliance, and standalone packaging files are available
under [`deploy/standalone`](./deploy/standalone). Review the exact image tag,
configuration, upgrade, and recovery instructions before using them outside a
test environment.

## Architecture and release documentation

- [Release scope](./docs/community/COMMUNITY_EDITION.md)
- [Compatibility policy](./docs/community/COMPATIBILITY.md)
- [OEM and VPS distribution](./docs/community/OEM_DISTRIBUTION.md)
- [Payment plugin architecture](./docs/plugins/PAYMENT_PLUGIN_ARCHITECTURE.md)
- [Supply-chain audit](./docs/security/SUPPLY_CHAIN_AUDIT.md)
- [v0.3 release candidate notes](./docs/releases/v0.3.0-community.1.md)

## Development and release checks

Run the full Go test suite with the default pure-Go crypto implementation:

```bash
make test
```

If native `libolm` is installed, run the native path with:

```bash
make test-libolm
```

Validate the current public-release boundary:

```bash
scripts/community/check-capabilities.sh
scripts/community/audit-public-history.sh
scripts/community/check-oem-distribution.sh --source
scripts/community/check-vulnerabilities.sh
```

The vulnerability check requires `govulncheck` on `PATH`. Release maintainers
should also follow the SBOM and license-review process in the
[supply-chain audit](./docs/security/SUPPLY_CHAIN_AUDIT.md).

## License and attribution

Mobazha-authored source in this repository, including retained Mobazha history,
is licensed under the [Mozilla Public License 2.0](./LICENSE).

Portions derived from OpenBazaar remain available under the
[OpenBazaar MIT License](./LICENSES/MIT-OpenBazaar.txt). Third-party
dependencies and assets remain subject to their respective licenses. See
[NOTICE](./NOTICE) and
[Attribution and source identity](./docs/community/ATTRIBUTION.md) for details.

Originally developed by [fengzie](https://github.com/fengzie) and maintained by
the Mobazha contributors. The canonical source repository is
[mobazha/mobazha](https://github.com/mobazha/mobazha).

## Contributing and security

Contributions are welcome. Before opening a pull request, read
[CONTRIBUTING.md](./CONTRIBUTING.md) and sign off commits under the
[Developer Certificate of Origin](./DCO.md).

Report security issues privately as described in [SECURITY.md](./SECURITY.md).
The source-code licenses do not grant rights to use Mobazha names or logos; see
[TRADEMARKS.md](./TRADEMARKS.md).
