# Native Image Studio server jobs

This feature is implemented in CubeRouter's current Go relay and React/TypeScript Media Studio. It does not import searouter's gateway, React pages, Python dispatcher, deployment manifests or channel database. It independently implements the established image-worker and ShieldGemma wire contracts.

The host application's Images/Video navigation, model tags, ordinary providers, organization fallback, pricing, usage logs, advanced settings, ten-reference limit and browser history remain in use. Server jobs apply only to explicitly configured image models. Video cancellation is outside this change.

## Destination configuration

| Setting | Meaning |
| --- | --- |
| `IMAGE_STUDIO_ENABLED=true` | Enable server jobs; disabled by default. Enabled jobs always require output review. |
| `IMAGE_STUDIO_MODELS=qwen-image-2.1` | Exact comma-separated destination model names. |
| `IMAGE_STUDIO_CHANNEL_IDS=101,102` | At most 64 destination worker channel IDs; one physical GPU per channel. These channels are reserved from ordinary relay traffic, including when admission is disabled. |
| `IMAGE_STUDIO_ALLOWED_GROUPS=image-studio` | Account-group allowlist; default `image-studio`; explicit empty value denies all accounts. |

Create worker channels using OpenAI type **1** or Image Edit (Base64) type **63**, with explicit model/group abilities, one service key of at least 32 characters, and the service root as base URL. Multi-key channels and nonempty parameter/header overrides are excluded so actual parameters, control identity and audit agree. Native model mappings and pricing still apply. Add the ordinary `text-to-image` / `image-to-image` metadata tags for model discovery.

Create reviewers with private channel type **65**, **ShieldGemma 2 (Image review)**. CubeStack remains **64**; Advanced Custom remains **58**. Use model `google/shieldgemma-2-4b-it`, the complete HTTPS `/v1/moderate` URL and one service key of at least 32 characters. Reviewer keys reject embedded whitespace and invalid HTTP header characters. Other accepts an optional PEM CA bundle. Review channels cannot relay user generation/chat requests. Their Test action submits a small synthetic PNG to the actual reviewer contract.

Private services require verified HTTPS, or loopback HTTP for a local service/SSH tunnel. Browser headers cannot select private control destinations or credentials. Removing the last reviewer pauses admission and cannot enable unchecked delivery. Configure destination records independently; do not copy HKBN databases, keys or channel IDs.

## Execution and output review

Worker selection reads current channel/ability records from the main DB and checks authenticated `GET /health`: `state=ready`, `gpu_count=1`, `control_protocol=qwen-execution-v1`. Priorities and durable least-recently-used timestamps select available workers. Conditional main-DB leases fence physical GPUs, accounts and reviewer replicas across gateways.

Inference uses the native image relay once, with a server-generated `X-Image-Execution-ID`. Cancellation sends authenticated `POST /internal/cancel` with `{"execution_id":"..."}` to the same worker. Only matching `qwen-execution-v1` acknowledgements in `cancelling` or `fenced` state are accepted. Control retries once per second; generation never retries. Compatible workers must bound stalled execution and advertise readiness only after cleanup. This change does not install GPU worker code or model weights.

The output gate runs **before** provider response delivery and billing settlement. It accepts one bounded PNG in `data[0].b64_json`; URLs and malformed/oversized results fail closed. Review must include `image-moderation-v1`, verified ShieldGemma weights, a 40-character model revision, the exact final PNG SHA-256, and all three `sexual` / `violence` / `dangerous` scores, with threshold exactly **0.5**. Missing or contradictory fields, wrong hashes, negative decisions and unavailable reviewers prevent release. Availability failures may try another reviewer; negative/malformed verdicts are terminal.

No unreviewed image reaches the browser, thumbnails or IndexedDB. Audit persistence is also required before release. The internal relay and failed result fetch return HTTP 422 with `output_blocked` / `output_check_unavailable`. Native pre-consume is refunded on those failures. Existing native pricing and violation-fee policies otherwise apply.

## Queue, cancellation and recovery

- One active execution per account; at most ten waiting jobs globally. A 256 MiB retained-byte budget includes input JSON and a 12 MiB output reservation per outstanding job; memory pressure can reduce admission below count limits.
- Request JSON: 16 MiB maximum. At most ten references, 10 MiB each, 11 MiB decoded aggregate including any region mask; server-side validation repeats the checks. Final PNG: 12 MiB maximum.
- Waiting TTL: 15 minutes. Terminal identity/result TTL: ten minutes. Acknowledgement releases result bytes only after the browser's history transaction succeeds.
- Five-second browser polling shows queue position. Existing native rate limiting enforces IP 2400/60s and account 120/60s on Studio routes.
- Queued models use 20/40/50 quality steps, CFG 1 and seed 42 unless overridden. Explicit zero CFG/seed values survive. New editing references and continued edits choose fresh seeds that users can override.
- Ordinary queued edit references retain uploaded detail within the server byte bound. Drawing/mask paths use normalized references for alignment.
- Existing account-scoped history upgrades from IndexedDB version 1 to 2 with a pending-request store. An atomic claim prevents simultaneous submissions from two tabs. A late acknowledgement cannot erase another tab's newer recovery record. Final history writes check the same request key and cancellation intent in one transaction; cancelled or replaced recovery records cannot publish artwork.
- The key and full draft are committed **before** the sole POST. Reconnection looks up the key; it never replays inference. A changed gateway epoch or missing expired job requires explicit draft review. Saved cancellation intent retries on reconnect and wins over a completed response before download.
- Waiting cancellation releases input immediately. Running cancellation can become logically terminal after 30 seconds, while retaining input accounting, account and physical slot until the execution callback settles. Late images are discarded. Transport/5xx uncertainty quarantines physical/account leases for up to 15 minutes; new admission receives `studio_account_busy` while fenced.

