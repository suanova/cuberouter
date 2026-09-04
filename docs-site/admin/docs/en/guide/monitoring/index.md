# Monitoring & Operations

This guide brings together every place in CubeRouter for observing gateway capacity, model and channel performance, and upstream availability, together with the related configuration options and measurement semantics. It is aimed at administrators who troubleshoot service quality, plan capacity, or want to feed metrics into an external monitoring stack.

Monitoring is organized around three viewpoints:

| Viewpoint | What it covers | Where to find it |
|-----------|----------------|------------------|
| Gateway capacity & health | Performance health indicators, Capacity (24h) cards, channel breakdown | **Dashboard** in the left navigation |
| Model & channel performance | Model card badges, the model detail **Performance** tab, per-channel detail | **Model Square** → model detail |
| Upstream & instance availability | Uptime Kuma groups, model availability events | Dashboard, model detail **Performance** tab |

::: warning Permissions
The capacity view on the Dashboard is available to administrators only. **Channel information is internal**: the server strips channel fields from responses based on the caller's role, so non-admin responses never include channel detail. Performance-related settings can only be changed by **Super Admin (Root)**.
:::

## Quick Navigation

| Topic | Description |
|-------|-------------|
| [Capacity & Health](#capacity-health-dashboard) | Reading the health indicators and Capacity (24h) cards |
| [Model Trends](#model-trends-model-square-and-model-detail) | The model detail Performance tab and availability events |
| [Configuration](#configuration) | Model performance metrics and system performance monitoring settings |
| [Prometheus Export](#prometheus-metrics-export) | Enabling, metric reference, and scrape examples |
| [Semantics & Approximations](#semantics-approximations) | Measurement semantics and caveats |
| [Related Entry Points](#related-entry-points) | Logs, system info, and other troubleshooting tools |

---

## Capacity & Health (Dashboard)

After signing in, open the **Dashboard**. The performance health area sits at the top, followed by the Capacity (24h) cards and the channel breakdown.

### Performance Health Indicators

The health area shows three core gateway-wide metrics:

- **Success rate**: share of successful requests, 0-100
- **Average latency**: mean request latency
- **Throughput**: requests completed per unit of time

Below it, badges list the **Top 5 models by request volume**, each showing that model's success rate/latency/throughput. When there are more models, the rest collapse into a "+N Models" chip that expands on click.

### Capacity (24h) Cards

The capacity cards always cover the last 24 hours and consist of four sparkline cards:

| Card | Meaning |
|------|---------|
| **RPS** | Requests per second. For each aggregation bucket, RPS = attempts in the bucket ÷ bucket duration (seconds). The granularity follows the configured **Aggregation bucket** — the coarser the bucket, the closer the value is to a plain average over the span |
| **Inflight peak** | Peak number of requests in flight, sampled once every 2 seconds. It is an approximation — short spikes shorter than the sampling interval can be missed |
| **Rejected 503** | Rejections by overload protection. When resource usage exceeds the configured thresholds, new relay requests are rejected with HTTP 503 (thresholds are set under **System Settings → Operations → Performance → System Performance Monitoring**) |
| **Rate limit 429** | Rejections by model-level rate limiting. Requests that hit a model's rate-limit ceiling and return 429 are counted here |

Periods without data are not drawn (the API returns -1 for empty buckets and the UI hides those points), so do not read an empty gap as zero traffic.

### Channel Breakdown (admin only)

After selecting a model, the channel breakdown lists that model's requests, average latency, P95 latency, success rate and TPS per channel — useful for finding out which channel is behind a model that is slow overall or whose failure rate is climbing. The card is visible to admins only: channel fields are stripped from non-admin responses on the server.

Typical reading patterns:

- Average latency and P95 rising together on one channel → the upstream is slowing down
- Success rate collapsing on one channel while others stay healthy → investigate that upstream or its credentials first
- A channel with traffic but zero TPS → check whether it was auto-disabled

---

## Model Trends (Model Square and Model Detail)

### Model Square Badges

Each model card in the **Model Square** shows success rate, average latency and throughput badges for quick comparison. Badge data follows the same aggregation-bucket and flush-cycle behavior as everything else; a freshly enabled model may show no data until its first complete bucket exists.

### Model Detail — Performance Tab

Open a model card and switch to the **Performance** tab, which contains:

- **Metric overview**: TPS, average latency, success rate
- **Per-group performance table**: the same metrics broken down per user group, to compare how different callers (groups) perform
- **Latency trend (last 24h)**: average TTFT together with P95/P99 percentile lines
- **Availability (last 24h) and 30-day daily uptime**: event bars mark aggregation buckets with availability anomalies over the window, summarized as "N incidents totalling X minutes"

### Measurement Semantics

- Latency and TTFT come from a millisecond histogram. P50/P95/P99 are **estimates based on histogram cell spans, without interpolation inside a cell**. Samples over 240 seconds fall into the final cell and count as 240,000 ms
- Availability events are bucketed by the aggregation window: a bucket with requests and an anomaly (e.g. a surge in failures) is an anomalous bucket, and adjacent anomalous buckets merge into one incident
- Buckets with no requests are excluded from calculations and shown as empty

---

## Configuration

Performance metrics and overload protection are configured on the **System Settings** page (log in with a Root account).

### Model Performance Metrics (Monitoring & Alerts)

System Settings → **Operations** category on the left → **Monitoring & Alerts** section → the **Model performance metrics** area:

| Option | Default | Description |
|--------|---------|-------------|
| Enable model performance metrics | On | Controls collection, database flushing and retention cleanup. When off, the Dashboard and model performance pages stop updating, but in-process counters keep running and export counters continue from process start once re-enabled |
| Flush interval (minutes) | 5 | How often in-memory buckets are written to the database; one component of data lag |
| Aggregation bucket | 1 hour | Aggregation granularity — 1 minute, 5 minutes or 1 hour. The Capacity (24h) cards and model performance charts share this granularity; finer buckets show fresher, more detailed movement but cost more storage and database writes |
| Retention days | 0 (keep forever) | How many days of performance data to keep. 0 means data is kept permanently; **set an explicit value in production** (e.g. 30-90 days) so the metric tables do not grow without bound |
| Metrics Export Enabled | Off | Whether the Prometheus text-format export endpoint is exposed (see below) |
| Metrics Export Token | Empty | Bearer token required to access the export endpoint; leave empty to disable auth and restrict access at the network layer instead |

### System Performance Monitoring (Performance)

System Settings → **Operations** category on the left → **Performance** section → **System Performance Monitoring** area:

| Option | Default | Description |
|--------|---------|-------------|
| Enable Performance Monitoring | On | Monitors resource usage and enforces overload protection when enabled |
| CPU Threshold (%) | 90 | Usage above this counts as overload |
| Memory Threshold (%) | 90 | Usage above this counts as overload |
| Disk Threshold (%) | 90 | Usage above this counts as overload |

With monitoring enabled, when any resource exceeds its threshold, **new relay requests are rejected with HTTP 503** (in-flight requests are unaffected) until usage drops back below the threshold. Rejections are counted in the Dashboard's **Rejected 503** card and exported as `cuberouter_overload_rejects_total`.

### Recommendations

- **Bucket granularity vs. data lag**: performance data lags by at most about "1 aggregation bucket + 1 flush interval". With a 1-hour bucket and 5-minute flush, the most recent 1-2 points on the Dashboard are usually still incomplete. Switch to 1-minute or 5-minute buckets when you need to see trends sooner
- **Fine granularity costs more**: a 1-minute bucket generates roughly 60x the data of a 1-hour bucket. The 1-hour bucket is enough for day-to-day capacity watching; switch finer temporarily when chasing an incident
- **Retention**: the default of 0 keeps everything forever; choose a retention window that matches your audit and troubleshooting needs
- **Overload thresholds**: CPU/memory thresholds around 85-95 and disk around 90-95 are a reasonable starting point; make sure they match container/host resource limits (e.g. when a container memory limit is below 100%)

---

## Prometheus Metrics Export

With **Metrics Export Enabled**, the platform exposes process-level metrics in Prometheus text format that Prometheus can scrape directly.

### Enabling

1. Go to **System Settings → Operations → Monitoring & Alerts**
2. Make sure **Enable model performance metrics** is on
3. Turn on **Metrics Export Enabled**
4. Optionally set a **Metrics Export Token** for request authentication
5. Click **Save**

### Endpoint & Auth

- Endpoint: `GET /api/metrics` (also mounted at `/api/v1/metrics` and `/api/v2/metrics`)
- When export is off the endpoint always returns **404** (to avoid revealing its existence)
- With a token configured, requests must carry `Authorization: Bearer <token>`; a wrong or missing token returns 401
- An empty token means no authentication — **anyone who can reach the address can read the metrics**. On public or untrusted networks, set a token or restrict the endpoint at the network layer (firewall / reverse-proxy ACL)

Example request:

```bash
curl -H "Authorization: Bearer <metrics-export-token>" https://gateway.example.com/api/metrics
```

### Metric Reference

Every exported metric is a view of a **single gateway process** — nothing is aggregated across instances. Relay metrics are labeled by model + group and carry **no channel dimension** (per-channel cardinality is too high to export as labels).

| Metric | Type | Meaning |
|--------|------|---------|
| `cuberouter_relay_attempts_total` | counter | Total attempts to enter relay handling, including requests rejected by auth, rate limiting or overload (no labels) |
| `cuberouter_relay_requests_total` | counter | Total completed relay requests (incl. failures); covers only requests that entered relay handling and were recorded. The gap versus attempts is the share rejected before entering relay |
| `cuberouter_relay_latency_seconds` | histogram | Relay request latency (`_bucket`/`_sum`/`_count`); derive percentiles with `histogram_quantile` |
| `cuberouter_relay_ttft_seconds` | histogram | Time-to-first-token of relay requests (`_bucket`/`_sum`/`_count`) |
| `cuberouter_inflight_requests` | gauge | Relay requests currently in flight (process-local live value, no labels) |
| `cuberouter_overload_rejects_total` | counter | Cumulative rejections by overload protection (HTTP 503, no labels) |
| `go_goroutines` | gauge | Number of Go goroutines |
| `go_memstats_alloc_bytes` | gauge | Bytes of Go heap memory currently allocated |
| `go_memstats_heap_objects` | gauge | Number of objects on the Go heap |

The latency and TTFT histograms use the following `le` boundaries (seconds): `0.1, 0.25, 0.5, 1, 2, 4, 8, 16, 32, 64, 128, 240, +Inf` (100 ms up to 240 s). Samples over 240 seconds count toward `+Inf` only.

### Scrape Example

```yaml
scrape_configs:
  - job_name: cuberouter
    metrics_path: /api/metrics
    scheme: https
    authorization:
      # Match the Metrics Export Token; remove this block when no token is set
      credentials: <metrics-export-token>
    static_configs:
      # Multi-instance deployments: one target per gateway instance (pod)
      - targets:
          - gateway-1.example.com
          - gateway-2.example.com
```

With multiple instances, scrape **per instance (pod)**, because:

- All exported metrics accumulate in process memory, so counters reset to zero on restart (normal counter behavior)
- Values are independent per instance — do not sum instances before computing rates. Use `up` and the `go_*` metrics if you want per-instance health comparison

### Notes

- Model-level 429 rate-limit rejections from the capacity cards are **not** part of the export; only the metrics listed above are exposed
- The export shares the same in-process counters as metric collection. When collection is disabled the exported counters freeze (they stop growing with new requests) and resume once it is re-enabled
- Retention cleanup does not affect the in-process export counters; export only lives as long as the process

---

## Semantics & Approximations

These conventions apply across most charts on this page; it helps to read them before interpreting data:

| Item | Description |
|------|-------------|
| Aggregation bucket | Capacity cards, model performance charts and availability events are all aligned to the configured **Aggregation bucket** (1 minute / 5 minutes / 1 hour) |
| Data lag | Dashboard data lags by at most about 1 aggregation bucket + 1 flush interval; a bucket appears only after it has been flushed to the database |
| Attempts definition | Requests that entered the relay handling path, **including** ones rejected by auth, rate limiting or overload; each is attributed to the bucket of its **completion time**. RPS is derived from this count |
| Inflight peak | Sampled once every 2 seconds; short spikes shorter than the interval can be missed |
| P50/P95/P99 | Cell-span estimates over the histogram, no interpolation inside cells; samples above 240 s count as 240,000 |
| Missing data | The API returns -1 for buckets without data and the UI does not render those points — treat empty spots as "no data", not zero |
| Success rate | A 0-100 value; buckets without requests are excluded |
| Multi-instance | Dashboard and model performance data aggregate instances through the shared database; the Prometheus export is a per-process view |

---

## Uptime Kuma Probing (Separate Deployment)

### How the two pieces relate

Uptime Kuma is an **independently deployed, open-source uptime probe** — CubeRouter does not bundle or start it. Division of labor: **Kuma performs scheduled probes (and alerting) of external targets, including the gateway itself; CubeRouter aggregates each Kuma group's probe results onto the Dashboard for display.** Probing and display are decoupled — a Kuma outage does not affect gateway proxying and vice versa.

### Deploying Kuma

```bash
docker run -d --name uptime-kuma -p 3001:3001 \
  -v uptime-kuma-data:/app/data \
  louislam/uptimekuma:1
```

Open `http://<host>:3001`, create the admin account and initialize. Deploy Kuma **outside the gateway** (a separate host or publicly reachable location) so it reflects the real user path (DNS/CDN/bandwidth/certificates); on the same machine it can only prove the process is alive.

### Integration steps

1. **Create monitors in Kuma**: add the targets you care about as HTTP probes — e.g. the gateway `https://<gateway-domain>/api/status` (200 = alive), upstream API domains, TLS certificate remaining days — with a probe interval of your choice;
2. **Configure alerting in Kuma**: notification channels (Telegram/email/Webhook etc.) are configured inside Kuma; CubeRouter does not send alerts;
3. **Enable a public status page in Kuma**: assign monitors to a status-page group and set it to **Public**, then note the page **slug** (the `my-status` in `/status/my-status`). The page must be public so CubeRouter can read it without credentials;
4. **Wire it up in CubeRouter**: go to **System Settings → Console Content** → **Uptime Kuma**, enable it, then **Add Group** with a **Category Name** (for display), the **Kuma instance URL** (no trailing slash, e.g. `https://status.example.com`) and the **Status Page Slug**; save;
5. The Dashboard's **Uptime panel** then shows each monitor's name, status and uptime percentage per group.

Up to 20 groups are supported; one group maps to one public status page on Kuma. If nothing shows up, first check that the page is Public and that the URL/slug match the status page address.

---

## Related Entry Points

Once monitoring data points at an anomaly, these pages help you drill down:

| Entry point | Purpose |
|-------------|---------|
| [Logs & Statistics](../log/index) | Usage logs (per-request details, failure reasons) and task logs to confirm what error an abnormal request actually hit |
| System Info | The instance list shows the running state of each gateway instance — first stop when an instance looks offline or versions are inconsistent in a multi-instance setup |
| Uptime Kuma | See "Uptime Kuma Probing (Separate Deployment)" above: after configuring groups under **System Settings → Console Content**, the Dashboard shows probe status per group — useful for dependencies beyond the gateway itself |
| Operation logs | Audit trail for admin actions. Sensitive operations such as deleting a channel are recorded there (channel deletion appears as type 3); compare the timeline against the capacity cards to see whether a config change correlates with a metric anomaly |

---

**Related documents**: [System Settings](../system/index) | [Logs & Statistics](../log/index) | [Model Management](../model/index) | [Channel Management](../channel/index)
