# Image Studio output review

For new deployments, manage Gemma endpoints, dedicated keys and private CA certificates through **Channel Management**. See [Channel setup and migration](CHANNELS.md). The environment-only endpoint and fixed replica pool below remain legacy compatibility options until the first moderation channel is saved.

Image Studio holds each generated PNG inside CubeRouter until ShieldGemma 2 has
checked sexual, violence/gore and dangerous content. A score >= 0.5 on any policy
blocks release. A timeout, invalid response, incomplete scores, unavailable model
or failed configured audit write also withholds the image. Users see a refusal
for a content decision and a separate temporary-error message for an outage.
No rejected image, thumbnail or download reaches the dispatcher/browser library.

```text
Browser -> RAM dispatcher -> CubeRouter channel -> Qwen worker
                                  <- generated PNG
                            authenticated ShieldGemma API
                                  -> 30-day audit metadata
                            allowed PNG OR terminal error -> dispatcher -> browser
```

This gate covers Studio generation and editing, including templates and colored
strokes. It does not moderate prompts, reference uploads, existing browser images,
or API groups using the default `external` review policy. Chat/token routing is unchanged.
When enabled, the UI disables the legacy browser mask-composite operation (and
refuses old pending mask drafts) because it alters pixels after server review.
Colored-stroke and whole-image editing remain available.

## Configure output review

For new deployments, register each running classifier in **Channels** using
**ShieldGemma 2 (Image moderation)**. See [CHANNELS.md](CHANNELS.md) for keys,
TLS certificates, shared waiting, failover and the persistent fail-closed policy.
No GPU IP or number of classifiers is compiled into CubeRouter. The gateway and
CPU-only dispatcher do not need GPUs.

The legacy single-endpoint deployment remains supported until the first
moderation channel is saved:

```dotenv
IMAGE_STUDIO_MODERATION_ENABLED=true
IMAGE_STUDIO_MODERATION_URL=https://review.example.internal/v1/moderate
IMAGE_STUDIO_MODERATION_TOKEN=<dedicated classifier service secret>
IMAGE_STUDIO_AUDIT_ENABLED=true
```

Inject variables into the gateway container, and use the deployment's secret
manager for the classifier key. It is unrelated to an HF download token or
CubeRouter administrator/API-token credentials. Keep TLS verification enabled.
The legacy classifier admits eight requests, serializes inference and waits at
most 30 seconds for its GPU. This means eight admitted requests, not eight
parallel GPU inferences. The gateway review deadline is 45 seconds.

Check authenticated health and a benign image from the gateway's network, then
verify the returned hash, scores and decision in **Image audit**. Audit metadata
expires after 30 days; images are not stored in the audit table. Existing records
without moderation fields were not reviewed. An unavailable classifier withholds
output; it does not label the content a policy violation.

Browser review and API review are separate policies. API requests default to
`external` (the integrator handles review). A root administrator can require
Gemma for an account group in **Image audit**. See
[API-QUEUE.md](../image-studio/API-QUEUE.md).

## Two-replica review queue

`pool.py` adds a CPU-only, authenticated review endpoint in front of existing
classifier replicas. Run **one pool process**, and dedicate each classifier to
that pool. Each replica consumes one job at a time from the shared FIFO. The
pool admits at most 64 images / 256 MiB of image bytes, including active checks;
72 bounded HTTP threads read at most 12 MiB each with a ten-second upload timeout.
Allow 2 GiB container memory for uploads, retained results and Python overhead.
No image is written to disk. Review expires after 35 seconds, within the gateway's
45-second timeout. A transport failure can retry classification on the other
replica; generation is never retried. Both replicas failing or queue overload
returns a fail-closed error. This does not guarantee zero rejection under outages
or unlimited load, and the pool process itself is not highly available.

Mount a private JSON config (not committed with real secrets) such as:

