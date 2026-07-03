# Reviewed License Conclusions

Syft may emit `NOASSERTION` when a Go module license is outside the nested
module directory, uses an unexpected filename, or is expressed in source-file
headers. The Mobazha release process does not silently accept those
results.

The reviewed conclusions are stored in
`config/community/license-conclusions.json`. Each entry is pinned to an exact
package name and version and records the evidence used by the review. Apply the
manifest to a fresh Syft SPDX JSON document with:

```bash
python3 scripts/community/apply-license-conclusions.py \
  /path/to/syft.spdx.json \
  /path/to/reviewed.spdx.json
```

The command fails when:

- a `NOASSERTION` package has no exact reviewed entry;
- a reviewed entry is absent from the input SBOM;
- the conclusion manifest contains an incomplete or duplicate entry.

For the initial candidate, the review resolves 25 exact Go module versions and
the repository root package. The notable compound conclusions are:

- the repository root: MPL-2.0 for Mobazha-authored source and MIT for the
  OpenBazaar-derived portions described by `NOTICE`;
- Ziren zkVM runtime: Apache-2.0 or MIT, matching the two upstream license files
  at the pinned commit;
- `blake256`: CC0-1.0, as declared by its source headers;
- `go-libtor`: BSD-3-Clause together with the bundled OpenSSL and zlib terms;
- `modernc.org/libc`: BSD-3-Clause together with the included musl MIT terms.

License conclusions are evidence for the SPDX inventory; they do not replace
the obligation to ship required copyright notices and license texts with a
binary release. Generate the deterministic Go module license bundle in a new
directory with:

```bash
python3 scripts/community/collect-go-module-licenses.py \
  /path/to/third-party-licenses
```

The collector follows the exact release binary dependency graph from
`go list -deps -tags goolm .`, deduplicates identical texts by SHA-256,
includes reviewed nested or upstream license files, and fails if any linked
module has no license evidence. Use `--package` and `--tags` for each additional
release target or platform-specific build. The final artifact must include the
matching bundles and must be reviewed again whenever a dependency version
changes.
