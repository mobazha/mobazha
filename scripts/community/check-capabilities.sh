#!/usr/bin/env bash
set -euo pipefail

repo_root="${1:-$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)}"
manifest="${repo_root}/config/editions/community.json"
embedded_manifest="${repo_root}/pkg/edition/manifests/community.json"

if [[ ! -f "${manifest}" ]]; then
  echo "community capability manifest not found: ${manifest}" >&2
  exit 1
fi

python3 - "${manifest}" "${embedded_manifest}" <<'PY'
import json
import pathlib
import sys

path = pathlib.Path(sys.argv[1])
data = json.loads(path.read_text(encoding="utf-8"))
embedded_path = pathlib.Path(sys.argv[2])
errors = []

if not embedded_path.is_file():
    errors.append(f"embedded edition manifest not found: {embedded_path}")
else:
    embedded = json.loads(embedded_path.read_text(encoding="utf-8"))
    if embedded != data:
        errors.append("distribution and embedded Community manifests differ")

if data.get("schemaVersion") != 1:
    errors.append("schemaVersion must be 1")
if data.get("edition") != "community":
    errors.append("edition must be community")
if data.get("license") != "MPL-2.0":
    errors.append("community core license must be MPL-2.0")
if data.get("paymentPluginSdkLicense") != "Apache-2.0":
    errors.append("plugin SDK license must be Apache-2.0")

payment = data.get("payment", {})
expected_chains = ["BTC", "BCH", "LTC"]
if set(payment) != {"chains", "rails"}:
    errors.append("payment may contain only positive chains and rails allowlists")
if payment.get("chains") != expected_chains:
    errors.append(f"payment.chains must be exactly {expected_chains!r}")
if payment.get("rails") != ["utxo_transparent"]:
    errors.append("payment.rails must be exactly ['utxo_transparent']")
if data.get("deploymentTargets") != ["standalone"]:
    errors.append("deploymentTargets must be exactly ['standalone']")

if data.get("zcash"):
    errors.append("zcash policy must be omitted when ZEC is not enabled")

if errors:
    for error in errors:
        print(f"ERROR: {error}", file=sys.stderr)
    raise SystemExit(1)

print("community capability manifest: OK")
PY

if rg -n 'github\.com/mobazha/mobazha3\.0/internal/' \
  "${repo_root}/pkg/paymentplugin" --glob '*.go'; then
  echo "ERROR: public payment plugin contract imports internal packages" >&2
  exit 1
fi

if [[ ! -f "${repo_root}/LICENSE" ]] || \
  ! rg -q '^Mozilla Public License Version 2\.0$' "${repo_root}/LICENSE"; then
  echo "ERROR: root LICENSE must contain MPL-2.0" >&2
  exit 1
fi

if [[ ! -f "${repo_root}/LICENSES/MIT-OpenBazaar.txt" ]]; then
  echo "ERROR: historical upstream MIT notice is missing" >&2
  exit 1
fi

if rg -q 'MOBAZHA_EDITION' "${repo_root}/deploy/standalone/docker-compose.yml"; then
  echo "ERROR: public deployment exposes a runtime edition escalation switch" >&2
  exit 1
fi

echo "community architecture guards: OK"

"${repo_root}/scripts/open-core/check-boundaries.sh"
