<div align="center">

![CubeRouter](/web/public/logo.png)

# CubeRouter

🍥 **Next-Generation LLM Gateway and AI Asset Management System**

<p align="center">
  <a href="./README.zh_CN.md">简体中文</a> |
  <a href="./README.zh_TW.md">繁體中文</a> |
  <strong>English</strong> |
</p>

<p align="center">
  <a href="https://raw.githubusercontent.com/suanova/cuberouter/main/LICENSE">
    <img src="https://img.shields.io/github/license/suanova/cuberouter?color=brightgreen" alt="license">
  </a><!--
  --><a href="https://github.com/suanova/cuberouter/releases/latest">
    <img src="https://img.shields.io/github/v/release/suanova/cuberouter?color=brightgreen&include_prereleases" alt="release">
  </a>
  <a href="https://atomgit.com/QuantumNous/new-api" target="_blank">
    <img alt="AtomGit G-Star" src="https://atomgit.com/QuantumNous/new-api/star/badge.svg"/>
  </a>
</p>

<p align="center">
  <a href="#-quick-start">Quick Start</a> •
  <a href="#-key-features">Key Features</a> •
  <a href="#-deployment">Deployment</a> •
  <a href="#-documentation">Documentation</a>
</p>

</div>

## 📝 Project Description

> [!IMPORTANT]
> - This project is intended solely for lawful and authorized AI API gateway, organization-level authentication, multi-model management, usage analytics, cost accounting, and private deployment scenarios.
> - Users must lawfully obtain upstream API keys, accounts, model services, and interface permissions, and must comply with upstream terms of service and applicable laws and regulations.
> - Users should ensure their use complies with upstream terms of service and applicable laws and regulations.
> - When providing generative AI services to the public, users should comply with applicable regulatory requirements and fulfill all filing, licensing, content safety, real-name verification, log retention, tax, and upstream authorization obligations required by their jurisdiction.

---

## 🚀 Quick Start

### Using Docker Compose (Recommended)

```bash
# Clone the project
git clone https://github.com/suanova/cuberouter.git
cd cuberouter

# Edit docker-compose.yml configuration
nano docker-compose.yml

# Start the service
docker-compose up -d
```

<details>
<summary><strong>Using Docker Commands</strong></summary>

```bash
# Pull the latest image
docker pull harbor.isuanova.com/suanova/cuberouter:latest

# Using SQLite (default)
docker run --name cuberouter -d --restart always \
  -p 3000:3000 \
  -e TZ=Asia/Shanghai \
  -v ./data:/data \
  harbor.isuanova.com/suanova/cuberouter:latest

# Using MySQL
docker run --name cuberouter -d --restart always \
  -p 3000:3000 \
  -e SQL_DSN="root:123456@tcp(localhost:3306)/oneapi" \
  -e TZ=Asia/Shanghai \
  -v ./data:/data \
  harbor.isuanova.com/suanova/cuberouter:latest
```

> **💡 Tip:** `-v ./data:/data` will save data in the `data` folder of the current directory, you can also change it to an absolute path like `-v /your/custom/path:/data`

</details>

---

🎉 After deployment is complete, visit `http://localhost:3000` to start using!

> [!WARNING]
> When operating this project as a public generative AI service or API resale service, users should first complete all required filing, licensing, content safety, real-name verification, log retention, tax, payment, and upstream authorization obligations.

