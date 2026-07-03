# Project Governance

Mobazha is maintained by the Mobazha organization with contributions from the community.

## Decision making

- Routine fixes and documentation changes are decided through pull-request review.
- Changes to public contracts, edition capabilities, payment models, security boundaries, licensing, or repository governance require an ADR or equivalent design review before implementation.
- Core remains authoritative for edition policy, order state, payment verification, settlement gates, audit, and key custody.
- Implementing a plugin does not add its capability to the default Mobazha release.

Maintainers seek rough consensus and use technical evidence, compatibility, security, and project scope to resolve disagreements. The Mobazha organization retains final responsibility for releases, security responses, trademarks, and appointing maintainers.

## Maintainers

Maintainers can review and merge pull requests, triage issues, manage releases, and enforce project policy. Access should follow least privilege and may be removed when it is no longer required.

## Releases

Releases are built from protected tags on `main` after required checks pass. Release notes must identify compatibility changes, capability changes, security fixes, known limitations, and artifact verification instructions.