### Replicas and deployment

Bodies/results are RAM-backed. Use sticky routing for `/api/media-studio/image-jobs` to the admitting gateway, or run that path on one gateway. Shared DB capacity leases do not make job lookup distributed. Gateway restart changes the epoch and loses RAM jobs; the browser preserves the draft and does not automatically replay uncertain inference. Durable leases continue fencing the old execution.

Drain queued/running jobs before replacing the admitting gateway. Enable admission only after matching frontend, gateway, workers and reviewers pass staging acceptance. Rollback requires disabling admission, draining work and deliberately removing reserved channel IDs before returning those channels to ordinary relay traffic.

## Native editing and examples

The existing rectangular mask editor is preserved. The new reference editor adds rectangles, filled lassos, brush width, coordinate entry, undo/redo, mask-only view and blue/red/green/white guides. A region mask is composited server-side against the first normalized reference, preserving outside pixels. That composite, not the model candidate, is reviewed and hashed. Strict-composite dimensions follow the normalized reference. This is not a claim that the worker implements masked diffusion.

Colored guides accompany the user's instruction and a request to remove guide marks; other areas may change. Original and actual model-facing instructions are audited separately. Drawings are bound to the first reference identity; unsupported model/reference changes discard them from submission. Canvas/decode failure stops submission.

Seven independently authored recipes use attributed official Qwen illustration assets: two text exercises and five reference exercises, including ten inputs. Crop rectangles select input tiles only; official result panels never enter generation. See [asset sources and hashes](../web/public/studio-examples/qwen21/SOURCES.md). Prompts are adapted exercises, not exact official generation settings.

## Audit and HTTP API

Root users open **Image output audit** at `/admin/image-output-audit`. Main-DB `studio_image_audits` stores job/user/model, original/model prompts, parameters, ordered input hashes, selected channel, final PNG hash, review and state/error. It excludes image bytes and service keys. Retention is 30 days, with startup cleanup and a five-minute janitor. The separately configured log database is unchanged.

| Method | Path | Purpose |
| --- | --- | --- |
| GET | `/api/media-studio/config` | Existing configuration plus queue model/access flags |
| GET | `/api/media-studio/image-jobs/session` | Epoch and this account's retained jobs |
| POST | `/api/media-studio/image-jobs` | `{mode,user_prompt,request}` with native image fields; requires `Idempotency-Key` and `X-Trial-Epoch` |
| GET | `/api/media-studio/image-jobs/{id-or-key}` | Owned status and queue position |
| POST | `/api/media-studio/image-jobs/{id}/cancel` | Idempotent cancellation |
| GET | `/api/media-studio/image-jobs/{id}/result` | Approved final PNG only |
| POST | `/api/media-studio/image-jobs/{id}/ack` | Release collected result |
| GET | `/api/image-studio-audit` | Root-only records; optional `user_id`, `before`, `before_job_id` cursor |

The former synchronous `/pg/images/*` rejects configured queued models. Other providers retain their native path. Ordinary API clients need separate general-purpose channels rather than the reserved Studio pool.

## Verification and acceptance

Automated contracts cover ownership, bounded queueing, expiry, idempotency, cancellation without early capacity release, late-output discard, explicit zero parameters, strict region preservation, review binding and browser recovery races. A real native Go relay with simulated HTTP worker/reviewer services verifies one inference per job and nonzero wallet pre-consume, settle and refund. It does not establish GPU quality or production reachability.

`go test ./model -run TestStudioImageDatabaseContracts -count=1 -v` uses SQLite and configured disposable MySQL/PostgreSQL (`TEST_MYSQL_DSN`, `TEST_POSTGRES_DSN`) to check representative existing account/channel data, fresh Studio schemas, repeated migration, leases and audit constraints. CI supplies MySQL 8.0.46 and PostgreSQL 16.15 and reports actual versions. This is representative-schema upgrade coverage, not an old-release-binary boot.

Before production admission, verify create/edit/ten references, region/guides, refresh recovery, queued/running cancellation, real reviewer allow/block/outage, exact audit hashes, configured pricing, worker readiness after cancel and sticky routing. Keep HKBN production and its GPU allocations separate from the destination rollout.
