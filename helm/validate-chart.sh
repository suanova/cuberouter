#!/usr/bin/env bash
# Validate the cube-router chart: lint + render all deployment modes.
# Usage: helm/validate-chart.sh [path-to-helm-binary]
set -euo pipefail

CHART_DIR="$(cd "$(dirname "$0")" && pwd)/cuberouter-chart"
HELM_BIN="${1:-$(command -v helm || true)}"
if [[ -z "${HELM_BIN}" ]]; then
  echo "usage: $0 [path-to-helm-binary]" >&2
  exit 1
fi

render() {
  # render <mode> <extra --set args...>
  local mode="$1"; shift
  "${HELM_BIN}" template cube-router "${CHART_DIR}" -n cube-router "$@" > /dev/null
}

echo "== helm lint =="
"${HELM_BIN}" lint "${CHART_DIR}"

echo "== helm template: default-standalone =="
render default-standalone
echo "   ok"

echo "== helm template: ha-postgres =="
render ha-postgres \
  --set postgresql.enabled=false \
  --set postgresql.crunchy.enabled=true \
  --set postgresql-operator.enabled=true
echo "   ok"

echo "== helm template: ha-redis =="
render ha-redis \
  --set redis.enabled=false \
  --set redis.operator.enabled=true \
  --set redis-operator.enabled=true
echo "   ok"

echo "== helm template: full-ha =="
render full-ha \
  --set postgresql.enabled=false \
  --set postgresql.crunchy.enabled=true \
  --set postgresql-operator.enabled=true \
  --set redis.enabled=false \
  --set redis.operator.enabled=true \
  --set redis-operator.enabled=true
echo "   ok"

echo "== helm template: external-deps =="
render external-deps \
  --set postgresql.enabled=false \
  --set redis.enabled=false \
  --set secrets.SQL_DSN=postgresql://u:p@db:5432/d \
  --set secrets.REDIS_CONN_STRING=redis://:p@r:6379
echo "   ok"

echo "== negative: no pg mode without SQL_DSN must fail =="
if render no-pg-mode --set postgresql.enabled=false 2> /dev/null; then
  echo "   FAIL: expected render error" >&2
  exit 1
fi
echo "   ok (render correctly rejected)"

echo "all checks passed"
