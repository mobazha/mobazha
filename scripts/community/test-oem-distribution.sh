#!/usr/bin/env bash
# SPDX-License-Identifier: MPL-2.0
# Copyright (c) 2026 fengzie and the respective contributors.

set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
artifact_dir="$(mktemp -d)"

cleanup() {
  rm -rf "$artifact_dir"
}
trap cleanup EXIT

mkdir -p "$artifact_dir/LICENSES" "$artifact_dir/config/editions"
cp "$repo_root/LICENSE" "$artifact_dir/LICENSE"
cp "$repo_root/NOTICE" "$artifact_dir/NOTICE"
cp "$repo_root/LICENSES/MIT-OpenBazaar.txt" \
  "$artifact_dir/LICENSES/MIT-OpenBazaar.txt"
cp "$repo_root/config/editions/community.json" \
  "$artifact_dir/config/editions/community.json"

printf '%s\n' \
  '# Source offer' \
  '' \
  'This distribution contains Mozilla Public License covered software.' \
  'Canonical source: https://github.com/mobazha/mobazha3.0' \
  'Source commit: test-fixture' > "$artifact_dir/SOURCE_OFFER.md"
printf '%s\n' '{"spdxVersion":"SPDX-2.3"}' > "$artifact_dir/SBOM.spdx.json"
printf '%s\n' '{"predicateType":"https://slsa.dev/provenance/v1"}' \
  > "$artifact_dir/provenance.json"
printf '%s\n' \
  '0000000000000000000000000000000000000000000000000000000000000000  fixture' \
  > "$artifact_dir/checksums.txt"

"$repo_root/scripts/community/check-oem-distribution.sh" --artifact "$artifact_dir"