```json
{"replicas":[
  {"url":"https://gemma-a.internal:18460/v1/moderate","token_file":"/run/secrets/gemma-a","ca_file":"/run/tls/a-ca.crt"},
  {"url":"https://gemma-b.internal:18460/v1/moderate","token_file":"/run/secrets/gemma-b","ca_file":"/run/tls/b-ca.crt"}
]}
```

Set `MODERATION_POOL_REPLICAS_FILE`, `MODERATION_POOL_TOKEN_FILE` (a separate
32–256-character ASCII secret), `MODERATION_POOL_BIND`, `MODERATION_POOL_PORT`,
`MODERATION_POOL_TLS_CERT` and `MODERATION_POOL_TLS_KEY`; run `python -B pool.py`.
TLS is mandatory off loopback. Gateway `IMAGE_STUDIO_MODERATION_URL` points to
the pool's `/v1/moderate`, and `IMAGE_STUDIO_MODERATION_TOKEN` holds the pool token.
Install the private CA in the gateway's trust store. Never turn off TLS verification.
Authenticated `/v1/health` reports queue occupancy and completed/failed counts;
it describes pool liveness, not individual classifier readiness. Monitor each
classifier's authenticated health separately. CPU regression command:
`python -m unittest discover -s deploy/image-moderation -p test_pool.py -v`.

The gateway still requires a **single active image execution process**, as
documented in the Studio deployment guide. Send capacity, execute and reconcile
callbacks to that same instance, and route image token requests sharing these
GPUs through it. The review pool does not repair cross-gateway GPU reservations
or execution recovery. Ordinary chat/token backends can retain their replicas.

## Classifier runtime details

`service.py` is a standalone authenticated FastAPI service; it can also be added
to the private PoC app with `app.include_router(router_for(runtime))` to reuse its
already-loaded GPU model. Bind to loopback or a private network behind TLS. Never
expose the PoC `/api/moderate` or `/api/reload` routes publicly.

For a standalone installation, provide the existing authorized model snapshot,
`SHIELDGEMMA_MODEL_DIR` (default `models/shieldgemma-2-4b-it`) and
`SHIELDGEMMA_TOKEN_FILE` (a mode-0600 file containing a separate 32–256-character
ASCII secret). Install `requirements-api.txt`, Transformers **4.57.6**, Accelerate
and the GPU vendor's compatible PyTorch. Run one Uvicorn process, without reload
or access logs: `python -m uvicorn service:app --host 127.0.0.1 --port 18440
--no-access-log`. Do not start multiple model-loading workers on the same GPU.

The imported implementation was tested with MetaX C500, model
`google/shieldgemma-2-4b-it`, revision
`eaf60452b5fc41a911338a022e628b0c15283897`, Torch 2.6.0+MetaX and Transformers
4.57.6. The snapshot's `download-metadata.json` identifies that revision. Official
model terms must be accepted by the downloading account. Download credentials
are not needed at inference time. Choose generation and review GPU allocation for each deployment.

The runtime uses eager attention and explicitly verifies/ties the shared output
head after loading. The Transformers 4.57.6 wrapper otherwise left that head
randomly initialized in the initial PoC. Any other missing/unexpected/mismatched
weights prevent readiness. Do not remove these checks when updating libraries.

## Validation and limits

CPU tests use controlled classifier scores to test block/outage behavior; they do
not generate harmful pictures. Run `pytest deploy/image-moderation`, Python queue
unittests, Go tests, and the frontend build. An opt-in real-model Go contract test
uses the above gateway environment variables plus `STUDIO_REVIEW_TEST_PNG` pointing
to an existing benign PNG: `go test ./controller -run
^TestStudioRealOutputClassifier$ -count=1`.

The 0.5 threshold is not calibrated for this product. Benign real-GPU smoke tests
verify the connection, weights and decision contract, not detection accuracy.
Evaluate false positives and missed violations on an approved, representative
dataset before claiming production safety coverage. This is an initial output
filter, not a guarantee that every unsafe image will be detected.

Official model/policies: https://huggingface.co/google/shieldgemma-2-4b-it
