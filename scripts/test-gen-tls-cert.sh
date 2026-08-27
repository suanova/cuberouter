#!/usr/bin/env bash
# Smoke test for gen-tls-cert.sh: exercises all three issuance modes (A: CA
# cert+key, B: script-generated CA, C: CA cert only / self-signed server cert)
# and verifies the chain, SANs, validity window, key permissions, SAN
# requirement, and a real TLS handshake. Run from anywhere:
#   scripts/test-gen-tls-cert.sh
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
GEN="$SCRIPT_DIR/gen-tls-cert.sh"

if ! command -v openssl >/dev/null 2>&1; then
  echo "SKIP: openssl not found" >&2
  exit 0
fi

WORK="$(mktemp -d)"
trap 'rm -rf "$WORK"' EXIT

PASS=0
FAIL=0

check() { # check <desc> <cmd...>
  local desc="$1"
  shift
  if "$@" >/dev/null 2>&1; then
    PASS=$((PASS + 1))
    echo "PASS: $desc"
  else
    FAIL=$((FAIL + 1))
    echo "FAIL: $desc"
  fi
}

expect_fail() { # expect_fail <desc> <cmd...>
  local desc="$1"
  shift
  if "$@" >/dev/null 2>&1; then
    FAIL=$((FAIL + 1))
    echo "FAIL: $desc (expected failure, but succeeded)"
  else
    PASS=$((PASS + 1))
    echo "PASS: $desc"
  fi
}

file_mode() { # portable stat: GNU -c %a, BSD/macOS -f %Lp
  if stat -c %a "$1" >/dev/null 2>&1; then
    stat -c %a "$1"
  else
    stat -f %Lp "$1"
  fi
}

# --- Mode B: no CA; the script generates a root CA and signs with it ---
OUT_B="$WORK/mode-b"
"$GEN" --out "$OUT_B" --domains "gw.corp.local" --ips "10.0.0.5" --days 365
check "mode B: ca.crt produced" test -f "$OUT_B/ca.crt"
check "mode B: server.crt produced" test -f "$OUT_B/server.crt"
check "mode B: server.key produced" test -f "$OUT_B/server.key"
check "mode B: chain verifies against ca.crt" openssl verify -CAfile "$OUT_B/ca.crt" "$OUT_B/server.crt"
check "mode B: server.crt carries full chain (server + CA)" test "$(grep -c 'BEGIN CERTIFICATE' "$OUT_B/server.crt")" -ge 2
check "mode B: server.key mode is 600" test "$(file_mode "$OUT_B/server.key")" = "600"
check "mode B: ca.key mode is 600" test "$(file_mode "$OUT_B/ca.key")" = "600"
san=$(openssl x509 -in "$OUT_B/server.crt" -noout -ext subjectAltName 2>/dev/null || true)
check "mode B: server.crt has DNS SAN" test "${san#*DNS:gw.corp.local}" != "$san"
check "mode B: server.crt has IP SAN" test "${san#*IP Address:10.0.0.5}" != "$san"
eku=$(openssl x509 -in "$OUT_B/server.crt" -noout -ext extendedKeyUsage 2>/dev/null || true)
check "mode B: server.crt is serverAuth" test "${eku#*TLS Web Server Authentication}" != "$eku"

not_before=$("$(command -v openssl)" x509 -in "$OUT_B/server.crt" -noout -startdate | cut -d= -f2)
not_after=$("$(command -v openssl)" x509 -in "$OUT_B/server.crt" -noout -enddate | cut -d= -f2)
nb_ts=$(date -d "$not_before" +%s 2>/dev/null || date -j -f "%b %d %H:%M:%S %Y %Z" "$not_before" +%s 2>/dev/null || echo 0)
na_ts=$(date -d "$not_after" +%s 2>/dev/null || date -j -f "%b %d %H:%M:%S %Y %Z" "$not_after" +%s 2>/dev/null || echo 0)
today_ts=$(date +%s)
check "mode B: NotBefore is not in the future" test "$nb_ts" -le "$today_ts"
check "mode B: validity window is 365 days" test $(( (na_ts - nb_ts) / 86400 )) -eq 365

# --- Mode A: existing CA cert + key; no new CA is produced ---
OUT_A="$WORK/mode-a"
"$GEN" --out "$OUT_A" --ca-cert "$OUT_B/ca.crt" --ca-key "$OUT_B/ca.key" --domains "gw.corp.local" --ips "10.0.0.5"
check "mode A: no ca.crt produced" test ! -e "$OUT_A/ca.crt"
check "mode A: server.crt produced" test -f "$OUT_A/server.crt"
check "mode A: chain verifies against the provided CA" openssl verify -CAfile "$OUT_B/ca.crt" "$OUT_A/server.crt"
check "mode A: server.crt carries full chain (server + CA)" test "$(grep -c 'BEGIN CERTIFICATE' "$OUT_A/server.crt")" -ge 2

