# Open Core Public History

The public repository begins with the reviewed Open Core source snapshot. The
private development and extraction histories are intentionally not ancestors
of the public root.

## Invariants

- The public history has exactly one root.
- Every reachable commit, tree, path, message, and blob is publishable.
- Commit messages do not contain private provenance trailers.
- Source-mapping manifests, Git notes, replace refs, and private refs are not
  part of the publication repository.
- Attribution is carried by `NOTICE`, the repository licenses, and the normal
  authorship of commits created after publication.

The verification script checks these structural invariants without binding the
repository to a particular root hash, commit count, release name, or source
repository revision.
