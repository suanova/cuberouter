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

# Assertions grep the rendered manifest from a file rather than a pipe: with
# `set -o pipefail` an early-exiting `grep -q` can SIGPIPE the writer and turn
# a real match into a non-zero pipeline status, which would silently skip the
# check below.
MS_MANIFEST="$(mktemp)"
trap 'rm -f "${MS_MANIFEST}"' EXIT

render_to_file() {
  local label="$1"; shift
  "${HELM_BIN}" template cuberouter "${CHART_DIR}" -n cuberouter "$@" > "${MS_MANIFEST}"
}

# A complete, valid mediaStudio configuration. --set-string keeps the
# credentials from being comma-split (a real S3 secret key is base64-ish and
# may contain commas); a values file works too.
MS_OK=(
  --set mediaStudio.enabled=true
  --set mediaStudio.s3.endpoint=https://objects.example.com
  --set mediaStudio.s3.bucket=cuberouter-media
  --set mediaStudio.s3.region=us-east-1
  --set-string mediaStudio.s3.access_key=AKIAEXAMPLEKEY
  --set-string mediaStudio.s3.secret_key=EXAMPLEsecretKEY
)

assert_no_media_studio() {
  local label="$1"; shift
  render_to_file "$label" "$@"
  if grep -q "MEDIA_STUDIO_S3_" "${MS_MANIFEST}"; then
    echo "   FAIL: $label must not render MEDIA_STUDIO_S3_*" >&2; exit 1
  fi
}