📖 For more deployment methods, please refer to [Deployment Guide](https://docs.newapi.pro/en/docs/installation)

---

## 📚 Documentation

<div align="center">

### 📖 [Official Documentation](https://docs.newapi.pro/en/docs) | [![Ask DeepWiki](https://deepwiki.com/badge.svg)](https://deepwiki.com/suanova/cuberouter)

</div>

**Quick Navigation:**

| Category | Link |
|------|------|
| 🚀 Deployment Guide | [Installation Documentation](https://docs.newapi.pro/en/docs/installation) |
| ⚙️ Environment Configuration | [Environment Variables](https://docs.newapi.pro/en/docs/installation/config-maintenance/environment-variables) |
| 📡 API Documentation | [API Documentation](https://docs.newapi.pro/en/docs/api) |
| ❓ FAQ | [FAQ](https://docs.newapi.pro/en/docs/support/faq) |
| 💬 Community Interaction | [Communication Channels](https://docs.newapi.pro/en/docs/support/community-interaction) |

---

## ✨ Key Features

> For detailed features, please refer to [Features Introduction](https://docs.newapi.pro/en/docs/guide/wiki/basic-concepts/features-introduction)

### 🎨 Core Functions

| Feature | Description |
|------|------|
| 🎨 New UI | Modern user interface design |
| 🌍 Multi-language | Supports Simplified Chinese, Traditional Chinese, English, French, Japanese |
| 🔄 Data Compatibility | Fully compatible with the original One API database |
| 📈 Data Dashboard | Visual console and statistical analysis |
| 🔒 Permission Management | Token grouping, model restrictions, user management |

### 💰 Authorized Usage Accounting and Billing

- ✅ Internal top-up and quota allocation for lawful authorized scenarios (EPay, Stripe)
- ✅ Organization-level per-request, usage-based, and cache-hit cost accounting
- ✅ Cache billing statistics for OpenAI, Azure, DeepSeek, Claude, Qwen, and supported models
- ✅ Flexible billing policies for internal management or authorized enterprise customers

### 🔐 Authorization and Security

- 😈 Discord authorization login
- 🤖 LinuxDO authorization login
- 📱 Telegram authorization login
- 🔑 OIDC unified authentication
- 🔍 Key quota query usage (with [new-api-key-tool](https://github.com/Calcium-Ion/new-api-key-tool))

### 🚀 Advanced Features

**API Format Support:**
- ⚡ [OpenAI Responses](https://docs.newapi.pro/en/docs/api/ai-model/chat/openai/create-response)
- ⚡ [OpenAI Realtime API](https://docs.newapi.pro/en/docs/api/ai-model/realtime/create-realtime-session) (including Azure)
- ⚡ [Claude Messages](https://docs.newapi.pro/en/docs/api/ai-model/chat/create-message)
- ⚡ [Google Gemini](https://doc.newapi.pro/en/api/google-gemini-chat)
- 🔄 [Rerank Models](https://docs.newapi.pro/en/docs/api/ai-model/rerank/create-rerank) (Cohere, Jina)

**Intelligent Routing:**
- ⚖️ Channel weighted random
- 🔄 Automatic retry on failure
- 🚦 User-level model rate limiting

**Format Conversion:**
- 🔄 **OpenAI Compatible ⇄ Claude Messages**
- 🔄 **OpenAI Compatible → Google Gemini**
- 🔄 **Google Gemini → OpenAI Compatible** - Text only, function calling not supported yet
- 🚧 **OpenAI Compatible ⇄ OpenAI Responses** - In development
- 🔄 **Thinking-to-content functionality**

**Reasoning Effort Support:**

<details>
<summary>View detailed configuration</summary>

**OpenAI series models:**
- `o3-mini-high` - High reasoning effort
- `o3-mini-medium` - Medium reasoning effort
- `o3-mini-low` - Low reasoning effort
- `gpt-5-high` - High reasoning effort
- `gpt-5-medium` - Medium reasoning effort
- `gpt-5-low` - Low reasoning effort

**Claude thinking models:**
- `claude-3-7-sonnet-20250219-thinking` - Enable thinking mode

**Google Gemini series models:**
- `gemini-2.5-flash-thinking` - Enable thinking mode
- `gemini-2.5-flash-nothinking` - Disable thinking mode
- `gemini-2.5-pro-thinking` - Enable thinking mode
- `gemini-2.5-pro-thinking-128` - Enable thinking mode with thinking budget of 128 tokens
- You can also append `-low`, `-medium`, or `-high` to any Gemini model name to request the corresponding reasoning effort (no extra thinking-budget suffix needed).

</details>

---

## 🤖 Model Support

> For details, please refer to [API Documentation - Gateway Interface](https://docs.newapi.pro/en/docs/api)

| Model Type | Description | Documentation |
|---------|------|------|
| 🤖 OpenAI-Compatible | OpenAI compatible models | [Documentation](https://docs.newapi.pro/en/docs/api/ai-model/chat/openai/createchatcompletion) |
| 🤖 OpenAI Responses | OpenAI Responses format | [Documentation](https://docs.newapi.pro/en/docs/api/ai-model/chat/openai/createresponse) |
| 🎨 Midjourney-Proxy | [Midjourney-Proxy(Plus)](https://github.com/novicezk/midjourney-proxy) | [Documentation](https://doc.newapi.pro/api/midjourney-proxy-image) |
| 🎵 Suno-API | [Suno API](https://github.com/Suno-API/Suno-API) | [Documentation](https://doc.newapi.pro/api/suno-music) |
| 🔄 Rerank | Cohere, Jina | [Documentation](https://docs.newapi.pro/en/docs/api/ai-model/rerank/creatererank) |
| 💬 Claude | Messages format | [Documentation](https://docs.newapi.pro/en/docs/api/ai-model/chat/createmessage) |
| 🌐 Gemini | Google Gemini format | [Documentation](https://docs.newapi.pro/en/docs/api/ai-model/chat/gemini/geminirelayv1beta) |
| 🔧 Dify | ChatFlow mode | - |
| 🎯 Custom upstream | Supports configuring legally authorized upstream endpoints | - |

### 📡 Supported Interfaces

<details>
<summary>View complete interface list</summary>

- [Chat Interface (Chat Completions)](https://docs.newapi.pro/en/docs/api/ai-model/chat/openai/createchatcompletion)
- [Response Interface (Responses)](https://docs.newapi.pro/en/docs/api/ai-model/chat/openai/createresponse)
- [Image Interface (Image)](https://docs.newapi.pro/en/docs/api/ai-model/images/openai/post-v1-images-generations)
- [Audio Interface (Audio)](https://docs.newapi.pro/en/docs/api/ai-model/audio/openai/create-transcription)
- [Video Interface (Video)](https://docs.newapi.pro/en/docs/api/ai-model/audio/openai/createspeech)
- [Embedding Interface (Embeddings)](https://docs.newapi.pro/en/docs/api/ai-model/embeddings/createembedding)
- [Rerank Interface (Rerank)](https://docs.newapi.pro/en/docs/api/ai-model/rerank/creatererank)
- [Realtime Conversation (Realtime)](https://docs.newapi.pro/en/docs/api/ai-model/realtime/createrealtimesession)
- [Claude Chat](https://docs.newapi.pro/en/docs/api/ai-model/chat/createmessage)
- [Google Gemini Chat](https://docs.newapi.pro/en/docs/api/ai-model/chat/gemini/geminirelayv1beta)

</details>

---

## 🚢 Deployment

> [!TIP]
> **Latest Docker image:** `harbor.isuanova.com/suanova/cuberouter:latest`

### 📋 Deployment Requirements

| Component | Requirement |
|------|------|
| **Local database** | SQLite (Docker must mount `/data` directory)|
| **Remote database** | MySQL ≥ 5.7.8 or PostgreSQL ≥ 9.6 |
| **Container engine** | Docker / Docker Compose |
| **System architecture** | 64-bit only (amd64 / arm64); 32-bit systems are not supported |

### ⚙️ Environment Variable Configuration

<details>
<summary>Common environment variable configuration</summary>

| Variable Name | Description | Default Value |
|--------|------|--------|
| `SESSION_SECRET` | Authentication signing secret; must be identical on every node | - |
| `SESSION_COOKIE_SECURE` | `false`/unset disables the refresh/logout OriginGuard for local HTTP dev proxies; `true` enables the Secure cookie and strict Origin checks | `false` |
| `SESSION_COOKIE_TRUSTED_URL` | Required with Secure mode: comma-separated exact HTTPS Origins allowed to call refresh/logout; not a relay CORS allowlist | - |
| `TLS_CERT_FILE` | HTTPS certificate file (PEM); the HTTPS listener is enabled only when set together with `TLS_KEY_FILE` | - |
| `TLS_KEY_FILE` | HTTPS private key file (PEM); must be set together with `TLS_CERT_FILE` | - |
| `TLS_PORT` | HTTPS listening port | `443` |
| `TRUSTED_PROXIES` | Unset/blank trusts loopback, RFC 1918 and IPv6 ULA with a startup warning; `none` trusts no proxies; an explicit proxy IP/CIDR list replaces the defaults | `127.0.0.0/8, ::1, 10.0.0.0/8, 172.16.0.0/12, 192.168.0.0/16, fc00::/7` |
| `USER_SESSION_ACTIVE_LIMIT` | Maximum active login Sessions per user | `50` |
| `USER_SESSION_ISSUANCE_LIMIT` | Maximum Sessions created per user within the issuance window, including revoked Sessions | `100` |
| `USER_SESSION_ISSUANCE_WINDOW_SECONDS` | Per-user Session issuance window; clamped to the revoked retention period when configured higher | `86400` |
| `USER_SESSION_REVOKED_RETENTION_DAYS` | Days to retain revoked Session rows for audit and issuance accounting | `7` |
| `USER_SESSION_HOURLY_ALERT_THRESHOLD` | Global Sessions created per hour that triggers an alert only; it never blocks login | `5000` |
| `CRYPTO_SECRET` | HMAC secret for cache keys; nodes sharing Redis must use the same effective value | Defaults to `SESSION_SECRET` |
| `SQL_DSN` | Database connection string | - |
| `REDIS_CONN_STRING` | Redis connection string | - |
| `RELAY_IDLE_CONN_TIMEOUT` | Idle keep-alive timeout for relay HTTP clients, seconds. Defaults to Go standard library behavior; set `0` to disable | `90` |
| `STREAMING_TIMEOUT` | Streaming timeout (seconds) | `300` |
| `STREAM_SCANNER_MAX_BUFFER_MB` | Max per-line buffer (MB) for the stream scanner; increase when upstream sends huge image/base64 payloads | `64` |
| `MAX_REQUEST_BODY_MB` | Max request body size (MB, counted **after decompression**; prevents huge requests/zip bombs from exhausting memory). Exceeding it returns `413` | `32` |
| `AZURE_DEFAULT_API_VERSION` | Azure API version | `2025-04-01-preview` |
| `ERROR_LOG_ENABLED` | Error log switch | `false` |
| `PYROSCOPE_URL` | Pyroscope server address | - |
| `PYROSCOPE_APP_NAME` | Pyroscope application name | `new-api` |
| `PYROSCOPE_BASIC_AUTH_USER` | Pyroscope basic auth user | - |
| `PYROSCOPE_BASIC_AUTH_PASSWORD` | Pyroscope basic auth password | - |
| `PYROSCOPE_MUTEX_RATE` | Pyroscope mutex sampling rate | `5` |
| `PYROSCOPE_BLOCK_RATE` | Pyroscope block sampling rate | `5` |
| `HOSTNAME` | Hostname tag for Pyroscope | `new-api` |

📖 **Complete configuration:** [Environment Variables Documentation](https://docs.newapi.pro/en/docs/installation/config-maintenance/environment-variables)

</details>

### 🔒 HTTPS (TLS)

HTTPS is opt-in: set `TLS_CERT_FILE` + `TLS_KEY_FILE` (PEM, must be paired) and the gateway serves HTTPS on `TLS_PORT` (default `443`) **in addition to** the plain HTTP port; unset, it behaves exactly as before.

For private networks, `scripts/gen-tls-cert.sh` generates the certificate for your situation:

| Situation | Command |
|------|------|
| **Private CA (cert + key available)** | `scripts/gen-tls-cert.sh --ca-cert ca.pem --ca-key ca.key --domains "gw.corp.local" --ips "10.0.0.5"` |
| **No CA at all** | `scripts/gen-tls-cert.sh --domains "gw.corp.local" --ips "10.0.0.5"` (generates a root CA for client distribution) |
| **CA cert only, no key** | `scripts/gen-tls-cert.sh --ca-cert ca.pem --domains "gw.corp.local"` (self-signed server cert) |

- SANs (domains/IPs) are mandatory; run without arguments for interactive prompts
- Output goes to `./certs/` (`--out` to change): `server.crt` (full chain), `server.key` (mode `0600`), and `ca.crt` when a CA is generated — distribute `ca.crt` to clients so HTTPS works without warnings
- Re-run the same command to renew (e.g. once a year)

**Client trust (choose one):**

| Option | How | Best for |
|------|------|------|
| Install into the system trust store | Import `ca.crt` (the root CA, not `server.crt`) into each machine's system/browser trust store | One-time setup; every client trusts automatically but requires distributing the CA to users |
| Configure per client | `curl --cacert ca.crt`, `SSL_CERT_FILE=ca.crt` (Go/curl), `NODE_EXTRA_CA_CERTS` (Node), `REQUESTS_CA_BUNDLE` (Python) | each client tool needs its own config |

LLM client tools (Claude Code, opencode, etc.) usually read environment variables — set `SSL_CERT_FILE` or `NODE_EXTRA_CA_CERTS` and they are covered.

**Local trial with Docker:**

```bash
docker build -t cuberouter:local .   # build from the local tree (includes the latest changes)
bash scripts/gen-tls-cert.sh --domains "localhost" --ips "127.0.0.1"   # mode B: also produces ca.crt
docker run -d --name cuberouter-https -p 3000:3000 -p 443:443 \
  -e TLS_CERT_FILE=/certs/server.crt -e TLS_KEY_FILE=/certs/server.key \
  -v "$PWD/certs:/certs:ro" -v cuberouter-https-data:/data \
  cuberouter:local
curl -s https://localhost/api/status --cacert ./certs/ca.crt   # verified against the generated root CA, no -k
```

### 🔧 Deployment Methods

<details>
<summary><strong>Method 1: Docker Compose (Recommended)</strong></summary>

```bash
# Clone the project
git clone https://github.com/suanova/cuberouter.git
cd cuberouter

# Edit configuration
nano docker-compose.yml

# Start service
docker-compose up -d
```

</details>

<details>
<summary><strong>Method 2: Docker Commands</strong></summary>

**Using SQLite:**
```bash
docker run --name cuberouter -d --restart always \
  -p 3000:3000 \
  -e TZ=Asia/Shanghai \
  -v ./data:/data \
  harbor.isuanova.com/suanova/cuberouter:latest
```

**Using MySQL:**
```bash
docker run --name cuberouter -d --restart always \
  -p 3000:3000 \
  -e SQL_DSN="root:123456@tcp(localhost:3306)/oneapi" \
  -e TZ=Asia/Shanghai \
  -v ./data:/data \
  harbor.isuanova.com/suanova/cuberouter:latest
```

> **💡 Path explanation:**
> - `./data:/data` - Relative path, data saved in the data folder of the current directory
> - You can also use absolute path, e.g.: `/your/custom/path:/data`

</details>

<details>
<summary><strong>Method 3: BaoTa Panel</strong></summary>

1. Install BaoTa Panel (≥ 9.2.0 version)
2. Search for **New-API** in the application store
3. One-click installation

📖 [Tutorial with images](./docs/BT.md)

</details>

### ⚠️ Multi-machine Deployment Considerations

> [!WARNING]
> - All nodes must use the same primary database and the same `SESSION_SECRET`; otherwise Access Tokens, refresh sessions, and temporary authentication flows cannot be verified consistently.
> - Nodes connected to the same Redis must also use the same `CRYPTO_SECRET`, or their cache-key digests will differ and shared entries cannot be reused consistently.

The database is authoritative for login Sessions and for the per-user active/issuance limits. Redis Session entries are short-lived caches whose TTL follows `SYNC_FREQUENCY` (60 seconds by default) and never exceeds the Session's remaining lifetime.

| Redis topology | Session propagation | Rate limiting |
| --- | --- | --- |
| Shared Redis | Revocations and version publications normally propagate immediately | Redis limits are shared across nodes |
| Independent Redis per node | Nodes converge from the database within the effective `SYNC_FREQUENCY`; a newly rotated token may receive a temporary 401 on a node with stale cache | Each node has its own allowance, so aggregate capacity can reach roughly the configured limit multiplied by the node count |
| No Redis | Every Session validation reads the database | In-memory limits are independent per node |

A shorter `SYNC_FREQUENCY` reduces the independent-Redis staleness window but causes one additional primary-key Session lookup per active SID, per node, per TTL. These guarantees make Session authentication bounded-stale across the supported topologies; rate limits and other Redis-backed control-plane caches remain topology-dependent.

See [User authentication and login sessions](./docs/authentication.md) for the token, Origin-check and PAT contracts.

### 🔄 Channel Retry and Cache

**Retry configuration:** `Settings → Operation Settings → General Settings → Failure Retry Count`

**Cache configuration:**
- `REDIS_CONN_STRING`: Redis cache (recommended)
- `MEMORY_CACHE_ENABLED`: Memory cache

---

## 📊 Performance Metrics and Prometheus Export (性能指标与导出)

The gateway samples every relayed request into in-memory buckets (模型/分组/渠道 × 时间桶), flushes completed buckets to the database, and keeps a process-local registry for Prometheus scraping. Two data domains:

| Domain | Table | Content | Frontend |
|------|------|------|------|
| 性能指标 (perf metrics) | `perf_metrics` | Per model × group × channel × bucket: request/success counts, latency & TTFT latency histograms, tokens | Average TTFT with P95/P99 latency trend lines on the model-square model detail page; per-channel breakdown on the admin dashboard |
| 容量指标 (capacity) | `capacity_metrics` | Per bucket: relay `attempts`, `rejected_503`, in-flight peak | Admin dashboard Capacity Overview card (容量总览) |

Channel identity is internal information: the `channels` detail of `GET /api/perf-metrics` is returned only to admins (role ≥ 10) and is stripped server-side for anonymous and regular users.

### Configuration (`perf_metrics_setting`, 设置 → 运营设置 → Monitoring & Alerts)

| Key | Default | Meaning |
|------|------|------|
| `enabled` | `true` | Master switch of perf sampling and DB flush |
| `flush_interval` | `5` | Flush cadence (minutes): completed buckets are written to DB and retention cleanup runs on this interval |
| `bucket_time` | `hour` | Bucket granularity: `hour` / `5min` / `minute`. `hour` is the recommended production granularity (smoother series, less rows); shorter buckets give finer-grained trend lines at more storage cost |
| `retention_days` | `0` | Days to keep rows before cleanup. `0` = **keep forever** (default). Production is recommended to set `≥ 7`. Cleanup deletes from `perf_metrics` and `capacity_metrics` together |
| `export_enabled` | `false` | Expose the Prometheus endpoint `GET /api/metrics` |
| `export_token` | empty | Static Bearer token for the export endpoint. Leave empty = **no authentication** — when empty, restrict access at the network layer (firewall / reverse-proxy allow-list to the monitoring subnet only) |

### Capacity API (`/api/capacity-metrics`, AdminAuth)

`GET /api/capacity-metrics?hours=N` — gateway capacity trend over the last `N` hours (default `24`, capped at `720` = 30 days). Returns `data.series` of:

| Field | Meaning |
|------|------|
| `ts` | Bucket start, Unix seconds |
| `attempts` | Relay requests, each attributed to the bucket in which it **completes** (end-time attribution — a stream crossing a bucket boundary lands in the later bucket). **Includes requests later rejected by auth, rate limiting, or overload 503** (the counting middleware runs first in every relay chain, before auth and before the overload check) |
| `rejected_503` | Subset of `attempts`: requests rejected with HTTP 503 by the overload protection (`SystemPerformanceCheck`, CPU/memory/disk thresholds). `rejected_503 ⊆ attempts` |
| `inflight_peak` | Peak in-flight concurrency in the bucket — a **2-second sampling approximation**, not an exact maximum |

Two approximations to expect: rows are written only after a bucket completes (flush loop), so with the default hourly buckets the newest visible point is typically the previous completed bucket — roughly one bucket (~1 h) behind the current wall-clock bucket, plus up to one flush period of write lag after the bucket closes; and RPS is derived client-side from `attempts` over the bucket width, where the width is inferred from neighboring point timestamps (the latest point reuses the previous bucket interval).

### Prometheus export (`/api/metrics`)

Enable via `perf_metrics_setting.export_enabled` + `export_token` (配置见上). Behavior:

- **Reachable on all three API prefixes:** `/api/metrics`, `/api/v1/metrics`, `/api/v2/metrics`.
- Disabled → `404` (does not reveal existence). With a token set, `Authorization: Bearer <token>` is required (constant-time comparison); otherwise `401`.
- **Process-level export:** each instance's registry accumulates since process start and resets on restart (standard counter semantics). For multi-instance deployments, scrape **every pod/instance** as its own target — do not aggregate instances into one target.
- **Freezing, not clearing:** when `enabled = false` while `export_enabled = true`, sampling stops and the model/group-dimensioned families stay frozen at their last values; the same switch also pauses the `capacity_metrics` DB flush and retention cleanup (both tables). Process-level counters (`cuberouter_relay_attempts_total`, `cuberouter_overload_rejects_total`) and the in-flight gauge keep updating — the in-process capacity path does not depend on the sampling switch, only its DB persistence and cleanup do.
- Series carry `model` and `group` labels only — the channel dimension is deliberately **not** exported (high cardinality).

| Metric | Type | Meaning |
|------|------|------|
| `cuberouter_relay_requests_total{model,group}` | counter | Total relay requests (incl. failures) since process start |
| `cuberouter_relay_latency_seconds{model,group}` | histogram | Relay latency, seconds |
| `cuberouter_relay_ttft_seconds{model,group}` | histogram | Time-to-first-token (streaming requests only), seconds |
| `cuberouter_inflight_requests` | gauge | Requests currently in flight (process-local) |
| `cuberouter_overload_rejects_total` | counter | Requests rejected with HTTP 503 by overload protection since process start |
| `cuberouter_relay_attempts_total` | counter | Relay attempts (incl. auth/rate-limit/overload-rejected) since process start |
| `go_goroutines`, `go_memstats_alloc_bytes`, `go_memstats_heap_objects` | gauge | Go runtime basics |

**Histogram bucket boundaries** (identical for latency and TTFT; samples above 240 s fall into the tail cell and only appear in `le="+Inf"` / `_count`):

| `le` (seconds) | 0.1 | 0.25 | 0.5 | 1 | 2 | 4 | 8 | 16 | 32 | 64 | 128 | 240 | +Inf |
|------|------|------|------|------|------|------|------|------|------|------|------|------|------|
| Upper bound (ms) | 100 | 250 | 500 | 1000 | 2000 | 4000 | 8000 | 16000 | 32000 | 64000 | 128000 | 240000 | — |

**Prometheus scrape configuration** (scrape every instance):

```yaml
scrape_configs:
  - job_name: cuberouter
    metrics_path: /api/metrics          # /api/v1/metrics and /api/v2/metrics also work
    bearer_token: "<export_token 的值>"
    static_configs:
      - targets:
          - "instance-1.example.com:3000"
          - "instance-2.example.com:3000"   # one target per gateway instance / pod
```

Example alerting queries: latency percentiles use PromQL interpolation — `histogram_quantile(0.95, sum by (le) (rate(cuberouter_relay_latency_seconds_bucket[5m])))`; overload 503 share: `rate(cuberouter_overload_rejects_total[5m]) / rate(cuberouter_relay_attempts_total[5m])`.

### Value semantics (数值口径)

- Percentile fields in `/api/perf-metrics` (`p50_latency_ms`, `p95_latency_ms`, `p99_latency_ms`, `p95_ttft_ms`) are **-1 when the bucket has no histogram data** — including legacy rows written before the histogram upgrade, which have counts but zero histogram cells.
- Built-in percentiles are **histogram crossing-bound estimates, without interpolation**: the quantile is reported at a cell boundary of the histogram. A quantile that falls in the tail cell (samples > 240 s) is reported as `240000` ms (documented upper-cap approximation).
- `success_rate` fields are **0..100 percent**, not a 0..1 fraction.
- On startup the migration auto-extends `perf_metrics` (`channel_id`, `channel_name`, `lat_b0..12`, `ttft_b0..12`), creates `capacity_metrics`, and drops the legacy `(model, group, bucket_ts)` unique index behind a per-dialect existence check. The MySQL/PostgreSQL migration branches are currently covered by automated tests on SQLite only — run a manual smoke test against real MySQL/PostgreSQL before release.

---

## 🔗 Related Projects

### Upstream Projects

| Project | Description |
|------|------|
| [One API](https://github.com/songquanpeng/one-api) | Original project base |
| [Midjourney-Proxy](https://github.com/novicezk/midjourney-proxy) | Midjourney interface support |

### Supporting Tools

| Project | Description |
|------|------|
| [new-api-key-tool](https://github.com/Calcium-Ion/new-api-key-tool) | Key quota query tool |
| [new-api-horizon](https://github.com/Calcium-Ion/new-api-horizon) | New API high-performance optimized version |

---

## 📜 License

This project is licensed under the [GNU Affero General Public License v3.0 (AGPLv3)](./LICENSE).

Additional terms under AGPLv3 Section 7 apply. Modified versions must preserve
the author attribution notice `Frontend design and development by New API
contributors.` in the appropriate legal notices and in any prominent about,
legal, footer, or attribution location presented by the user interface.

Modified versions that present a user interface must also preserve a visible
link to the original project: <https://github.com/QuantumNous/new-api>.

This is an open-source project developed based on [One API](https://github.com/songquanpeng/one-api) (MIT License).

If your organization's policies do not permit the use of AGPLv3-licensed software, or if you wish to avoid the open-source obligations of AGPLv3, please contact us at: [support@quantumnous.com](mailto:support@quantumnous.com)

---
