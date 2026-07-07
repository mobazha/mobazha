# Mobazha Node

**The open-source commerce backend for independently operated stores and connected markets.**

Mobazha Node owns store, catalog, quote, order, payment-verification,
fulfillment, recovery, messaging, and audit state. It can run on infrastructure
you control, serve an embedded storefront and administration interface, and
connect to other Mobazha experiences through versioned APIs and protocols.

[Product map](https://docs.mobazha.org/project/product-map) ·
[Install a Node](https://docs.mobazha.org/self-host/install) ·
[API reference](https://docs.mobazha.org/api-reference) ·
[Unified frontend](https://github.com/mobazha/mobazha-unified) ·
[Contributing](./CONTRIBUTING.md)

> **Release status:** the current public target is `v0.3.0-rc.1`. It is a
> release candidate for evaluation and testnet use and has not been tagged or
> published as a stable binary release. Verify the exact commit, runtime
> capabilities, and release evidence before material use.

[![Conceptual diagram of direct peer-to-peer and hybrid Mobazha store networks](https://docs.mobazha.org/images/docs/project/store-network-topologies.svg)](https://docs.mobazha.org/project/architecture)

_Independent and hosted stores can share public protocols without moving every
order into one central database. The selected seller backend remains
authoritative for its store and orders._

## Start with your goal

| Goal | Start here |
| --- | --- |
| Understand the product | [How Mobazha fits together](https://docs.mobazha.org/project/product-map) |
| Evaluate the hosted experience | [Open app.mobazha.org](https://app.mobazha.org/) |
| Run an independent store backend | [Install a Node](https://docs.mobazha.org/self-host/install) |
| Build a client or integration | [HTTP API and OpenAPI](https://docs.mobazha.org/build/api) |
| Connect an Agent or MCP client | [MCP integration guide](https://docs.mobazha.org/build/mcp) |
| Contribute to the runtime | [Development and release checks](#development-and-release-checks) |

## Where this repository fits

| Component | Responsibility | Source |
| --- | --- | --- |
| **Mobazha Node — this repository** | Commerce Core, business-state authority, persistence, payment verification, APIs, messaging, and operator controls | `mobazha/mobazha` |
| [Mobazha Unified](https://github.com/mobazha/mobazha-unified) | Storefront, checkout, seller administration, marketplace, and responsive experience surfaces | `mobazha/mobazha-unified` |
| Mobazha hosted services | Optional managed operation, routing, discovery, and other explicitly enabled services | Service-specific distributions and terms |
| [Mobazha Docs](https://docs.mobazha.org) | Canonical public product knowledge, user guidance, policy, architecture, and release scope | `mobazha/mobazha-docs` |

A channel, client, gateway, provider, or Agent can present information and
request work. The Node serving the active store or order context validates the
request and owns admitted business state.

## What the Node owns

### Commerce Core

- Store identity, profiles, policies, catalogs, listings, options, and supply
- Quotes, checkout validation, orders, fulfillment, cancellation, and refunds
- Payment instructions, observations, verification, and settlement gates
- Buyer protection, disputes, evidence, resolution, and ratings

### Connectivity and automation

- Peer-to-peer discovery, signed content, order messages, and notifications
- Versioned HTTP APIs under `/v1/` and authenticated WebSocket updates under `/ws`
- Signed webhook delivery for operator-controlled integrations
- Authenticated MCP Tool discovery and invocation under `/v1/mcp`

### Independent operation

- Embedded Web UI for storefront and seller administration
- Background-service installation and lifecycle management
- Health diagnostics, local data ownership, export, and compressed backups
- Explicit runtime capabilities so clients do not infer availability from source presence

The Community edition can operate without a Mobazha Hosting account. Optional
discovery, payment, delivery, messaging, AI, routing, or managed services remain
named dependencies with their own availability, data, and pricing boundaries.

## Operating paths

- **Self-hosted:** you run the Node and own server security, availability,
  backups, updates, network exposure, and selected integrations.
- **Hosted:** a service operator runs a compatible commercial distribution
  under its published terms while store and tenant boundaries remain explicit.
- **Hybrid:** an independent or hosted backend uses selected external services
  or participates in shared discovery and commerce protocols. Hybrid does not
  create a second owner for one order.

See [hosted and self-hosted responsibilities](https://docs.mobazha.org/start/choose-deployment)
and the [store-network architecture](https://docs.mobazha.org/project/architecture).

## Current release boundary

The default Community release boundary enables these payment methods, subject
to effective runtime capability, seller configuration, dependency health, and
the active transaction:

- Bitcoin (BTC)
- Bitcoin Cash (BCH)
- Litecoin (LTC)

Identifiers, adapters, experimental code, or documentation do not activate a
payment rail. Additional payment and extension work remains versioned and
capability-gated. Core retains order policy, verification, settlement gates,
audit, and key-custody boundaries.

Read the canonical [release scope](https://docs.mobazha.org/project/release-scope),
[runtime capability model](https://docs.mobazha.org/build/runtime-capabilities),
and repository-local [payment plugin architecture](./docs/plugins/PAYMENT_PLUGIN_ARCHITECTURE.md).

## Quick start from source

### Requirements

- Go 1.26.4
- Git
- A supported macOS or Linux development environment

Clone and build with the default pure-Go crypto implementation:

```bash
git clone https://github.com/mobazha/mobazha.git
cd mobazha
go build -tags goolm -o mobazha .
```

Initialize a testnet data directory, start the Node, and open the embedded UI:

```bash
./mobazha init --testnet
./mobazha start --testnet --open
```

The Web UI and HTTP API listen on `http://127.0.0.1:5102` by default. To use a
different data directory:

```bash
./mobazha init --testnet --datadir /path/to/mobazha-data
./mobazha start --testnet --datadir /path/to/mobazha-data --open
```

This is a source evaluation path, not a substitute for a tagged release,
signed artifacts, upgrade instructions, backup verification, or production
security review.

## Interfaces and integration contracts

| Need | Interface | Guidance |
| --- | --- | --- |
| Read state or request a protected action | HTTP `/v1/` | [API guide](https://docs.mobazha.org/build/api) |
| Refresh an interactive client | WebSocket `/ws` | [WebSocket guide](https://docs.mobazha.org/build/websocket) |
| Deliver durable operator events | Signed webhooks | [Webhook guide](https://docs.mobazha.org/build/webhooks) |
| Expose permitted Tools to an Agent | MCP `/v1/mcp` | [MCP guide](https://docs.mobazha.org/build/mcp) |

Events and notifications are refresh hints or deliveries, not independent
transaction authority. After reconnects, retries, or uncertain outcomes,
clients reconcile protected state through the owning Node.

## Operations

Install and manage the Node as a background service:

```bash
./mobazha service install
./mobazha service status
./mobazha service stop
./mobazha service start
```

Run diagnostics and create a compressed backup:

```bash
./mobazha doctor
./mobazha doctor --json
./mobazha backup --output mobazha-backup.tar.gz
```

Pre-release Docker, appliance, and standalone packaging files are under
[`deploy/standalone`](./deploy/standalone). Review exact image tags,
configuration, update, rollback, and restore behavior before relying on them.

## Development and release checks

Run the Go test suite with the default pure-Go crypto implementation:

```bash
make test
```

If native `libolm` is installed:

```bash
make test-libolm
```

Validate public capability, documentation, history, distribution, and
vulnerability boundaries:

```bash
scripts/community/check-capabilities.sh
scripts/community/check-documentation-authority.sh
scripts/community/audit-public-history.sh
scripts/community/check-oem-distribution.sh --source
scripts/community/check-vulnerabilities.sh
```

The vulnerability check requires `govulncheck` on `PATH`. Release maintainers
should also follow the SBOM and license-review process in the
[supply-chain audit](./docs/security/SUPPLY_CHAIN_AUDIT.md).

## Documentation

Canonical public knowledge:

- [Product model](https://docs.mobazha.org/project/product-map)
- [System and store-network architecture](https://docs.mobazha.org/project/architecture)
- [Release scope](https://docs.mobazha.org/project/release-scope)
- [Compatibility policy](https://docs.mobazha.org/project/compatibility)
- [Fees and economics](https://docs.mobazha.org/project/fees)
- [Packaging and distributions](https://docs.mobazha.org/project/distribution)

Repository-local implementation and release evidence:

- [Payment plugin architecture](./docs/plugins/PAYMENT_PLUGIN_ARCHITECTURE.md)
- [Supply-chain audit](./docs/security/SUPPLY_CHAIN_AUDIT.md)
- [v0.3.0-rc.1 release-candidate notes](./docs/releases/v0.3.0-rc.1.md)

## Contributing and security

Contributions are welcome. Read [CONTRIBUTING.md](./CONTRIBUTING.md), sign off
commits under the [Developer Certificate of Origin](./DCO.md), and follow the
private vulnerability-reporting process in [SECURITY.md](./SECURITY.md).

The source-code licenses do not grant rights to use Mobazha names or logos; see
[TRADEMARKS.md](./TRADEMARKS.md).

## License and attribution

Mobazha-authored source in this repository, including retained Mobazha history,
is licensed under the [Mozilla Public License 2.0](./LICENSE).

Portions derived from OpenBazaar remain available under the
[OpenBazaar MIT License](./LICENSES/MIT-OpenBazaar.txt). Third-party
dependencies and assets remain subject to their respective licenses. See
[NOTICE](./NOTICE) and
[Attribution and source identity](./docs/project/ATTRIBUTION.md).

Originally developed by [fengzie](https://github.com/fengzie) and maintained by
the Mobazha contributors. The canonical source repository is
[mobazha/mobazha](https://github.com/mobazha/mobazha).