assert_media_studio_rejected() {
  local label="$1"; shift
  if render "$label" "${MS_OK[@]}" "$@" 2> /dev/null; then
    echo "   FAIL: expected render error for $label" >&2; exit 1
  fi
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

echo "== helm template: pgBouncer enabled, backups disabled =="
render pooler \
  --set postgresql.pgBouncer.enabled=true \
  --set postgresql.backups.enabled=false
echo "   ok"

echo "== helm template: deployMode=base (single replica everywhere) =="
render base --set deployMode=base
echo "   ok"

echo "== init containers: wait-for-postgres + wait-for-redis rendered by default =="
"${HELM_BIN}" template cuberouter "${CHART_DIR}" -n cuberouter \
  | grep -q "name: wait-for-postgres"
"${HELM_BIN}" template cuberouter "${CHART_DIR}" -n cuberouter \
  | grep -q "name: wait-for-redis"
echo "   ok"

echo "== init containers: absent when both checks disabled =="
if "${HELM_BIN}" template cuberouter "${CHART_DIR}" -n cuberouter \
  --set cubeRouter.waitForPostgres=false \
  --set cubeRouter.waitForRedis=false | grep -q "wait-for-"; then
  echo "   FAIL: no init containers should be rendered" >&2; exit 1
fi
echo "   ok"

echo "== negative: password with space must fail =="
if render neg-pw --set "secrets.POSTGRES_PASSWORD=bad password" 2> /dev/null; then
  echo "   FAIL: expected render error" >&2; exit 1
fi
echo "   ok (render correctly rejected)"

echo "== negative: password with URI special character must fail =="
if render neg-uripw --set "secrets.POSTGRES_PASSWORD=p@ss" 2> /dev/null; then
  echo "   FAIL: expected render error" >&2; exit 1
fi
echo "   ok (render correctly rejected)"

echo "== negative: invalid deployMode must fail =="
if render neg-mode --set "deployMode=bogus" 2> /dev/null; then
  echo "   FAIL: expected render error" >&2; exit 1
fi
echo "   ok (render correctly rejected)"

echo "== mediaStudio disabled by default: no MEDIA_STUDIO_S3_* is rendered =="
assert_no_media_studio default
assert_no_media_studio base --set deployMode=base
# The pre-existing-secret install is the one that would otherwise gain a
# secretKeyRef to a key its cluster secret does not carry (pods stuck in
# CreateContainerConfigError).
assert_no_media_studio existing-secret \
  --set secret.create=false --set secret.existingSecret=legacy-app-secret
echo "   ok"

echo "== mediaStudio enabled: plain settings in the ConfigMap, credentials in the Secret =="
render_to_file enabled "${MS_OK[@]}"
check_rendered() {
  grep -q "$1" "${MS_MANIFEST}" || {
    echo "   FAIL: $1 missing from the render" >&2; exit 1
  }
}
check_rendered 'MEDIA_STUDIO_S3_ENDPOINT: "https://objects.example.com"'
check_rendered 'MEDIA_STUDIO_S3_BUCKET: "cuberouter-media"'
check_rendered 'MEDIA_STUDIO_S3_REGION: "us-east-1"'
check_rendered 'MEDIA_STUDIO_S3_ACCESS_KEY: "AKIAEXAMPLEKEY"'
check_rendered 'MEDIA_STUDIO_S3_SECRET_KEY: "EXAMPLEsecretKEY"'
check_rendered 'key: MEDIA_STUDIO_S3_ACCESS_KEY'
check_rendered 'key: MEDIA_STUDIO_S3_SECRET_KEY'
echo "   ok"

echo "== credentials reach the Deployment only as a secretKeyRef =="
for literal in AKIAEXAMPLEKEY EXAMPLEsecretKEY; do
  count="$(grep -c "${literal}" "${MS_MANIFEST}" || true)"
  if [[ "${count}" != "1" ]]; then
    echo "   FAIL: ${literal} appears ${count} time(s), expected exactly 1 (the Secret only)" >&2
    exit 1
  fi
done
echo "   ok"

echo "== mediaStudio enabled against a pre-created secret (no credentials in values) =="
render enabled-existing-secret \
  --set mediaStudio.enabled=true \
  --set mediaStudio.s3.endpoint=https://objects.example.com \
  --set mediaStudio.s3.bucket=cuberouter-media \
  --set mediaStudio.s3.region=us-east-1 \
  --set secret.create=false \
  --set secret.existingSecret=legacy-app-secret
echo "   ok"

echo "== mediaStudio endpoint with a trailing slash is accepted =="
render enabled-trailing-slash \
  --set mediaStudio.enabled=true \
  --set mediaStudio.s3.endpoint=https://objects.example.com/ \
  --set mediaStudio.s3.bucket=cuberouter-media \
  --set mediaStudio.s3.region=us-east-1 \
  --set-string mediaStudio.s3.access_key=AKIAEXAMPLEKEY \
  --set-string mediaStudio.s3.secret_key=EXAMPLEsecretKEY
echo "   ok"

echo "== negative: mediaStudio enabled with nothing configured must fail =="
if render neg-ms-empty --set mediaStudio.enabled=true 2> /dev/null; then
  echo "   FAIL: expected render error" >&2; exit 1
fi
echo "   ok (render correctly rejected)"

echo "== negative: mediaStudio endpoint variants must fail =="
assert_media_studio_rejected neg-ms-path --set mediaStudio.s3.endpoint=https://objects.example.com/bucket
assert_media_studio_rejected neg-ms-query --set "mediaStudio.s3.endpoint=https://objects.example.com?x=1"
assert_media_studio_rejected neg-ms-userinfo --set "mediaStudio.s3.endpoint=https://user:pass@objects.example.com"
# The chart rejects plain HTTP even for loopback, which the Go validator would
# accept; an operator on localhost uses the env vars directly instead.
assert_media_studio_rejected neg-ms-http --set mediaStudio.s3.endpoint=http://127.0.0.1:9000
assert_media_studio_rejected neg-ms-noscheme --set mediaStudio.s3.endpoint=objects.example.com
echo "   ok (render correctly rejected)"

echo "== negative: invalid bucket and region must fail =="
assert_media_studio_rejected neg-ms-bucket --set mediaStudio.s3.bucket=Bad_Bucket
assert_media_studio_rejected neg-ms-bucket-ip --set mediaStudio.s3.bucket=10.0.0.1
assert_media_studio_rejected neg-ms-region --set "mediaStudio.s3.region=bad region"
echo "   ok (render correctly rejected)"

echo "== negative: missing credentials while the chart owns the secret must fail =="
if render neg-ms-creds \
  --set mediaStudio.enabled=true \
  --set mediaStudio.s3.endpoint=https://objects.example.com \
  --set mediaStudio.s3.bucket=cuberouter-media \
  --set mediaStudio.s3.region=us-east-1 2> /dev/null; then
  echo "   FAIL: expected render error" >&2; exit 1
fi
echo "   ok (render correctly rejected)"

echo "== negative: config.extra must not shadow the mediaStudio block =="
assert_media_studio_rejected neg-ms-extra \
  --set config.extra.MEDIA_STUDIO_S3_ENDPOINT=https://dupe.example.com
echo "   ok (render correctly rejected)"

echo "all checks passed"
