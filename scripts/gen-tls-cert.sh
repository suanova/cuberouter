#!/usr/bin/env bash
# gen-tls-cert.sh - Generate a TLS server certificate for cuberouter in a
# private network, adapting to what the operator has:
#
#   A) a private CA cert + key      -> sign the server cert with it
#   B) nothing                      -> generate a root CA, then sign with it
#   C) a CA cert but no key         -> self-signed server cert (client trusts
#                                      the server cert directly)
#
# The certificate SANs (domains/IPs) are mandatory: without them every client
# fails hostname verification. Output is written to --out (default ./certs):
#   ca.crt     root CA (mode B only; distribute to clients)
#   server.crt server certificate; full chain (server + CA) in modes A/B
#   server.key private key, mode 0600, no passphrase (Go cannot load one)
#
# Re-run to renew: same command overwrites the previous output.
# Usage:
#   ./gen-tls-cert.sh [--domains d1,d2] [--ips 1.2.3.4,5.6.7.8]
#                     [--ca-cert PATH] [--ca-key PATH]
#                     [--out DIR] [--days N] [--help]
# With no --domains/--ips it walks through interactive prompts.
set -euo pipefail

usage() {
  sed -n '2,40p' "$0" | sed 's/^# \{0,1\}//'
}

DOMAINS=""
IPS=""
CA_CERT=""
CA_KEY=""
OUT_DIR="./certs"
DAYS=365

while [ $# -gt 0 ]; do
  case "$1" in
    --domains) DOMAINS="$2"; shift 2 ;;
    --ips) IPS="$2"; shift 2 ;;
    --ca-cert) CA_CERT="$2"; shift 2 ;;
    --ca-key) CA_KEY="$2"; shift 2 ;;
    --out) OUT_DIR="$2"; shift 2 ;;
    --days) DAYS="$2"; shift 2 ;;
    --help|-h) usage; exit 0 ;;
    *) echo "unknown argument: $1" >&2; usage; exit 1 ;;
  esac
done

if ! command -v openssl >/dev/null 2>&1; then
  echo "错误: 未找到 openssl,请先安装 (error: openssl not found)" >&2
  exit 1
fi
if ! command -v date >/dev/null 2>&1; then
  echo "错误: 未找到 date (error: date not found)" >&2
  exit 1
fi
case "$DAYS" in
  ''|*[!0-9]*) echo "错误: --days 必须是正整数 (error: --days must be a positive integer)" >&2; exit 1 ;;
esac

# Trim whitespace around every comma-separated item.
normalize_list() {
  echo "$1" | tr ',' '\n' | sed 's/^[[:space:]]*//; s/[[:space:]]*$//' | grep -v '^$' || true
}

domain_list=()
while IFS= read -r d; do domain_list+=("$d"); done < <(normalize_list "$DOMAINS")
ip_list=()
while IFS= read -r i; do ip_list+=("$i"); done < <(normalize_list "$IPS")

