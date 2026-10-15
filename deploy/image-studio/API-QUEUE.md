# API image queue and audit

Deploy the matching **CubeRouter backend, frontend and image dispatcher** from
this MR to the test environment first, then production. Updating the MR alone
does not change a running deployment. All public Go instances must be updated.
Drain jobs before restarting the RAM dispatcher; its pending jobs/results do not
survive a restart. Do not run mixed old/new Go replicas during this rollout.

No additional service, GPU or hardcoded channel list is introduced. Keep the
existing dynamic channel setup and the single private relay destination used by
dispatcher capacity/execute/reconcile. This change does not resolve or certify
the separate 30-worker large-image issue.

## Existing configuration required

Go: `IMAGE_STUDIO_CHANNEL_MODEL`, `IMAGE_STUDIO_CHANNEL_IDS=auto`,
`IMAGE_STUDIO_DISPATCHER_URL`, dispatcher/relay tokens,
`IMAGE_STUDIO_ALLOWED_GROUPS`, **`IMAGE_STUDIO_AUDIT_ENABLED=true`**.
Dispatcher: existing `IMAGE_STUDIO_RELAY_URL`, relay/dispatcher tokens and
`IMAGE_STUDIO_RELAY_CONCURRENCY=auto`.

The new dispatcher advertises `image-api-queue-v1`. New Go rejects an older or
direct-worker dispatcher instead of bypassing the queue. The new API callback is
`/internal/image-studio/execute-api`; keep all internal routes private.

## Client contract

Normal model API keys still use `Authorization: Bearer sk-...`, with no login
cookie or `New-Api-User` header. Qwen requests to `/v1/images/generations`,
`/v1/images/edits` and `/v1/edits` enter the same queue as browser Studio.
Other image models and LLM APIs keep their original relay path.

Generation JSON example:

```json
{
  "model": "qwen-image-2.1",
  "prompt": "A quiet harbour at sunrise",
  "n": 1,
  "size": "1312x1312",
  "response_format": "b64_json",
  "extra_fields": {"steps": 40, "cfg": 1, "seed": 42}
}
```

Edits use multipart `image` or `image[]` files (up to 10), with the same fields
and `extra_fields` as a JSON string. Request limit remains 16 MiB including
base64 conversion; use smaller references if necessary. Only inline PNG output,
`n=1` and standard quality are supported. Unsupported fields are rejected rather
than silently dropped. Model parameters keep the existing Studio limits.

- Send a unique `Idempotency-Key` (16–80 letters/digits/`_`/`-`) per intended
  generation. Reuse it only to recover that request, not for a new generation.
- Normally the response is HTTP 200 with `data[0].b64_json`.
- `Prefer: respond-async` returns HTTP 202 immediately with `id` and `status_url`.
  Without it, Go waits up to about 110 seconds before returning the same 202.
  Configure the public reverse proxy read timeout above 115 seconds for the
  synchronous path, or use the async option to avoid long HTTP connections.
- Poll `GET /v1/images/jobs/{id}` using **the same model API key**. HTTP 202 means
  pending; HTTP 200 contains the usual image response. Respect `Retry-After`.
  `X-Image-Job` / `Location` headers also identify accepted jobs.
- Existing bounds remain: one unfinished job per account across browser and API,
  10 waiting jobs globally, and the existing queue RAM/TTL limits. Explicit
  admission rejection (429/503) requires a **new key** for a new attempt after
  waiting. Do not automatically replay an uncertain or failed inference.
- Prompt/parameter changes with the same key produce 409. Idempotency records
  remain for 30 days; image result bytes remain in dispatcher RAM for 10 minutes.
  Retrying after results expire or the dispatcher restarts does not regenerate
  an image. A user must deliberately start a new request with a new key.

Token ownership, expiry, quota, IP/model restrictions and current account group
are rechecked at execution and before output delivery. Limited API tokens retain
normal billing; they are not converted into unlimited browser-session tokens.
Inference can still incur its normal charge if its output is subsequently
blocked by moderation.

## Moderation and audit administration

Root administrators can open `/image-studio-audit` from **Image audit**
in the admin sidebar. It lists browser/API activity, account, channel, job state,
prompts, parameters, image hashes, Gemma verdicts/scores and expiry. Filter by
user/job/hash/source/verdict and use Older records to paginate.

The same page configures moderation by **account group**. Token-selected routing
groups cannot weaken the account group's review requirement.

- **External** (default for groups without a setting): the API integrator handles
  moderation. The audit explicitly says `external`, never `passed`. This
  preserves the current HKBN API integration agreement.
- **Required**: use existing Gemma channels (or the existing legacy configured
  moderator). A blocked result returns 422; unavailable/invalid review returns
  503. Neither response contains image bytes. No client field can disable review.

Browser Studio retains its existing moderation policy. Changes apply at execution
and retrieval, including queued jobs. An old externally reviewed result cannot
be downloaded through the API after its account group switches to required.
Changing this policy or normal channels after deployment requires no new MR.

The existing root-only `GET /api/image-studio-audit` array response remains
compatible. New optional filters: `source`, `moderation_state`, `before_id`.
`next_before_id` is the cursor for the next page (up to 50 rows).
Root-only `GET/PUT /api/image-api-policy` lists or saves policy; PUT body:
`{"group":"image-studio","moderation":"required"}`.

No image pixels are saved in the audit DB. Prompts, parameters, hashes and scores
expire after 30 days through the existing cleanup. A hash cannot restore an image.

## Local verification

```sh
go test ./controller ./model ./middleware ./router ./relay/common
python -B -m unittest discover -s deploy/image-studio
cd web && bun run build && cd ..
go build -o cuberouter .
python deploy/image-studio/smoke_api.py /absolute/path/to/cuberouter
```

The smoke test starts the real Go gateway and Python queue with a disposable
SQLite DB and loopback simulated GPU/moderator. It checks parameter preservation,
token billing, required/external review, 4-account queuing, access control,
ordinary LLM calls, async retrieval, hashes and no replay after dispatcher restart.
It does not certify GPU model performance or production proxy/network settings.
