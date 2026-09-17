# Qwen workflow adapter

Native CubeRouter Media Studio with account-owned uploads/history, generation, editing, masks, text tools and before/after comparisons. [Architecture and integration runbook](../../docs/media-studio-integration.md).

## Configure

### Reviewing PR #99 after a normal Docker build

The new UI is always visible at `/media-studio`, including the template gallery and **Image to image** tab. Without an image service it shows a connection notice, disables uploads/generation/history, and offers an explicit **Use basic image generation** action. It no longer silently replaces the new UI with the old page.

The initial PR commit `6c61d31b` did silently fall back when `MEDIA_STUDIO_BRIDGE_URL` was absent. If you still see only the old page, rebuild the updated PR head (not the base `media-studio` branch), recreate the running container, and refresh the browser. There is no frontend build flag to enable image-to-image.

Template assets live under `/studio-templates/`, separate from the `/media-studio` page route. The original asset directory collided with the embedded Go static server and could cause a redirect loop on direct navigation. After rebuilding, `curl -I http://127.0.0.1:3001/media-studio` must return `200`, not a directory `301`. If a browser cached the original redirect, clear that site's cache or test in a fresh browser session.

### Reproduce the connected review stack

Use Docker Engine/Desktop with Compose v2+ and Python 3.10+ for the one-time configuration helper. Run commands from the repository root. This stack uses new named volumes and port 3001; it does not reuse an existing CubeRouter database or automatically copy its channels/users.

1. Generate a local configuration (secrets are written to an ignored file, never printed; existing files are not overwritten):

   ```sh
   python scripts/qwen_image_bridge/init_deployment.py
   ```

2. For the existing .162 worker, create `.media-studio/ssh/` with `config` (copy `scripts/qwen_image_bridge/ssh_config.example`), your authorized deployment key as `id_ed25519`, and a trusted `known_hosts` file. These files are ignored by Git and excluded from the root Docker build. The key must work noninteractively. The tunnel copies it into private temporary storage to handle Windows mount permissions. Do not disable host-key verification. For a jump host, add `ProxyJump jump` under `Host workflow`, define `Host jump`, and supply the corresponding trusted host keys. Existing SSH credentials are not included in this repository.

3. Build and start all three components:

   ```sh
   docker compose --env-file .env.media-studio -f docker-compose.media-studio.yml --profile ssh up -d --build
   docker compose --env-file .env.media-studio -f docker-compose.media-studio.yml exec studio-adapter python check_deployment.py --web-origin http://127.0.0.1:3000
   ```

   The first command builds the actual CubeRouter Dockerfile plus the packaged adapter and SSH client. The second checks adapter authentication and all three upstream services without generating an image. A healthy adapter container alone does not prove the worker is reachable. If your deployment already has a private reachable workflow origin, set `QWEN_WORKFLOW_ORIGIN` in `.env.media-studio` and omit `--profile ssh`.

4. Open `http://127.0.0.1:3001`, complete CubeRouter's normal first-run setup/login, and configure **two OpenAI image channels** in the admin UI:

   | Setting | Create channel | Edit channel |
   | --- | --- | --- |
   | Model | `qwen-image-2512` | `qwen-image-edit-2511` |
   | Base URL (no `/v1` suffix) | `http://127.0.0.1:18163` | `http://127.0.0.1:18163` |
   | Channel key | `MEDIA_STUDIO_BRIDGE_KEY` from the local env file | Same key |
   | Endpoint type | `image-generation` | `image-generation` |

   Enable the channels for the logged-in user's group; configure image pricing and sufficient account quota using existing CubeRouter settings. Do not enable model mapping or body overrides. The containers share a network namespace, so this loopback address works **inside the CubeRouter container**, not on the host. The adapter/tunnel have no published ports. Only the web port is published, bound to localhost by default. For deliberate network access set `STUDIO_BIND_ADDRESS` to the desired interface and keep normal CubeRouter authentication.

5. Open `/media-studio`: confirm service status, generate one image, then use **Continue editing** or upload a reference in **Image to image**. Confirm saved history and the associated request in Usage Logs. A worker health check does not verify channel pricing, quota or generation quality.

The worker is an **external prerequisite**: .162 already runs the private workflow gateway on loopback 8002, tools on 8191 and GPU model services on 8188/8189. This Compose file reuses those services; it does not download weights, install the full OCR/GPU worker or expose the old public demo. It does not depend on Patrick's Windows process, local database or local SSH tunnel. An empty machine without the worker can review the UI but cannot generate images.

To stop the review stack, use the same Compose command with `--profile ssh down` (without `-v`, to retain accounts and media). If you recreate only the CubeRouter container, recreate the adapter/tunnel too because they share its network namespace. Do not share the review volumes between independent deployments.

### Existing CubeRouter deployment / run without Docker

Python 3.10+, standard library only. Set process environment variables and run `python bridge.py`:

| Variable | Purpose |
| --- | --- |
| `QWEN_BRIDGE_API_KEY` | Private random bearer key, minimum 24 characters |
| `QWEN_BRIDGE_PORT` | Loopback port, default `18163` |
| `QWEN_WORKFLOW_ORIGIN` | Private workflow origin, default `http://127.0.0.1:18162` |
| `QWEN_STUDIO_DATA` | Absolute persistent directory; enables account workflows |
| `QWEN_STUDIO_RETENTION_DAYS` | 1–365 days, default 30 |

On CubeRouter's Go process, set `MEDIA_STUDIO_BRIDGE_URL=http://127.0.0.1:18163` and `MEDIA_STUDIO_BRIDGE_KEY` to the same secret. Create OpenAI channels for `qwen-image-2512` and `qwen-image-edit-2511`, with that base URL/key and `image-generation` endpoint metadata. Set model image prices through existing CubeRouter pricing. Do not enable model mapping, body overrides or pass-through transformations: submitted requests must exactly match their preparation. Secrets stay outside Git.

The adapter binds to loopback. Never expose its private `/studio/{owner}/...` routes to browsers. The current SSH tunnel forwards local `18162` to .162 **loopback 8002**, backed by private workflow/tools **8191**, sharing existing GPU models **8188/8189**. Do not use the old public demo as the workflow origin; it has shared public history. Without `MEDIA_STUDIO_BRIDGE_URL`, the new frontend remains visible with a connection notice and an explicit basic-generation option. No database migration is needed.

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
