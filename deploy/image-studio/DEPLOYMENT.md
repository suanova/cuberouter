# Deploy Image Studio with CubeRouter

Deploy the frontend, ALL Go gateways and CPU dispatcher from the **same commit**.
Merge alone does not enable the service. The UI is `/media-studio`; root audit
and API review policy are at `/image-studio-audit`.

## Service configuration

Use the existing database/network. Add one CPU-only dispatcher built from
`deploy/image-studio/Dockerfile`. CI builds
`harbor.isuanova.com/suanova/cuberouter-image-studio` alongside the gateway.
Use matching commit tags. This container needs neither weights nor GPUs.

All public gateways send jobs to ONE dispatcher. Execution, capacity and recovery
callbacks must reach ONE private Go gateway, not a load balancer rotating between
replicas. That gateway owns generation reservations and shares the main database.
In a single-gateway deployment the existing gateway can perform both roles.

| Service | Configuration |
| --- | --- |
| All Go gateways | `IMAGE_STUDIO_DISPATCHER_URL=http://image-studio-dispatcher:18195` |
| Go + dispatcher | `IMAGE_STUDIO_DISPATCHER_TOKEN`: same random secret, 32+ characters |
| Go + dispatcher | `IMAGE_STUDIO_RELAY_TOKEN`: different random secret, 32+ characters |
| Go | `IMAGE_STUDIO_CHANNEL_MODEL=qwen-image-2.1` |
| Go | `IMAGE_STUDIO_CHANNEL_IDS=auto` |
| Go | `IMAGE_STUDIO_AUDIT_ENABLED=true` |
| Go | `IMAGE_STUDIO_ALLOWED_GROUPS=image-studio` |
| Dispatcher | `IMAGE_STUDIO_RELAY_URL=http://PRIVATE-GATEWAY:3000` |
| Dispatcher | `IMAGE_STUDIO_RELAY_CONCURRENCY=auto` |

Keep dispatcher and `/internal/image-studio/*` private. Use a trusted private
network or authenticated TLS across hosts. Store service secrets in deployment
secret management; they are not Hugging Face or admin API keys. Preserve existing
database/session/billing settings. See `compose.example.yaml` or run
`configure_deployment.py --help` for an optional existing-Compose overlay helper.
Pass the actual gateway service name for your deployment.

## Channel management and access

At `/channels`, register each Qwen GPU worker as an OpenAI-compatible channel:
its reachable endpoint/key, model `qwen-image-2.1`, and group `image-studio`.
The worker must implement the bridge contract in `qwen_channel_bridge.py`,
including authenticated `/health` with `state=ready`, `gpu_count=1`, generation
and multipart editing. Arbitrary upstream APIs may need an adapter.

In **Model Metadata**, create/update `qwen-image-2.1` with BOTH capability tags
`text-to-image` and `image-to-image`. Media Studio discovers models from these
tags in the pricing catalog, not from the model name. Configure the model price
and allowed group using the existing CubeRouter settings. Without these tags,
an otherwise working API channel does not appear in the Studio model selector.

Add/remove/enable/disable matching channels in the UI: capacity follows live
channels without an MR or hardcoded node IP/count. Assign test users to the allowed
account group; admin role alone does not grant image access. Organization API keys
use the organization's account group, membership checks and billing. Browser
Studio uses the signed-in user's group. LLM routing/billing remain unchanged.

Add `ShieldGemma 2 (Image moderation)` channels with the full HTTPS `/v1/moderate`
URL, dedicated service key and optional public CA certificate. The UI fills the
private model/group. This fork uses type **65**, because 64 is CubeStack and 58 is Advanced
Custom. Do not copy HKBN channel type numbers or databases wholesale.
See [reviewer protocol](../image-moderation/README.md). No hardcoded reviewer IP
is required. Database leases serialize use of each reviewer across Go replicas.
Review requests wait in bounded queues; failure/timeout refuses delivery. After a
reviewer is first registered, removing the last reviewer does not disable review.

API moderation is independently set per account group at `/image-studio-audit`:
`external` by default (integrator responsible) or `required` (Gemma must approve).
[API-QUEUE.md](API-QUEUE.md) covers 202 polling, errors and inference charging.

## Upgrade and acceptance

Drain jobs and collect outputs before restarting the dispatcher: jobs/results
are RAM-only. Deploy the matching components together; avoid mixed versions.
Then validate generation, references, colored edits, denied accounts, prompt/hash
and moderation audits, ordinary token service, and multiple concurrent accounts.
Test worker/reviewer outages in staging before production.

Default bounds: one unfinished job per account, ten waiting jobs, 256 MiB queue
RAM, 15-minute queue expiry and ten-minute output collection. Full admission
queues reject explicitly. Capacity can follow up to 64 channels; that is a
configuration ceiling, not a measured throughput guarantee. This sync does not
resolve or certify the earlier 30-worker large-image deployment issue.

## Storage

Per-origin/account IndexedDB stores history (50 jobs / 100 MiB, no timed expiry).
Different test/production domains have separate browser histories. The dispatcher
holds temporary images in RAM. The main database retains identities, prompts,
settings, hashes and moderation reports for 30 days with expiry filtering and
cleanup; no image bytes enter the audit. Usage-log and backup retention remain
operator-controlled.
