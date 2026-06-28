# Supply-chain audit baseline

Status: pre-release review

This document records the reproducible security and license checks for the initial Community Edition candidate. It is not legal advice and does not replace review of release artifacts.

## Vulnerability baseline

The baseline uses Go 1.26.4 and `govulncheck` v1.5.0 with the `goolm` build tag:

```bash
govulncheck -tags goolm ./...
```

Updating the Go toolchain, libp2p, the Kademlia DHT, QUIC/WebTransport, `x/net`, and `x/image` reduced reachable findings from 24 to one database-range discrepancy.

`GO-2024-3218` is still emitted for `github.com/libp2p/go-libp2p-kad-dht` v0.40.0 because the Go vulnerability database currently marks every version as affected. The linked GitHub Reviewed advisory, `GHSA-mqr9-hjr8-2m9w`, states that affected versions are `<= 0.20.0`. The candidate uses v0.40.0, so the publication gate records this exact mismatch and rejects every other reachable finding. Re-evaluate the exception on every dependency update and release.

Run `scripts/community/check-vulnerabilities.sh` with `govulncheck` on `PATH` to apply that gate.

## SBOM baseline

Syft v1.44.0 generated an SPDX JSON inventory from the clean publication clone. The current scan contains 338 packages and 1,394 relationships. Its 26 initial `NOASSERTION` conclusions are covered by the exact-version review manifest and resolve to zero after applying that manifest. A fresh SBOM must still be generated from the final release commit and attached to the release; a workstation path or an older generated file is not a release artifact.

Example:

```bash
syft dir:. -o spdx-json=mobazha-community.spdx.json
```

## License review

- Mobazha-authored Community Edition source, including retained Mobazha history: MPL-2.0. OpenBazaar-derived portions retain their original MIT terms; see `NOTICE` and `LICENSES/MIT-OpenBazaar.txt`.
- Syft `NOASSERTION` results are resolved only through the exact-version manifest in `config/community/license-conclusions.json`; see `docs/security/LICENSE_CONCLUSIONS.md` and `scripts/community/apply-license-conclusions.py`.
- The future payment-plugin SDK target: Apache-2.0.
- `go-ethereum` library packages are LGPL-3.0 according to the upstream project. Binary redistribution obligations and notices require explicit final review even though the related payment capability is not enabled by the Community Edition policy.
- `github.com/gagliardetto/treeout` is pinned to the first upstream revision that contains its MIT license file, avoiding the unlicensed v0.1.4 tag.
- Automated classifiers are evidence, not authority. Unknown, compound, generated, native-code, and copyleft results require inspection of the exact module revision and the final linked binary.

The `goolm` runtime license collector currently covers 225 linked Go modules and 159 unique license or notice texts with no missing module evidence. Platform-specific builds, non-Go assets, inclusion of the generated bundle in release artifacts, and the final legal review remain release blockers.
