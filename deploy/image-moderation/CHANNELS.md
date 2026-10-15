# Manage ShieldGemma through Channels

Deploy this CubeRouter backend **and its matching web build once**, to every Go
replica that serves Studio sessions or private dispatcher callbacks. Startup
migrates two small SQL tables (`image_moderation_policies` and
`image_moderation_leases`). All these replicas must use the same database and
synchronized clocks. No dispatcher image change or additional CPU pool service
is needed for this change. GPU services continue to run the existing authenticated
`image-moderation-v1` protocol.

Do not add the new channel type during a mixed old/new rollout. Old backends
cannot honor it. Finish the deployment, then configure channels.

## Administrator setup

In **Channel Management → Add channel**, select
**ShieldGemma 2 (Image moderation)**:

1. Give each running Gemma instance its own descriptive channel name.
2. Enter its complete HTTPS URL ending in `/v1/moderate`.
3. Enter that instance's dedicated moderation service key in the normal protected
   Key field. Do not use an HF token or a CubeRouter administrator API key.
4. For a private CA, paste the public CA certificate in PEM format. Publicly
   trusted certificates need no CA field. Certificate and hostname verification
   remain enabled; no insecure TLS switch is provided.
5. Save and use the channel's **Test** action. It classifies a small green PNG
   created in RAM, checks its hash, revision and all three policy scores, and
   never calls Qwen or stores the fixture. Routine “test all” skips moderators
   so LLM response-time thresholds cannot disable or enable the safety gate.

Saving the **first** moderation channel switches Studio to channel management
immediately and persistently, even if the old moderation environment flag is
false. Configure a ready, reachable service on the first save. Further channel
adds, edits, key/CA updates, disables and deletes require **no restart or MR**.
Waiting reviews reload channel settings; a request already sent may finish on
the previous channel configuration.

The channel is internal: no public API model ability or token group entitlement
is created. It is independent of the `image-studio` user group and Qwen channels.
General token service and Studio access checks are unchanged.

Before the first moderation channel is added, the existing deployment's
`IMAGE_STUDIO_MODERATION_ENABLED`, `..._URL` and `..._TOKEN` behavior is preserved.
Afterwards, those legacy settings are ignored. Disabling/deleting every moderator
**fails closed**, with `output_check_unavailable`; it does not restore the old
endpoint or bypass review. The SQL policy marker survives channel deletion.

## Distribution and waiting

```text
Image Studio user → existing generation queue → enabled Qwen channel
  → generated PNG held in RAM
  → shared SQL lease selects an available enabled Gemma channel
  → validated allow/block + hash + scores → 30-day audit
  → allowed output delivered; blocked/unavailable output withheld
```

Register one endpoint per physical Gemma replica. For the 30 + 2 plan, register
30 Qwen channels and two Gemma channels on separate GPU nodes. Do not also send
traffic to those same Gemma replicas through the legacy pool/direct clients.
Duplicate rows for the exact same canonical URL share one lease, but DNS aliases
for a physical replica cannot be automatically identified.

Each Gemma endpoint admits one CubeRouter inference at a time across Go
replicas, using an atomic SQL lease. Equal-priority endpoints rotate; configured
priority is respected, with other endpoints used when a higher-priority one is
busy. Moderation does not use LLM billing weights or public token routing.

Up to 64 reviews / 256 MiB of PNG data wait per Go process, with a **45-second
total deadline** including calls. Each call has a 12-second timeout. These are
bounded waits, not an unlimited backlog or a guarantee of strict FIFO among Go
replicas. A 30-output burst fits the count limit; actual throughput still depends
on image sizes, classifier latency and the network. At limits or deadline,
fail closed rather than release unchecked output.

Availability failures may retry classification on another configured endpoint,
once per channel, within the same deadline. A valid block or malformed/mismatched
verdict is terminal. Qwen generation is never replayed by this mechanism.
A broken connection leaves a 60-second lease to avoid immediately overlapping
an uncertain inference. Lease expiry permits automatic recovery after a Go
process dies. Known HTTP failures receive a short cooldown. Images stay in RAM;
SQL stores only endpoint hashes, ownership and expiry metadata.

## Audit and acceptance

The existing administrator-only `/api/image-studio-audit` returns
`moderation_state` and `moderation_result`. The latter now includes
`source: "channels"`, `channel_id`, `attempted_channel_ids`, model, revision,
threshold and the three validated scores. Generation `channel_id` remains the
Qwen channel; the nested moderation channel is distinct. Retention remains
30 days, with no long-term image storage. Legacy moderation reports use
`source: "legacy_environment"` so they cannot be mistaken for a Channel check.

Before enabling all production Qwen channels, test a benign create and edit,
then a burst at production image sizes. Verify the expected moderation
channel IDs, SHA-256 match and three scores in the audit. Verify waiting while
Gemma is busy and availability failover with one moderator disabled. In an
isolated test, verify that disabling all moderators withholds output. Check
Studio group restrictions and ordinary token requests as deployment regressions.

Local automated tests cover 30 outputs / two TLS classifiers, shared atomic
leases, live add/disable/delete, duplicate endpoint exclusion, failover, CA
verification, no fallback after deleting the last channel, terminal blocks/hash
mismatches, cancellation, admission bounds and preservation of existing audit
release checks. These tests do not by themselves certify the production route.

### Deployment acceptance

The source integration had isolated real-GPU tests, but this branch's automated
checks use controlled classifier fixtures. Validate real GPU connectivity,
representative image sizes and concurrent load in the destination environment
before production rollout. Do not treat a successful mocked test as a capacity
or classifier-accuracy certification.
