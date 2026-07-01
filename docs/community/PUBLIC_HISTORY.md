# Open Core Public History

The public repository preserves the authoritative development history while
projecting every commit onto the paths retained by Open Core. This keeps the
origin, ordering, author timestamps, committer timestamps, and explanatory
commit bodies without publishing provider-only files.

## Invariants

- The retained history starts at root `56a8d8475522ae7570dc2984c3f87843a5e2a769`.
- Commit `7ca9f834091f4176a2e9b68fd8f1f0aa673b4a6d` is the reviewed source-aligned
  anchor for the final Open Core extraction series.
- The anchor contains exactly 1,835 commits and keeps its original author
  timestamp, `2026-06-28T09:44:00+08:00`.
- History after the anchor is linear and contains no external provenance
  trailers.
- Commit subjects and bodies use distribution-neutral terminology.
- Every path reachable after the anchor is present in the final publishable
  tree; provider-only and superseded paths cannot reappear in intermediate
  snapshots.

The repository intentionally does not publish source-mapping manifests or
references to another repository. Reconstructed commit identifiers are an
implementation detail of the path projection; authorship and timestamps are
the audit identity.
