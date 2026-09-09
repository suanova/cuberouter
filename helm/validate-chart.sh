#!/usr/bin/env bash
# Validate the cuberouter chart: lint + render all deployment modes.
# Usage: helm/validate-chart.sh [path-to-helm-binary]
set -euo pipefail

CHART_DIR="$(cd "$(dirname "$0")" && pwd)/cuberouter-chart"
HELM_BIN="${1:-$(command -v helm || true)}"
if [[ -z "${HELM_BIN}" ]]; then
  echo "usage: $0 [path-to-helm-binary]" >&2
  exit 1
fi

render() {
  local label="$1"; shift
  "${HELM_BIN}" template cuberouter "${CHART_DIR}" -n cuberouter "$@" > /dev/null
}

echo "== helm lint =="
"${HELM_BIN}" lint "${CHART_DIR}"

echo "== helm template: default (full HA, zero flags) =="
render default
echo "   ok"

echo "== helm template: existingSecret (0.6.0 pattern) =="
render existing-secret \
  --set secret.create=false \
  --set secret.existingSecret=legacy-app-secret
echo "   ok"

echo "== negative: password with space must fail =="
if render neg-pw --set "secrets.POSTGRES_PASSWORD=bad password" 2> /dev/null; then
  echo "   FAIL: expected render error" >&2; exit 1
fi
echo "   ok (render correctly rejected)"

echo "all checks passed"
