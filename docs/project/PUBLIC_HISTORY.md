# Mobazha Public History

The public repository preserves a reviewed, sanitized projection of the
project's development history from its original root. Source-repository commit
objects and private-only commits are not ancestors of the public branch;
publishable counterparts retain the original topology and author/committer
identity and timestamps. Publication-specific fixes may extend that history.

## Invariants

- The public history has exactly one root.
- Every reachable commit, tree, path, message, and blob is publishable.
- Commit messages do not contain private provenance trailers.
- Source-mapping manifests, Git notes, replace refs, and private refs are not
  part of the publication repository.
- Attribution is carried by `NOTICE`, the repository licenses, and the
  preserved authorship of publishable commits.
- Commit maps and other source-to-public provenance records remain in the
  private release archive rather than the publication repository.

`scripts/community/audit-public-history.sh` verifies topology and repository
metadata hygiene. Content safety is verified separately by the architecture
boundary, secret, license, and test gates. These checks intentionally avoid
binding publication to a particular root hash, commit count, release name, or
source-repository revision.
