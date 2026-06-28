#!/usr/bin/env bash
set -euo pipefail

repo_root="${1:-$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)}"

if ! command -v govulncheck >/dev/null 2>&1; then
  echo "govulncheck is required" >&2
  exit 2
fi

report="$(mktemp)"
cleanup() {
  rm -f "${report}"
}
trap cleanup EXIT

set +e
(
  cd "${repo_root}"
  govulncheck -tags goolm ./...
) >"${report}" 2>&1
scan_status=$?
set -e

if (( scan_status == 0 )); then
  cat "${report}"
  echo "community vulnerability boundary: OK"
  exit 0
fi

ids="$(rg -o 'GO-[0-9]{4}-[0-9]+' "${report}" | sort -u | tr '\n' ' ' | sed 's/ $//')"
if [[ "${ids}" != "GO-2024-3218" ]]; then
  cat "${report}" >&2
  echo "community vulnerability boundary: FAILED (reachable findings: ${ids:-unknown})" >&2
  exit 1
fi

dht_version="$(cd "${repo_root}" && go list -m -f '{{.Version}}' github.com/libp2p/go-libp2p-kad-dht)"
python3 - "${dht_version}" <<'PY'
import re
import sys

match = re.fullmatch(r"v(\d+)\.(\d+)\.(\d+)", sys.argv[1])
if not match or tuple(map(int, match.groups())) <= (0, 20, 0):
    raise SystemExit(f"DHT version is within or cannot be compared to the reviewed affected range: {sys.argv[1]}")
PY

echo "community vulnerability boundary: OK with documented GO-2024-3218 database-range exception (${dht_version})"
