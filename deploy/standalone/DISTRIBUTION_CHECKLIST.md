# Standalone distribution artifact checklist

This implementation-local checklist defines the files verified by
`scripts/community/check-oem-distribution.sh` for an exact source revision and
release bundle. Public distribution policy lives at
<https://docs.mobazha.org/project/distribution>.

## Source-tree inputs

- `LICENSE`, `NOTICE`, `SECURITY.md`, `TRADEMARKS.md`, and the OpenBazaar MIT notice.
- `config/editions/community.json` for the exact capability allowlist.
- Standalone Compose and example environment configuration.
- The canonical public distribution-policy link.

## Release-bundle outputs

- `SOURCE_OFFER.md` naming the canonical repository and exact tag or commit.
- An SPDX SBOM, SHA-256 checksums, and machine-readable provenance.
- The exact Community capability manifest used to build the artifact.
- License and notice material required by the distributed files.
- Version-specific upgrade, backup, restore, reset, and export guidance supplied by the release process.

Run the source and artifact checks with:

```bash
./scripts/community/check-oem-distribution.sh --source
./scripts/community/check-oem-distribution.sh --artifact /path/to/release-bundle
```
