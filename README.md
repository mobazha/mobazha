# Mobazha 3.0

Mobazha 3.0 is a self-hostable, decentralized marketplace node. It provides marketplace, order, messaging, wallet, payment, and embedded storefront services in one Go application.

## Community Edition

The Community Edition is built from the same backend architecture as other Mobazha editions. An explicit, server-side edition policy narrows the capabilities that a distribution may expose; frontend configuration and installed extensions may narrow that set further, but cannot widen it.

The first Community Edition payment allowlist is:

- Bitcoin (BTC)
- Bitcoin Cash (BCH)
- Litecoin (LTC)
- Zcash transparent addresses (ZEC)

The canonical machine-readable policy is [`config/editions/community.json`](config/editions/community.json). Scope, history, licensing, and extension boundaries are described in [`docs/community/COMMUNITY_EDITION.md`](docs/community/COMMUNITY_EDITION.md).

## Architecture

- `cmd/` — CLI commands and process entry points
- `internal/` — application composition, APIs, marketplace domain, storage, networking, and bundled implementations
- `pkg/` — public contracts and reusable packages
- `libs/` — embedded libraries maintained with the node
- `mobile/` — mobile bindings and integration code
- `deploy/` — deployment assets
- `docs/` — architecture decisions and operator/developer documentation

Payment extensions use a versioned, provider-neutral boundary. See [`docs/plugins/PAYMENT_PLUGIN_ARCHITECTURE.md`](docs/plugins/PAYMENT_PLUGIN_ARCHITECTURE.md) and [`docs/adr/015-payment-plugin-boundary.md`](docs/adr/015-payment-plugin-boundary.md).

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

Community deployment assets set `MOBAZHA_EDITION=community`. Other existing installations default to the unrestricted composition for backward compatibility and should set their edition explicitly when packaging a release.

## Test

```bash
make test
./scripts/community/check-capabilities.sh
```

## License

Mobazha-authored source in this repository is licensed under the [Mozilla Public License 2.0](LICENSE). Historical upstream components retain their original licenses and notices; see [NOTICE](NOTICE) and vendored dependency license files. The payment plugin protocol/SDK may be distributed under Apache-2.0 where explicitly marked.
