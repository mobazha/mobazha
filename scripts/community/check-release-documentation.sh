#!/usr/bin/env bash
# SPDX-License-Identifier: MPL-2.0
# Copyright (c) 2021-2026 fengzie and the respective contributors.

set -euo pipefail

tag="${1:-${GITHUB_REF_NAME:-}}"
manifest_url="${DOCS_MANIFEST_URL:-https://docs.mobazha.org/sources.json}"
manifest_path="${DOCS_MANIFEST_PATH:-}"

fail() {
  printf 'release documentation check failed: %s\n' "$1" >&2
  exit 1
}

fetch() {
  local url="$1"
  local attempt
  for attempt in 1 2 3; do
    if curl --fail --silent --show-error --location --max-time 20 \
      --user-agent 'mobazha-node-release-docs-check/1.0' "$url"; then
      return 0
    fi
    sleep "$attempt"
  done
  return 1
}

[[ "$tag" =~ ^v[0-9]+\.[0-9]+\.[0-9]+([.-][0-9A-Za-z.-]+)?$ ]] \
  || fail "release tag is missing or invalid: ${tag:-<missing>}"

release_document="docs/releases/${tag}.md"
[[ -f "$release_document" ]] || fail "$release_document is missing"

for url in \
  'https://docs.mobazha.org/project/release-scope' \
  'https://docs.mobazha.org/self-host/install'; do
  grep --fixed-strings --quiet "$url" "$release_document" \
    || fail "$release_document is missing $url"
done

grep --fixed-strings --quiet 'https://docs.mobazha.org' README.md \
  || fail 'README.md is missing the public documentation portal'

expected_revision="${GITHUB_SHA:-$(git rev-parse HEAD)}"
if [[ -n "$manifest_path" ]]; then
  reviewed_revision="$(jq --raw-output '.sources[] | select(.id == "community-backend") | .revision' "$manifest_path")"
else
  manifest="$(fetch "$manifest_url")" \
    || fail "unable to read documentation source manifest: $manifest_url"
  reviewed_revision="$(jq --raw-output '.sources[] | select(.id == "community-backend") | .revision' <<<"$manifest")"
fi

[[ "$reviewed_revision" == "$expected_revision" ]] \
  || fail "docs source revision ${reviewed_revision:-<missing>} does not match release $expected_revision"

for url in \
  'https://docs.mobazha.org/self-host/install' \
  'https://docs.mobazha.org/project/release-scope' \
  'https://docs.mobazha.org/openapi.json'; do
  fetch "$url" >/dev/null || fail "$url is unavailable"
done

printf 'release documentation check passed: %s at %s\n' "$tag" "${expected_revision:0:12}"
