#!/usr/bin/env bash
set -euo pipefail

repo_root="${1:-$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)}"
root="56a8d8475522ae7570dc2984c3f87843a5e2a769"
anchor="6fb2eb1786645edee6e15b57c5d035bb5f732bef"

fail() {
  echo "ERROR: $*" >&2
  exit 1
}

actual_roots="$(git -C "${repo_root}" rev-list --max-parents=0 HEAD)"
[[ "${actual_roots}" == "${root}" ]] || fail "unexpected public root: ${actual_roots}"

git -C "${repo_root}" merge-base --is-ancestor "${anchor}" HEAD \
  || fail "reviewed source-aligned anchor is not reachable"

anchor_count="$(git -C "${repo_root}" rev-list --count "${anchor}")"
[[ "${anchor_count}" == "1835" ]] \
  || fail "reviewed anchor must contain exactly 1835 commits, found ${anchor_count}"

anchor_date="$(git -C "${repo_root}" show -s --format='%aI' "${anchor}")"
[[ "${anchor_date}" == "2026-06-28T09:44:00+08:00" ]] \
  || fail "reviewed anchor author timestamp changed: ${anchor_date}"

if [[ -n "$(git -C "${repo_root}" rev-list --merges "${anchor}..HEAD")" ]]; then
  fail "post-anchor Open Core history must remain linear"
fi

if git -C "${repo_root}" log --format='%B' "${anchor}..HEAD" \
  | grep -Eiq '^(Original-Commit:|[[:space:]]*\(cherry picked from commit )'; then
  fail "public history contains external provenance trailers"
fi

final_paths="$(mktemp)"
commit_paths="$(mktemp)"
trap 'rm -f "${final_paths}" "${commit_paths}"' EXIT
git -C "${repo_root}" ls-tree -r --name-only HEAD | LC_ALL=C sort > "${final_paths}"

while IFS= read -r commit; do
  git -C "${repo_root}" ls-tree -r --name-only "${commit}" | LC_ALL=C sort > "${commit_paths}"
  extra_path="$(comm -23 "${commit_paths}" "${final_paths}" | head -n 1)"
  [[ -z "${extra_path}" ]] \
    || fail "${commit} contains non-publishable path: ${extra_path}"
done < <(git -C "${repo_root}" rev-list "${anchor}..HEAD")

[[ ! -e "${repo_root}/.community-export.json" ]] \
  || fail "external source mapping metadata must not be published"

echo "public history: OK"
echo "  root: ${root}"
echo "  anchor: ${anchor} (${anchor_count} commits, ${anchor_date})"
echo "  source-aligned commits after anchor: $(git -C "${repo_root}" rev-list --count "${anchor}..HEAD")"
