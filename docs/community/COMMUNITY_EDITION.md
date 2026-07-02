# Mobazha Community Edition

Mobazha Community Edition is the public, self-hostable marketplace node. The initial release enables Bitcoin, Bitcoin Cash, and Litecoin payments.

The machine-readable allowlist is `config/editions/community.json`. Runtime availability is the intersection of the edition allowlist and seller configuration. A frontend may narrow the set but must never widen it.

Identifiers and adapters for additional chains may remain in the source for data migration and protocol compatibility. Their presence does not enable those chains in Community Edition and does not constitute a compatibility commitment.

Mobazha-authored Community Edition source is licensed under MPL-2.0. Portions derived from OpenBazaar remain available under the OpenBazaar MIT terms; see `NOTICE` and `LICENSES/MIT-OpenBazaar.txt`. The future payment-plugin protocol, SDK, schemas, and examples are intended to use Apache-2.0. Third-party notices must be complete before the first public release.

Zcash is outside the initial release while its production monitoring and seller-settlement journey is completed. Bundled fiat providers are not enabled in the initial Community Edition runtime.

Additional payment capabilities are intended to evolve as independently
versioned, out-of-process plugins. Core retains edition policy, order state,
verification, audit, and key custody; plugins do not receive raw seed or
private-key material. See `docs/plugins/PAYMENT_PLUGIN_ARCHITECTURE.md` and
ADR-015.

English is the default language for repository documentation unless a document is explicitly maintained as a Chinese edition.
