# Image quality

## Quality priority

Image quality takes priority over throughput and memory savings. Preserve full
VAE context, original PNG reference bytes and the selected output dimensions.
Keep the model weights and dtype unchanged. An out-of-memory failure must not
silently trigger a retry with tiles, smaller references, fewer steps or a smaller
output. Reference conditioning limits are explicit service limits and must be
disclosed; full VAE decoding does not restore detail already removed by resizing.

Use fixed inputs, prompts, seeds and dimensions when comparing a proposed
optimization. Inspect actual output texture, small text and seams, alongside
peak memory and successful completion. Higher step counts or larger images alone
are not evidence of better quality. Retain the validated 40-step, CFG-1 defaults
unless a controlled visual comparison justifies changing them.

The worker waits for the final denoising step to finish on the GPU, then releases
the request's prefix KV caches before VAE decoding. These caches are still
available to every denoising step. A temporary transformer hook tracks both CFG
branches; it is removed and its cached references are released on success and
failure. This reduces overlapping memory use without lowering precision,
changing latents or recomputing the image at a smaller size.

## Problem and change

Image Studio previously reused a generated artwork's seed when continuing or
annotating it. Qwen's maintainers describe a noise replay failure when generation
and editing reuse the same noise at the same target size: over-sharpening and
grain can appear instead of a clean edit. See the confirmed correction in
[Diffusers issue 14824](https://github.com/huggingface/diffusers/issues/14824).

New edits of saved artwork now select a different seed before the dialog opens.
The dialog displays that seed, and the saved request, recovery and audit paths
retain it. A single uploaded reference gets a fresh seed and defaults to its
original aspect and supported output dimensions. Explicit advanced seed choices
are still sent unchanged; do not silently perturb seeds in the GPU worker.

Two separate sources of avoidable loss are addressed in the GPU worker:

- A single reference no longer always shrinks to a 1024-level conditioning image;
  it follows the source area up to 2,097,152 pixels, with pipeline alignment.
- Creation and all edit reference counts use full VAE context instead of the
  256-pixel tiles enabled by the old worker. Multi-reference conditioning now
  uses the higher, validated quality limits below. The worker keeps tiling off
  and clears VAE feature caches in `finally`, including after inference failure.

PNG output and original reference bytes are retained. No sharpening filter,
upscaler, prompt rewriting, weight replacement or moderation bypass is added.
The model still regenerates the whole image: detail and identity may change,
especially over repeated edits. The UI recommends describing only the intended
changes and retaining the original for comparison.

## Validation on 2026-10-12

The checked deployment uses Qwen revision
`b3179ad355be050328e483a9dfdd9e60cd62adfa`, Diffusers `0.41.0.dev0`, and a
single MetaX C500 per worker. Controlled probes used the same installed pipeline
as production (class SHA-256
`8a35a96a1f82fd4126243ae0a355b9c53898c60cd4e793077376874500a6cd75`).

A CPU-only VAE round trip on a 512-square portrait crop, with no diffusion,
measured 35.79 dB reconstruction PSNR with the default tiles and 39.41 dB with
full context. Mean absolute RGB error fell from 2.33 to 1.43. This isolates an
extra codec loss; it does not measure instruction following or promise that
model-generated edits preserve every original pixel.

An isolated staging GPU probe completed all four cases at 40 steps, CFG 1:

| Case | Reference conditioning | Output | Time | Peak allocated GPU memory |
| --- | --- | --- | --- | --- |
| Existing tiled worker | 1024 | 1312 × 1312 | 46.31 s | 36.41 GB |
| Full VAE | 1024 | 1312 × 1312 | 41.42 s | 45.85 GB |
| Full VAE, native reference | 1280 | 1312 × 1312 | 44.63 s | 47.06 GB |
| Full VAE, larger output | 1280 | 1664 × 1248 | 53.49 s | 49.34 GB |

The larger output reserved 54.76 GB on a 68.28 GB C500 and did not exhaust
memory. Disabling tiling alone did not remove the full-frame over-sharpening in
the repeated-seed case. It must not be presented as the primary noise replay fix.
Test images and account credentials are excluded from this repository.

A second five-case C500 probe confirmed the seed fix independently. With the
original reference, prompt, dimensions and tiled worker unchanged, changing only
seed 42 to 43 removed visible over-sharpening. After a laptop-to-black edit, two
identical black-to-blue edits of its output reproduced grain at seed 43 and gave
a visibly cleaner result at seed 44. Both outputs were 1280-square and took about
41 seconds. Gradient energy in the upper 900 rows (outside the laptop edit) was
2.24 times the input with the reused seed and 1.13 times with the new seed; this
is an over-sharpening diagnostic, not a universal quality score. All five cases
completed without OOM (maximum allocation 46.52 GB). All eight original staging
workers were restored and reported ready after each probe.

Local regressions: 51 Python tests and 19 Studio JavaScript tests passed; the
production frontend build passed. Browser checks used the actual Studio
components with a clearly labeled local simulated API, verifying new edit seeds,
unchanged submitted settings, original upload dimensions, manual ratio selection
and translated guidance. These checks are separate from model/GPU validation.

## Rollout and rollback

### Text-to-image seam validation, 13 October 2026

A production reproduction used the coffee-product prompt from account
`hkbn-image01` at 12:04:12 HKT: 1312 × 1312, 40 steps, CFG 1, seed 42. It completed
through the normal moderated API and returned an original PNG with the same
vertical purple/green artifacts as the reported image. This was a create job,
independent of the repeated-edit seed failure.

An isolated staging C500 generated one set of latents at those settings. Decoding
those identical latents with 256-pixel tiles reproduced the lines; disabling VAE
tiling removed them. The visible columns recur about every 192 pixels, matching
the installed tile stride. Full decoding took 2.02 seconds versus 6.05 seconds
for tiled decoding, with peak allocation 43.59 GB versus 32.96 GB. The pipeline,
prompt, seed and latent values were fixed, so this isolates the decoder path.
All eight original staging workers were restored and ready after the probe.

`generation_vae_tiling=false` in worker health identifies this additional fix.
MR !15's single-reference fix alone does not change creation. The combined
`worker21-edit-quality.patch` now includes both changes and was checked against
the inspected current production source; apply it once to that old source, or
review the delta if the earlier patch is already installed. Do not apply it twice.
The quality-priority follow-up also disables tiling for multi-reference edits.
Worker health exposes `multi_reference_vae_tiling=false`. Conditioning budgets
remain separate from the decoder. The installed pipeline accepts a single area
setting for all references, so it follows the largest source image's area up to
the count-specific limit. Multi-image edits retain the previous 1024-level
(two/three) or 512-level (four/ten) floor for small assets. Smaller images in a
mixed set may be upsampled. These are explicit conditioning limits, not a claim
that every multi-reference source stays at its original resolution.

### Higher quality reference limits, 13 October 2026

| Reference count | Previous conditioning | New maximum equivalent side |
| --- | --- | --- |
| 1 | Source area, up to 2 MP | Unchanged, up to 2 MP (1448) |
| 2 | 1024 | Up to 2 MP (1448) |
| 3 | 1024 | 1280 |
| 4 | 512 | 1024 |
| 5–10 | 512 | 896 |

The equivalent side is an area budget, not a square crop: the pipeline keeps
each input's aspect ratio and aligns dimensions. Output sizes remain unchanged.
The worker never retries a failed request at a lower quality. Health reports
`reference_resolution_policy=quality_first` and the full
`reference_resolution_caps` mapping; the legacy `multi_reference_resolution`
field now reports 896 for the five-to-ten-reference ceiling.

Bounded C500 probes used synthetic 1312-square inputs, 1664 × 1248 output,
40 steps and CFG 1. They validate memory feasibility, not a benchmark of semantic
accuracy or a guarantee for every image, prompt or manual CFG setting:

| Case | Final-step KV release | Result | Time | Peak allocated memory |
| --- | --- | --- | --- | --- |
| 2 references, 1448 conditioning | No | Completed | 83.74 s | 54.45 GB |
| 3 references, 1280 conditioning | No | OOM | 143.98 s | 54.82 GB |
| 3 references, 1280 conditioning | Yes | Completed | 147.06 s | 46.75 GB |
| 4 references, 1024 conditioning | No | Completed | 103.10 s | 54.54 GB |
| 10 references, 768 conditioning | No | OOM | 143.01 s | 53.89 GB |
| 10 references, 768 conditioning | Yes | Completed | 167.68 s | 49.38 GB |
| 10 references, 896 conditioning | No | OOM | 182.01 s | 57.95 GB |
| 10 references, 896 conditioning | Yes | Completed | 190.65 s | 55.01 GB |

The 896 case released 16.50 GB of completed prefix KV tensors and reserved
61.83 GB on a 68.28 GB C500. Ten references at 1024 still failed an earlier
one-step feasibility probe; that setting is not enabled. Higher input context
can substantially increase latency; the 10-reference 512/full-VAE baseline took
80.92 seconds. All eight original staging workers were restored and ready after
each diagnostic group. Production was not changed.

Cache cleanup does not change weights, dtype, prompt, seed, steps or latent
arithmetic. Its numerical equivalence is checked separately by decoding the same
generated latents before and after releasing their prefix caches, avoiding the
nondeterminism of independently rerunning diffusion. Independent control runs
must not be treated as byte-identical merely because they share a seed.

### Deployment steps

1. Build/deploy the gateway/UI from this MR to Nutsflush using the existing
   deployment process. This is required for the new edit seed and upload defaults.
2. Separately copy `edit_quality.py` beside each mounted `worker21.py`, then
   review and apply `worker21-edit-quality.patch` to the existing worker source.
   Run `git apply --check /path/to/worker21-edit-quality.patch` from that code
   directory first; stop and compare the source if the check fails. The patch was
   checked against the inspected production worker and preserves its bridge
   bind address and physical-GPU lock/admission settings: those runtime details
   can differ from this repository's isolated worker example. Do not replace
   them blindly or change worker URLs, tokens, channel counts, weights or drivers.
3. Drain the chosen test worker and verify it is idle before restarting only that
   worker. Require health `state=ready`, the original physical GPU/revision,
   `single_reference_max_pixels=2097152` and
   `single_reference_vae_tiling=false`, `generation_vae_tiling=false` and
   `multi_reference_vae_tiling=false` and
   `conditioning_cache_release_before_decode=true`. An import error usually means the new
   helper was not copied alongside the worker.
4. On staging, generate, continue editing twice, annotate once, and edit a
   single uploaded photo. Check distinct parent/child seeds, full PNG downloads,
   original dimensions, moderation/audit completion and actual visual detail.
   Compare identical input/prompt/seed settings to evaluate VAE changes alone.
   Also run a text-to-image job and a ten-reference job to confirm unchanged
   paths. Monitor GPU memory and failures before proceeding to other workers.
5. After staging acceptance, the deployment owner can roll the same worker and UI
   changes through production with draining. Keep the agreed 14 Qwen + 2
   ShieldGemma allocation. Do not restart a gateway/dispatcher holding RAM jobs.

Rollback: drain the affected worker, restore its recorded previous source files,
then restart that worker and verify health. Restore the previous gateway/UI
image separately if necessary. A UI rollback resumes the old seed inheritance;
a worker rollback resumes the old reference resize/tiling behavior. Never delete
jobs, library data, databases, model weights or channel records as rollback.

## CubeRouter port on 2026-10-13

The seed and upload defaults are included in the native TypeScript Media Studio.
The GPU patch is an optional, separate worker rollout; merging this port does not
apply it or certify current cuberouter.ai worker deployment. Preserve existing
worker credentials, bind addresses, GPU locks and allocations when applying it.
The combined patch includes the merged Nutsflush MR !16 full-context decoder,
higher reference limits and final-step conditioning-cache cleanup. Its helper
tests are included here. The measurements above are Nutsflush evidence from
2026-10-12 and 2026-10-13, not measurements of cuberouter.ai.
