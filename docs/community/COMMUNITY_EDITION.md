# Mobazha Community Edition

Status: Foundation implemented; external plugin runtime planned

Mobazha Community Edition is the public, self-hostable marketplace node and unified storefront. Its first release provides a complete UTXO commerce loop and a stable extension boundary for additional payment plugins.

## Included payment capabilities

- Bitcoin (BTC)
- Bitcoin Cash (BCH)
- Litecoin (LTC)
- Zcash transparent addresses (ZEC)

The machine-readable allowlist is `config/editions/community.json`. Runtime availability is the intersection of edition capabilities and capabilities actually composed and configured in the node. Once external plugins are activated, their negotiated capabilities and health add another narrowing gate. Frontends may narrow this set but never widen it.

## Not included in the first release

- Payment capabilities outside the four-chain allowlist above
- Bundled fiat payment providers

Unsupported identifiers may remain recognizable for wire/data compatibility, but they are not enabled payment capabilities and cannot create new Community Edition payments.

Legacy adapters present at the approved source-history anchor may remain for migration and protocol compatibility. Their presence in source does not enable them in Community Edition or constitute a compatibility commitment.

## License targets

- Community node and unified frontend: MPL-2.0
- Payment plugin protocol, SDK, schemas, and examples: Apache-2.0
- Individual plugins: declared per plugin
- Mobazha name and logos: governed separately from source-code licenses

The root license and third-party notices remain the authoritative legal files. Historical upstream portions retain their original notices as documented in `NOTICE`.

## Public repository history

The public repository may retain existing commits that pass source-boundary, secret, license, and reachable-object audits. If existing ancestry cannot be published safely, the public history may be rebuilt from an approved sanitized time anchor while preserving truthful source authorship and author/committer dates. Rewritten commits have new object IDs and auditable private provenance mappings.

Retaining history never expands the Community Edition capability allowlist. Only approved branches, tags, commits, trees, blobs, and other reachable Git objects may be published.

## Plugin direction

The public core owns order state, capability policy, verification, audit, and key custody. Payment plugins provide chain metadata, address validation, payment setup/observation, unsigned transaction construction, fee estimation, and settlement operations through a versioned protocol.

Plugins never receive raw seed/private-key material or import Mobazha internal packages. See `docs/plugins/PAYMENT_PLUGIN_ARCHITECTURE.md` and ADR-015.

The current foundation implements the edition manifest, a fail-closed runtime and payment-ingress allowlist, and the public plugin manifest/health registry. The first release may bundle BTC/BCH/LTC/ZEC implementations behind compatibility adapters. Process supervision and RPC remain later SDK milestones.

## Contribution boundary

Adding a chain to Community Edition requires an ADR, capability-manifest change, security review, compatibility tests, frontend support, and an explicit license decision. Implementing a plugin alone does not make that chain part of the default edition.

## Documentation language

English is the default and canonical language for documentation in this repository. Chinese is used only for a deliberately maintained Chinese edition, which should be clearly identified and linked to its corresponding English source unless the document explicitly declares a different canonical language.