# Interactive fallback: collect SANs, then CA details.
if [ ${#domain_list[@]} -eq 0 ] && [ ${#ip_list[@]} -eq 0 ]; then
  echo "未提供域名/IP,进入交互模式 (no SANs given, entering interactive mode)"
  read -r -p "访问域名,逗号分隔,可含通配符,如 gw.corp.local,*.corp.local (domains): " DOMAINS || true
  read -r -p "访问 IP,逗号分隔,如 10.0.0.5,192.168.1.100 (IPs): " IPS || true
  while IFS= read -r d; do domain_list+=("$d"); done < <(normalize_list "$DOMAINS")
  while IFS= read -r i; do ip_list+=("$i"); done < <(normalize_list "$IPS")

  if [ -z "$CA_CERT" ]; then
    read -r -p "私有 CA 证书路径,没有则直接回车 (private CA cert path, Enter for none): " CA_CERT || true
    CA_CERT=$(echo "$CA_CERT" | xargs)
  fi
  if [ -n "$CA_CERT" ] && [ -z "$CA_KEY" ]; then
    read -r -p "CA 私钥路径,没有则回车(将自签服务端证书) (CA private key path, Enter for self-signed): " CA_KEY || true
    CA_KEY=$(echo "$CA_KEY" | xargs)
  fi
  if [ -z "$OUT_DIR" ] || [ "$OUT_DIR" = "./certs" ]; then
    read -r -p "输出目录,默认 ./certs (output dir): " OUT_DIR || true
    OUT_DIR=$(echo "$OUT_DIR" | xargs)
    [ -z "$OUT_DIR" ] && OUT_DIR="./certs"
  fi
fi

if [ ${#domain_list[@]} -eq 0 ] && [ ${#ip_list[@]} -eq 0 ]; then
  echo "错误: 必须提供至少一个域名或 IP,证书将无法通过客户端主机名校验 (error: at least one domain or IP is required for SAN)" >&2
  exit 1
fi
if [ -n "$CA_KEY" ] && [ -z "$CA_CERT" ]; then
  echo "错误: 提供 CA 私钥时必须同时提供 CA 证书 (error: CA key requires CA cert)" >&2
  exit 1
fi
if [ -n "$CA_CERT" ] && [ ! -f "$CA_CERT" ]; then
  echo "错误: CA 证书文件不存在 (error: CA cert not found): $CA_CERT" >&2
  exit 1
fi
if [ -n "$CA_KEY" ] && [ ! -f "$CA_KEY" ]; then
  echo "错误: CA 私钥文件不存在 (error: CA key not found): $CA_KEY" >&2
  exit 1
fi
if [ -n "$CA_CERT" ] && [ -n "$CA_KEY" ]; then
  # A leaf certificate signs happily but the chain is rejected by clients;
  # require a real signing CA before issuing anything.
  bc=$(openssl x509 -in "$CA_CERT" -noout -ext basicConstraints 2>/dev/null || true)
  if ! printf '%s' "$bc" | grep -q 'CA:TRUE'; then
    echo "错误: 提供的 CA 证书不是签发 CA (error: CA cert is not a signing CA, need basicConstraints CA:TRUE): $CA_CERT" >&2
    exit 1
  fi
  ku=$(openssl x509 -in "$CA_CERT" -noout -ext keyUsage 2>/dev/null || true)
  if ! printf '%s' "$ku" | grep -q 'Certificate Sign'; then
    echo "错误: 提供的 CA 证书缺少 keyCertSign (error: CA cert lacks keyCertSign in keyUsage): $CA_CERT" >&2
    exit 1
  fi
  cert_pub=$(openssl x509 -in "$CA_CERT" -noout -pubkey 2>/dev/null || true)
  key_pub=$(openssl pkey -in "$CA_KEY" -pubout 2>/dev/null || true)
  if [ -z "$cert_pub" ] || [ -z "$key_pub" ] || [ "$cert_pub" != "$key_pub" ]; then
    echo "错误: CA 证书与私钥不匹配或无法读取 (error: CA cert and key do not match or cannot be read): $CA_CERT / $CA_KEY" >&2
    exit 1
  fi
fi

CN="${domain_list[0]:-${ip_list[0]}}"
echo "服务端证书 CN/SAN: CN=$CN domains=${domain_list[*]} ips=${ip_list[*]}"
echo "有效期 (validity): $DAYS 天 (days)"

mkdir -p "$OUT_DIR"
WORK="$(mktemp -d)"
trap 'rm -rf "$WORK"' EXIT

# OpenSSL config template with SANs; a config file keeps compatibility with
# openssl < 1.1.1 which lacks the -addext flag.
CNF="$WORK/openssl.cnf"
{
  echo "[req]"
  echo "distinguished_name = req_distinguished_name"
  echo "req_extensions = v3_req"
  echo "prompt = no"
  echo ""
  echo "[req_distinguished_name]"
  echo "CN = $CN"
  echo ""
  echo "[v3_req]"
  echo "keyUsage = keyEncipherment, digitalSignature"
  echo "extendedKeyUsage = serverAuth"
  echo "subjectAltName = @alt_names"
  echo ""
  echo "[alt_names]"
  idx=1
  for d in "${domain_list[@]}"; do
    echo "DNS.$idx = $d"
    idx=$((idx + 1))
  done
  idx=1
  for i in "${ip_list[@]}"; do
    echo "IP.$idx = $i"
    idx=$((idx + 1))
  done
} > "$CNF"

# Mode B: no CA at all -> generate a root CA (10 years) for signing and for
# clients to trust. After this the issuance is identical to mode A: CA_CERT
# and CA_KEY both point at the generated files.
GENERATED_CA=0
if [ -z "$CA_CERT" ]; then
  echo "模式: 生成新根 CA 并签发 (mode B: generating a new root CA and signing)"
  # Config file (not -addext) keeps compatibility with openssl < 1.1.1 and
  # stamps the CA with the extensions the CA validation above requires.
  CA_CNF="$WORK/ca.cnf"
  {
    echo "[req]"
    echo "distinguished_name = dn"
    echo "x509_extensions = v3_ca"
    echo "prompt = no"
    echo "[dn]"
    echo "CN = Cuberouter Local CA"
    echo "[v3_ca]"
    echo "basicConstraints = critical,CA:TRUE"
    echo "keyUsage = critical,keyCertSign,cRLSign"
    echo "subjectKeyIdentifier = hash"
  } > "$CA_CNF"
  openssl req -x509 -newkey rsa:2048 -nodes \
    -keyout "$OUT_DIR/ca.key" -out "$OUT_DIR/ca.crt" \
    -days 3650 -config "$CA_CNF"
  chmod 600 "$OUT_DIR/ca.key"
  CA_CERT="$OUT_DIR/ca.crt"
  CA_KEY="$OUT_DIR/ca.key"
  GENERATED_CA=1
fi

# Server key + CSR; -nodes means no passphrase (Go's LoadX509KeyPair cannot
# load an encrypted key).
echo "生成服务端密钥和签名请求 (generating server key and CSR)"
openssl req -newkey rsa:2048 -nodes \
  -keyout "$OUT_DIR/server.key" -out "$WORK/server.csr" -subj "/CN=$CN"
chmod 600 "$OUT_DIR/server.key"

# Note: NotBefore is the signing time; the openssl CLI cannot backdate it
# (neither x509 -req nor req -x509 supports -startdate), so a clock up to
# hours slow sees a "not yet valid" error only briefly. The 365-day default
# window absorbs the rest of the skew.

# Random serial in a temp file so an existing CA directory never needs to be
# writable and serials never repeat.
openssl rand -hex 8 > "$WORK/ca.srl"

if [ -n "$CA_KEY" ]; then
  if [ "$GENERATED_CA" = "1" ]; then
    echo "用生成的根 CA 签发 (signing with the generated root CA)"
  else
    echo "模式: 用私有 CA 签发 (mode A: signing with the provided CA)"
  fi
  openssl x509 -req -in "$WORK/server.csr" \
    -CA "$CA_CERT" -CAkey "$CA_KEY" -CAserial "$WORK/ca.srl" \
    -out "$WORK/server.crt" -days "$DAYS" \
    -extfile "$CNF" -extensions v3_req
  # Full chain: server cert + CA, so Go's LoadX509KeyPair presents it whole
  # and clients only need to trust the root CA.
  cat "$WORK/server.crt" "$CA_CERT" > "$OUT_DIR/server.crt"
else
  echo "模式: 自签服务端证书 (mode C: self-signed server cert)"
  openssl x509 -req -in "$WORK/server.csr" \
    -signkey "$OUT_DIR/server.key" \
    -out "$OUT_DIR/server.crt" -days "$DAYS" \
    -extfile "$CNF" -extensions v3_req
fi

echo ""
echo "======================================================"
echo "生成完成 (Certificates generated)"
echo "======================================================"
echo "服务端配置 (Server configuration):"
echo "  TLS_CERT_FILE=$OUT_DIR/server.crt"
echo "  TLS_KEY_FILE=$OUT_DIR/server.key"
echo "  TLS_PORT=443   # 可用 TLS_PORT 环境变量覆盖 (override with TLS_PORT)"
echo ""
TRUST_FILE=""
if [ "$GENERATED_CA" = "1" ]; then
  TRUST_FILE="$OUT_DIR/ca.crt"
elif [ -n "$CA_CERT" ] && [ -n "$CA_KEY" ]; then
  # Mode A: the server cert was signed by this CA, so clients trust it.
  TRUST_FILE="$CA_CERT"
fi
# Mode C (CA cert without key) falls through: the server cert is self-signed
# and the provided CA never signed it, so clients must trust server.crt.
if [ -n "$TRUST_FILE" ]; then
  echo "客户端信任 (Client trust):"
  echo "  将 $TRUST_FILE 导入客户端信任库(浏览器/系统/curl --cacert),"
  echo "  之后访问 https 将不再告警。"
  echo "  (Import the CA certificate into client trust stores so HTTPS works without warnings.)"
  echo "  验证 (verify): curl --cacert $TRUST_FILE https://${CN}/api/status"
else
  echo "客户端信任 (Client trust):"
  echo "  无 CA 私钥,已生成自签服务端证书;客户端直接信任 server.crt 或使用 --cacert。"
  echo "  (No CA key available; clients must trust server.crt directly or use --cacert.)"
  echo "  验证 (verify): curl --cacert $OUT_DIR/server.crt https://${CN}/api/status"
fi
