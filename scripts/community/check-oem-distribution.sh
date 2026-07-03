#!/usr/bin/env bash
# SPDX-License-Identifier: MPL-2.0
# Copyright (c) 2026 fengzie and the respective contributors.

set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"

usage() {
  cat <<'EOF'
Usage:
  check-oem-distribution.sh --source
  check-oem-distribution.sh --artifact /path/to/release-bundle

--source validates the public source-tree material required before building a
Community OEM or VPS distribution. --artifact validates the non-source release
material required for a finished distribution bundle.
EOF
}

fail() {
  echo "OEM distribution check failed: $*" >&2
  exit 1
}

require_file() {
  local path="$1"
  [[ -f "$path" ]] || fail "missing required file: $path"
  [[ -s "$path" ]] || fail "required file is empty: $path"
}

require_text() {
  local path="$1"
  local text="$2"
  require_file "$path"
  grep -Fq "$text" "$path" || fail "$path is missing required text: $text"
}

require_json() {
  local path="$1"
  require_file "$path"
  python3 - "$path" <<'PY'
import json
import pathlib
import sys

path = pathlib.Path(sys.argv[1])
with path.open(encoding="utf-8") as handle:
    json.load(handle)
PY
}

mode="${1:-}"

case "$mode" in
  --source)
    [[ $# -eq 1 ]] || { usage >&2; exit 2; }

    require_file "$repo_root/LICENSE"
    require_file "$repo_root/NOTICE"
    require_file "$repo_root/SECURITY.md"
    require_file "$repo_root/TRADEMARKS.md"
    require_file "$repo_root/LICENSES/MIT-OpenBazaar.txt"
    require_file "$repo_root/config/editions/community.json"
    require_file "$repo_root/deploy/standalone/docker-compose.yml"
    require_file "$repo_root/deploy/standalone/.env.example"
    require_file "$repo_root/docs/project/RELEASE_SCOPE.md"
    require_file "$repo_root/docs/project/OEM_DISTRIBUTION.md"

    require_text "$repo_root/docs/project/OEM_DISTRIBUTION.md" "not legal advice"
    require_text "$repo_root/docs/project/OEM_DISTRIBUTION.md" "SOURCE_OFFER.md"
    require_text "$repo_root/docs/project/OEM_DISTRIBUTION.md" "Mobazha Certified"
    require_text "$repo_root/docs/project/OEM_DISTRIBUTION.md" "without a required"

    "$repo_root/scripts/community/check-capabilities.sh" "$repo_root"
    echo "OEM source-distribution material: OK"
    ;;
  --artifact)
    [[ $# -eq 2 ]] || { usage >&2; exit 2; }
    artifact_dir="$2"
    [[ -d "$artifact_dir" ]] || fail "artifact directory does not exist: $artifact_dir"

    require_file "$artifact_dir/LICENSE"
    require_file "$artifact_dir/NOTICE"
    require_file "$artifact_dir/LICENSES/MIT-OpenBazaar.txt"
    require_file "$artifact_dir/SOURCE_OFFER.md"
    require_json "$artifact_dir/SBOM.spdx.json"
    require_file "$artifact_dir/checksums.txt"
    require_json "$artifact_dir/provenance.json"
    require_file "$artifact_dir/config/editions/community.json"

    require_text "$artifact_dir/SOURCE_OFFER.md" "Mozilla Public License"
    require_text "$artifact_dir/SOURCE_OFFER.md" "https://github.com/mobazha/mobazha"
    require_text "$artifact_dir/SOURCE_OFFER.md" "Source commit"
    grep -Eq '^[[:xdigit:]]{64}[[:space:]]+' "$artifact_dir/checksums.txt" ||
      fail "checksums.txt must contain at least one SHA-256 checksum"
    cmp -s "$artifact_dir/config/editions/community.json" \
      "$repo_root/config/editions/community.json" ||
      fail "artifact Community capability manifest differs from the audited source"

    echo "OEM release artifact material: OK"
    ;;
  -h|--help|"")
    usage
    ;;
  *)
    usage >&2
    exit 2
    ;;
esac
