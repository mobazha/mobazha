# Mobazha 3.0

Mobazha 3.0 is the open-source commerce core and self-hosted distribution of Mobazha. It provides marketplace, order, messaging, wallet, payment, and embedded storefront services in one Go application.

## Open source and self-hosted distribution

This repository is the public upstream for shared Mobazha domain logic, application services, public APIs, capability policy, and self-hosted operation. It is not a reduced commercial fork. Hosted and commercial products use separate compositions and release pipelines while conforming to the shared public contracts.

The public self-hosted release uses the `community` distribution profile. An explicit, server-side distribution policy narrows the capabilities that a release may expose; frontend configuration and installed extensions may narrow that set further, but cannot widen it. `community` is a packaging and policy identifier, not the identity of the core domain model.

The first public self-hosted payment allowlist is:

- Bitcoin (BTC)
- Bitcoin Cash (BCH)
- Litecoin (LTC)

The canonical machine-readable policy is [`config/editions/community.json`](config/editions/community.json). Scope, history, licensing, and extension boundaries are described in [`docs/community/COMMUNITY_EDITION.md`](docs/community/COMMUNITY_EDITION.md). Public API and cross-distribution compatibility commitments are documented in [`docs/community/COMPATIBILITY.md`](docs/community/COMPATIBILITY.md).

## Architecture

- `cmd/` — CLI commands and process entry points
- `internal/` — application composition, APIs, marketplace domain, storage, networking, and bundled implementations
- `pkg/` — public contracts and reusable packages
- `libs/` — embedded libraries maintained with the node
- `mobile/` — mobile bindings and integration code
- `deploy/` — deployment assets
- `docs/` — architecture decisions and operator/developer documentation

Payment extensions use a versioned, provider-neutral boundary. See [`docs/plugins/PAYMENT_PLUGIN_ARCHITECTURE.md`](docs/plugins/PAYMENT_PLUGIN_ARCHITECTURE.md) and [`docs/adr/015-payment-plugin-boundary.md`](docs/adr/015-payment-plugin-boundary.md).

First-party commercial products compose the same Node runtime through an
explicit distribution profile; they do not replace Core files with product
build tags. See [`docs/adr/016-in-process-distribution-composition.md`](docs/adr/016-in-process-distribution-composition.md).

## Requirements

- Go 1.25.5 or newer
- Git

## Build and run

```bash
go build -tags goolm -o mobazha .
./mobazha start
```

On first start, the node initializes its data directory and serves the web interface at `http://localhost:5102` by default.

To run as a background service:

```bash
mobazha service install
mobazha service status
```

Self-hosted deployment assets set `MOBAZHA_EDITION=community`. Every packaged distribution should select its policy explicitly; business logic should consume concrete capabilities rather than branch on the distribution name.

## Test

```bash
make test
./scripts/community/check-capabilities.sh
```

## License

Mobazha-authored source in this repository is licensed under the [Mozilla Public License 2.0](LICENSE). Historical upstream components retain their original licenses and notices; see [NOTICE](NOTICE) and vendored dependency license files. The payment plugin protocol/SDK may be distributed under Apache-2.0 where explicitly marked.
