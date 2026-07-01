# Contributing to Mobazha Community Edition

Thank you for helping improve Mobazha Community Edition.

## Before you start

- Use GitHub Issues or Discussions for proposals and user questions.
- Open an issue before large architectural changes so maintainers and contributors can agree on scope.
- Do not include credentials, private endpoints, customer data, proprietary code, or generated binaries.
- Report vulnerabilities privately according to `SECURITY.md` instead of opening a public issue.

## Community payment boundary

The default Community Edition payment allowlist is BTC, BCH, and LTC. A new chain or payment model requires an ADR, capability-manifest change, threat review, compatibility and negative tests, frontend support, and an explicit license decision.

Payment plugins must use the public versioned contract. They must not import `internal/`, receive `MobazhaNode`, access raw seed or private-key material, or bypass Core payment-verification and settlement gates.

## Development workflow

1. Create a focused branch from `main`.
2. Add or update tests with the implementation.
3. Run:

   ```bash
   ./scripts/community/check-capabilities.sh
   go test -tags goolm ./...
   go vet -tags goolm ./...
   go build -tags goolm ./...
   ```

4. Keep commits reviewable and use a concise conventional subject such as `fix(wallet): reject unsupported community coin`.
5. Update public documentation when behavior, API contracts, configuration, or security assumptions change.

## License and attribution headers

New first-party source files must carry an SPDX license identifier near the top
of the file:

```text
SPDX-License-Identifier: MPL-2.0
```

Use the comment syntax of the language. Mobazha-originated files may also carry
an accurate copyright notice for `fengzie and the respective contributors`.
Contributors retain copyright in their own contributions unless a separate
written agreement says otherwise, and may add accurate notices in their own
name. Do not remove existing Mobazha, contributor, OpenBazaar, or third-party
notices. See `docs/community/ATTRIBUTION.md`.

## Developer Certificate of Origin

Every commit must be signed off to certify the Developer Certificate of Origin in `DCO.md`:

```bash
git commit -s -m "fix(scope): concise description"
```

The sign-off must use a name and email address you are authorized to contribute under. Pull requests with missing sign-offs may be asked to amend their commits.

## Review expectations

Maintainers review correctness, tests, compatibility, security boundaries, licensing, and documentation. A contribution may be declined when it widens the default edition without an accepted design decision or creates a dependency from the public Core into a private implementation.
