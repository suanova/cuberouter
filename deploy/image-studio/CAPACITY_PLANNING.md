# Cyberport: 30 Qwen channels + 2 ShieldGemma GPUs

The agreed initial allocation is 30 Qwen Image 2.1 GPUs and two ShieldGemma 2 GPUs
within four 8-GPU Cyberport hosts (32 GPUs total). This is a planning baseline,
not a production certification or authorization to stop the MiniMax services.
Verify the Cyberport GPU model, memory and runtime compatibility before cutover.

| Logical host | Qwen workers / image channels | ShieldGemma replicas | Total GPUs |
| --- | ---: | ---: | ---: |
| A | 7 | 1 | 8 |
| B | 7 | 1 | 8 |
| C | 8 | 0 | 8 |
| D | 8 | 0 | 8 |
| Total | 30 | 2 | 32 |

A/B/C/D are placement labels, not actual addresses. Assign physical GPU IDs at
deployment. Both moderation replicas serve requests; neither is a permanently
idle spare. Place them on different hosts so a single host failure does not take
down all moderation. Reduced moderation capacity may require throttling new
generation. This does not guarantee unchanged throughput during a failure.

## Provider routing without node addresses in application code

Run one independently pinned Qwen worker and compatible bridge per GPU. Register
30 OpenAI-compatible image channels across the hosts above, with the same model name
and the appropriate permitted user group. Use the actual reachable worker URL/key
in Channel Management; neither node IPs nor a fixed list of eight IDs belongs in
the gateway/dispatcher script. Use `IMAGE_STUDIO_CHANNEL_IDS=auto` on the gateway
and `IMAGE_STUDIO_RELAY_CONCURRENCY=auto` on the dispatcher. Current auto mode is
bounded at 64 channels. Explicit legacy ID lists now also accept up to 64 IDs.

All four hosts participate in the same pool. A worker is reserved before inference
and checked for health; unavailable workers are skipped for new jobs. Loss of a
whole host removes that host's seven or eight generation workers, not the other
hosts' GPU capacity. Losing A or B also removes one moderation replica.
Interrupted jobs are not automatically generated again. The gateway/dispatcher
themselves still require one active instance each; shared durable queue/leases
and coordinated failover are separate requirements for application-layer HA.

## ShieldGemma count depends on images per second

The recorded single-image C500 test checked all three policies in approximately
0.985/0.924 seconds after warmup (first request 8.076 seconds), with approximately
10.632 GiB peak allocated memory. This is a small benign-image smoke test, not
sustained throughput or p95 latency, and not a Cyberport hardware benchmark.
ShieldGemma 2 is Google's 4B image classifier:
https://ai.google.dev/gemma/docs/shieldgemma

For planning, let G be active generation GPUs, Tg mean generation seconds/image,
Ts measured moderation service seconds/image, and U the target utilization.
Then moderation replicas = ceil((G / Tg) * Ts / U). Add spare capacity for
failure tolerance. This approximates steady state, not a tail-latency guarantee.
Measure burst queue delay and sustained service time before using this estimate.

The initial allocation is two moderation replicas. It is not derived by dividing
30 channels by eight: eight is the current API admission bound, not GPU batch
size or a model capacity limit. Each request contains one image; the current
runtime checks one image at a time, including its three policies.

Assuming Ts stays at one second, two replicas have an ideal ceiling of two images
per second. At 70% planned utilization their combined target is 1.4 images/second.
With G=30, this corresponds to a mean generation time of roughly 21.4 seconds or
longer. These are illustrative calculations, not measured sustained throughput.

| Mean generation time (assumed, 30 GPUs) | Image arrivals per second | Implication for two replicas at Ts=1s |
| --- | ---: | --- |
| 10 seconds | 3.0 | Above the ideal review ceiling; backlog/pressure must limit admission |
| 20 seconds | 1.5 | Below the ideal ceiling but above the 70% planning target |
| 30 seconds | 1.0 | Within the planning target, subject to burst and p95 tests |
| 60 seconds | 0.5 | Substantial steady-state headroom under these assumptions |

A one-off burst of 30 images would take about 15 seconds to drain with two
replicas, or 30 seconds with one, ignoring transfer/other work and assuming a
consistent one-second check. This requires the moderation queue described below;
the existing eight-request single-replica API cannot accept that burst as-is.
Test the two-replica baseline first and expand only if measured throughput or
waiting-time targets require it. No recommendation to reserve four GPUs remains.

Dedicated moderation GPUs avoid competing with generation. Different-host
placement preserves moderation capacity after a host loss. If fewer replicas
are healthy, throttle new work/queue it; never deliver unchecked images.

## Moderation queue target and work required before rollout

- The single-runtime API accepts at most eight in-flight requests and serializes
  inference. For the two-replica deployment, point the gateway's review endpoint
  to `../image-moderation/pool.py`, which admits up to 64 checks / 256 MiB and sends
  one request at a time to each replica. Keep replica addresses in its private
  deployment configuration, not application code. Monitor classifier readiness
  separately from pool liveness and validate actual burst latency before cutover.
- Use a bounded FIFO for pending checks, dispatching to either healthy replica
  as it becomes free. Bound both image count and retained bytes, remove expired
  or abandoned queued checks, and apply admission pressure when full. Never drop
  moderation or regenerate an image merely because review is slow.
- Separate the physical generation lease from review waiting: release the GPU
  lease only after upstream inference has definitively completed, then keep the
  result private until moderation and audit pass. The current implementation
  still holds its generation lease through review. Keep the job/recovery identity
  active until final settlement; do not simply release the entire logical job.
- Size moderation queue bytes/count and end-to-end deadlines together. Current
  runtime lock wait is 30 seconds and gateway moderation HTTP timeout is 45 seconds.
  Validate warmup, 30 simultaneous completions, one-replica loss, restoration and
  prolonged overload. A refusal or moderation failure must never regenerate the
  image or release an unchecked result.
- The generation queue still defaults to 10 waiting jobs and 256 MiB aggregate
  content, with one active job/account. Its HTTP server admits at most 24 requests
  at once; these are short API calls, distinct from asynchronous GPU executions.
  Multi-user submission/polling and ten-reference edits need separate load tests
  and bounded admission/memory tuning. Do not equate 64 logical slots with proven
  service capacity for 64 users submitting simultaneously.
- Test actual Cyberport image sizes, steps, create/edit mixes and reference counts.
  Measure generation rate, moderation sustained throughput/p95, queue delay, memory,
  audits and ordinary token service under load. A 32-slot local scheduler test uses
  stubs; it does not certify physical GPUs, moderation scale or production HA.
- Calibrate the moderation threshold on representative allowed/disallowed content.
  Current 0.5 is a PoC threshold. The current gate covers Studio outputs only;
  ordinary image token APIs, input images and historic images are outside it.

Update matching gateway/dispatcher images once for auto discovery. Routine changes
to compatible image channels then remain in Channel Management. Provisioning GPU
containers, changing model APIs or adding moderation replicas is infrastructure
work, not something registering a provider channel automatically performs.

## Deployment status

Auto-discovery and local 32-slot scheduler regression coverage are in this branch.
The CPU-only `../image-moderation/pool.py` implements the bounded two-replica
review queue described in its README. Its tests cover a 30-image burst, one
replica unavailable, both unavailable, FIFO, expiry and memory/count limits.
Physical deployment and real-GPU acceptance results must be recorded separately;
code presence does not demonstrate live production configuration. Generation
leases still include review waiting, and multiple active image gateways still
require shared coordination or deployment pinning to one active execution instance.