# --- Mode C: CA cert without private key -> self-signed server cert ---
OUT_C="$WORK/mode-c"
OUT_C_OUT=$("$GEN" --out "$OUT_C" --ca-cert "$OUT_B/ca.crt" --domains "gw.corp.local")
check "mode C: server.crt produced" test -f "$OUT_C/server.crt"
check "mode C: no ca.crt produced" test ! -e "$OUT_C/ca.crt"
check "mode C: server.crt is exactly one certificate" test "$(grep -c 'BEGIN CERTIFICATE' "$OUT_C/server.crt")" -eq 1
check "mode C: server.crt is self-signed (issuer == subject)" test \
  "$("$(command -v openssl)" x509 -in "$OUT_C/server.crt" -noout -issuer | cut -d= -f2-)" = \
  "$("$(command -v openssl)" x509 -in "$OUT_C/server.crt" -noout -subject | cut -d= -f2-)"
check "mode C: client trust points at the generated server.crt" test \
  "${OUT_C_OUT#*curl --cacert $OUT_C/server.crt}" != "$OUT_C_OUT"

# --- SAN is mandatory: no domains/IPs must fail, even with CA args ---
expect_fail "missing SAN is rejected" "$GEN" --out "$WORK/no-san" --ca-cert "$OUT_B/ca.crt" --ca-key "$OUT_B/ca.key"

# --- Supplied CA must be a real signing CA matching its key ---
expect_fail "leaf cert as --ca-cert is rejected" "$GEN" --out "$WORK/no-ca" \
  --ca-cert "$OUT_B/server.crt" --ca-key "$OUT_B/server.key" --domains "gw.corp.local"
expect_fail "mismatched CA cert/key is rejected" "$GEN" --out "$WORK/mismatch" \
  --ca-cert "$OUT_B/ca.crt" --ca-key "$OUT_B/server.key" --domains "gw.corp.local"
garbage_cert="$WORK/garbage.crt"
echo "not a certificate" > "$garbage_cert"
garbage_key="$WORK/garbage.key"
echo "not a key" > "$garbage_key"
expect_fail "garbage CA cert is rejected" "$GEN" --out "$WORK/garbage-cert" \
  --ca-cert "$garbage_cert" --ca-key "$OUT_B/ca.key" --domains "gw.corp.local"
expect_fail "garbage CA key is rejected" "$GEN" --out "$WORK/garbage-key" \
  --ca-cert "$OUT_B/ca.crt" --ca-key "$garbage_key" --domains "gw.corp.local"

# --- Real TLS handshake against a live server ---
PORT=""
for probe in $(seq 20000 20020); do
  if ! (exec 3<>"/dev/tcp/127.0.0.1/$probe") 2>/dev/null; then
    PORT=$probe
    break
  fi
  exec 3>&- 2>/dev/null || true
done
if [ -z "$PORT" ]; then
  check "TLS handshake verifies with ca.crt" false
else
  openssl s_server -accept "127.0.0.1:$PORT" -cert "$OUT_B/server.crt" -key "$OUT_B/server.key" -www -quiet >/dev/null 2>&1 &
  S_PID=$!
  trap 'kill "$S_PID" 2>/dev/null || true' EXIT
  # Wait until the server is actually listening before connecting; polling the
  # port is reliable where retrying s_client right away is not.
  ready=""
  for attempt in $(seq 1 50); do
    if (exec 3<>"/dev/tcp/127.0.0.1/$PORT") 2>/dev/null; then
      ready=yes
      break
    fi
    sleep 0.1
  done
  handshake=no
  if [ -n "$ready" ]; then
    for attempt in $(seq 1 3); do
      # grep -q, not -o: s_client prints "Verify return code" once at top level
      # and again inside SSL-Session details, so capturing -o output yields a
      # multi-line value that no single-string comparison can match.
      if echo | openssl s_client -connect "127.0.0.1:$PORT" -CAfile "$OUT_B/ca.crt" 2>/dev/null | grep -q "Verify return code: 0"; then
        handshake=yes
        break
      fi
      sleep 0.2
    done
  fi
  kill "$S_PID" 2>/dev/null || true
  wait "$S_PID" 2>/dev/null || true
  trap 'rm -rf "$WORK"' EXIT
  check "TLS handshake verifies with ca.crt" test "$handshake" = "yes"
fi

echo
echo "passed: $PASS, failed: $FAIL"
[ "$FAIL" -eq 0 ]
