# Qwen workflow adapter

Native CubeRouter Media Studio with account-owned uploads/history, generation, editing, masks, text tools and before/after comparisons. [Architecture and integration runbook](../../docs/media-studio-integration.md).

## Configure

Python 3.10+, standard library only. Set process environment variables and run `python bridge.py`:

| Variable | Purpose |
| --- | --- |
| `QWEN_BRIDGE_API_KEY` | Private random bearer key, minimum 24 characters |
| `QWEN_BRIDGE_PORT` | Loopback port, default `18163` |
| `QWEN_WORKFLOW_ORIGIN` | Private workflow origin, default `http://127.0.0.1:18162` |
| `QWEN_STUDIO_DATA` | Absolute persistent directory; enables account workflows |
| `QWEN_STUDIO_RETENTION_DAYS` | 1–365 days, default 30 |

On CubeRouter's Go process, set `MEDIA_STUDIO_BRIDGE_URL=http://127.0.0.1:18163` and `MEDIA_STUDIO_BRIDGE_KEY` to the same secret. Create OpenAI channels for `qwen-image-2512` and `qwen-image-edit-2511`, with that base URL/key and `image-generation` endpoint metadata. Set model image prices through existing CubeRouter pricing. Do not enable model mapping, body overrides or pass-through transformations: submitted requests must exactly match their preparation. Secrets stay outside Git.

The adapter binds to loopback. Never expose its private `/studio/{owner}/...` routes to browsers. The current SSH tunnel forwards local `18162` to .162 **loopback 8002**, backed by private workflow/tools **8191**, sharing existing GPU models **8188/8189**. Do not use the old public demo as the workflow origin; it has shared public history. Without `MEDIA_STUDIO_BRIDGE_URL`, the frontend retains PR #96's basic Media Studio. No database migration is needed.

## Contract

Use the CubeRouter dashboard session/access token, not a channel key:

1. `POST /api/v1/media-studio/jobs` prepares a create/edit/regional request and returns `{job, relay_body}`. Do not persist/log the short-lived relay capability in the UI.
2. `POST /pg/images/generations/studio` with exactly `relay_body`. The normal image relay handles channel selection, quota and usage logs. Never automatically retry.
3. Poll `GET /api/v1/media-studio/jobs/{id}`. Only `completed` releases result assets. `awaiting_settlement` is waiting for CubeRouter's relay confirmation.
4. Fetch `GET /api/v1/media-studio/assets/{id}/content` with authentication. The UI uses temporary blob URLs for image elements.

Other routes under `/api/v1/media-studio`: `GET config`, `GET jobs`, `DELETE jobs/{id}`, `POST uploads` (raw PNG/JPEG/WebP), `POST masks`, `POST ocr`, `POST preview`, `POST text`. Go supplies the authenticated owner; users cannot choose another owner or invoke private authorize/settle routes.

Limits: uploads 10 MB; references 1–3; outputs 1–4; create 1–100 steps, edit 1–60, regional 8–60. Regional uses one reference/result and CFG 1 or 4. Explicit zero seed/CFG remain valid for ordinary create/edit. Steps determine the priced quality tier: fast ≤20, standard ≤40, high >40.

Create sizes: `1328x1328`, `1664x928`, `928x1664`, `1472x1104`, `1104x1472`, `1584x1056`, `1056x1584`. Edit: `auto`, `1024x1024`, `1344x768`, `768x1344`. Auto follows model reference sizing; it does not promise original pixel dimensions. Regional/text preserve source dimensions.

## Durability and limits

- One GPU operation per adapter at a time, two CPU text tasks. Remote services may also be busy.
- CubeRouter owns reserve/consume/refund and usage logs. The adapter never debits wallets. Only a successful relay callback releases GPU results and records the request ID.
- Timeout does not cancel GPU work. No automatic replay; late completion cannot override failed settlement. Restart marks unfinished jobs interrupted. If release confirmation fails after charging, reconcile the saved studio job and usage-log ID before refunding/retrying; no distributed reconciliation daemon exists yet.
- Atomic JSON and PNG copies under `{data}/{owner}/{jobs,assets,masks}`. Expired reads fail immediately; physical cleanup runs at most hourly on API traffic. Deleting a saved job removes its output/candidate copies, with active-job protection. Unused uploads expire by TTL.
- Bounds: 1,000 jobs and approximately 2 GiB of media per account; history lists the last 100 unexpired jobs. Local TTL/deletion does **not** clean raw GPU/remote files: see production requirements in the runbook.

Compatibility: the original `/v1/images/generations` request without studio fields still serves basic T2I with `b64_json`. Prepared studio requests return authenticated CubeRouter asset URLs. `/v1/models` advertises both models when account storage is enabled. `/health` reports adapter readiness only.

## Verify

```sh
python -B -m unittest discover -s scripts/qwen_image_bridge -v
go test ./controller -run '^TestStudio' -count=1
cd web
bun run test src/features/media-studio
bun run typecheck
bun run build
```

Tests cover ownership, one-use submission, altered/replayed bodies, result release, late completion after failed settlement, dimensions/count, interruption, expiration/deletion, CPU-slot handling, bounds, templates and independent image cancellation. Real GPU/API/browser validation is still required after deploying a new worker or changing prices.
