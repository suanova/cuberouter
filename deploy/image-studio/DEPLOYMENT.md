# Enable the Qwen Image 2.1 queue

Extend the existing CubeRouter Compose deployment with one CPU-only sidecar.
Use a gateway and sidecar built from the same change. The helper preserves
existing databases, volumes and services; it does not deploy, create users or
channels, or generate administrator API keys.

## Register existing workers

In the TARGET installation, create one OpenAI channel per physical GPU using the
existing authorized worker endpoint and provider key:

| Setting | Value |
| --- | --- |
| Model / test model | `qwen-image-2.1` |
| Base URL | Worker gateway URL, without `/v1` or trailing slash |
| Group | `image-studio` |
| Priority / weight | Same priority across workers / weight 1 |
| Parameter override | `{"response_format":"b64_json"}` |

Create the pricing group `image-studio`, multiplier 1, user-selectable OFF. Assign
only approved accounts. Group membership, not administrator role, allows queue
access. Ordinary token features remain available; other models still require
appropriate group/channel access. Do not add GPU channels to public groups.

Add model metadata tags `text-to-image` and `image-to-image` for both selectors.
Set the model's per-request price (an explicit zero is suitable for a free trial).
Record the new channel IDs: IDs from another installation are not interchangeable.

From the gateway host, verify authenticated `GET /health` returns ready,
`gpu_count: 1` and the correct physical GPU. Workers must accept JSON
`/v1/images/generations`, multipart `/v1/images/edits`, return PNG `b64_json`, and
support `extra_fields` steps/CFG/seed. The tested eight-worker backend implements
this protocol. Generic signed-URL edit requests are not the worker contract.

## Configure the existing Compose project

Python 3.12 and Docker Compose v2 are required. Replace IDs/project name with the
actual target values; keep private output outside the checkout.

```bash
python3 deploy/image-studio/configure_deployment.py \
  --compose ./docker-compose.yml \
  --project-name cuberouter \
  --gateway-service cuberouter \
  --channel-ids 101,102,103,104,105,106,107,108 \
  --output /opt/cuberouter-private/image-studio
```

Pass all original `--compose` and `--env-file` arguments in order. Use the EXISTING
project name. By default, the sidecar builds from checked-out source. CI publishes
`harbor.isuanova.com/suanova/cuberouter-image-studio:<commit-sha>` on main and matching
release tags. Pass `--dispatcher-image` to use a matching immutable image.

The helper validates the combined Compose config and prints the exact `up`
command. Review the overlay, then run that command. Use the full command for later
updates. Do not publish rendered config: it contains two service secrets. The
sidecar has no public port, persistent image volume or GPU access.

Manual deployment settings:

| Component | Environment |
| --- | --- |
| Gateway | `IMAGE_STUDIO_DISPATCHER_URL=http://image-studio-dispatcher:18195` |
| Both | `IMAGE_STUDIO_DISPATCHER_TOKEN` (shared secret, 32+ characters) |
| Both | `IMAGE_STUDIO_RELAY_TOKEN` (different shared secret, 32+ characters) |
| Gateway | `IMAGE_STUDIO_CHANNEL_MODEL=qwen-image-2.1` |
| Gateway | `IMAGE_STUDIO_CHANNEL_IDS=<actual target IDs>` |
| Gateway | `IMAGE_STUDIO_ALLOWED_GROUPS=image-studio` |
| Gateway | `IMAGE_STUDIO_AUDIT_ENABLED=true` |
| Sidecar | `IMAGE_STUDIO_RELAY_URL=http://cuberouter:3000` |
| Sidecar | `IMAGE_STUDIO_RELAY_CONCURRENCY=8` (or actual allocated GPU count) |

Restrict `/internal/image-studio/execute` at the public proxy to the private service
network; it also requires its private bearer secret. Allow 16 MiB bodies for
`/api/image-studio/`. Avoid request-body logging or disk buffering. Inference
calls need valid HTTPS and sufficient timeouts; do not disable TLS verification.

## Acceptance and rollback

1. Sign in as an approved account; open `/media-studio` and select Qwen.
2. Load an official example. It must remain editable without starting a job.
3. Submit, refresh mid-job, and verify one result/charge. Continue editing and
   verify previous dimensions/parameters are retained.
4. Add colored strokes; inspect submitted image/prompt under the result. Try
   multiple references within the ten-image/16 MiB limit.
5. Submit from separate accounts: jobs above GPU capacity wait; the same account
   cannot have two active jobs. Verify non-members and cross-account reads fail.
6. Verify standard usage records and root-only prompt/hash audit agree on
   account/job/channel.

Use ONE gateway replica for this pool. If another installation uses the same
workers, split allocation or coordinate tests; the queues are not globally shared.

For rollback, drain jobs/download results, disable the new channels, remove this
overlay from the original Compose command and recreate only the gateway. Stop the
sidecar. Keep database volumes and the additive audit table for its retention.
