#!/usr/bin/env bash
set -euo pipefail

repo_root="${1:-$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)}"

fail() {
  echo "ERROR: $*" >&2
  exit 1
}

git -C "${repo_root}" rev-parse --is-inside-work-tree >/dev/null 2>&1 \
  || fail "not a Git work tree: ${repo_root}"

head="$(git -C "${repo_root}" rev-parse --verify HEAD)"
roots="$(git -C "${repo_root}" rev-list --max-parents=0 HEAD)"
root_count="$(printf '%s\n' "${roots}" | sed '/^$/d' | wc -l | tr -d ' ')"
[[ "${root_count}" == "1" ]] || fail "public history must have exactly one root, found ${root_count}"

if git -C "${repo_root}" log --format='%B' HEAD \
  | grep -Eiq '^(Original-Commit:|[[:space:]]*\(cherry picked from commit )'; then
  fail "public history contains private provenance trailers"
fi

if git -C "${repo_root}" ls-files \
  | grep -Eiq '(^|/)(\.community-export\.json|source[-_]?map|commit[-_]?map|extraction[-_]?provenance)(\.|/|$)'; then
  fail "source-mapping metadata must not be published"
fi
[[ ! -e "${repo_root}/.gitmodules" ]] \
  || fail "submodule references require a separate publication review"

if git -C "${repo_root}" for-each-ref --format='%(refname)' refs/replace refs/notes \
  | grep -q .; then
  fail "replace or notes refs are not allowed in the publication repository"
fi

if git -C "${repo_root}" for-each-ref --format='%(refname)' \
  | grep -Eiq '^refs/(original|codex-backup)/|^refs/remotes/(archive|audit-|private)(/|$)'; then
  fail "private, archive, or history-rewrite refs are not allowed in the publication repository"
fi

git -C "${repo_root}" fsck --full --no-dangling >/dev/null

echo "public history: OK"
echo "  root: ${roots}"
echo "  head: ${head}"
echo "  commits: $(git -C "${repo_root}" rev-list --count HEAD)"
