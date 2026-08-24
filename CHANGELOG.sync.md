# Upstream Sync Changelog

## 2026-08-24 — 10 commits from new-api#main

| SHA | Intent | Type | Risk |
|-----|--------|------|------|
| `60a1acb703a6` | Update import paths in the newapi channel adaptor to use dto and types packages relocated under the relaykit module | refactor | low |
| `b8bb3f40ac9d` | Move shared generic types (PriceData, RWMap, Set) out of relaykit/types into the host repo's top-level types package and update all importers, decoupling the relaykit submodule from the host module | refactor | low |
| `8aa5e754a86b` | Move trusted-proxy configuration logic from the main package into the middleware package, exporting it as ConfigureTrustedProxies with no behavioral change | refactor | low |
| `8a7a49072ab0` | Add a GitHub Actions workflow that mirrors GitHub releases (on non-alpha tags or manual dispatch) to GitCode, waiting for other release workflows before syncing assets. | internal | low |
| `2ec6171faa74` | Sanitize release note bodies in the GitHub release-sync workflow by replacing ASCII apostrophes with Unicode right single quotes so they cannot break the downstream single-quoted shell script in sync_to_gitcode. | internal | low |
| `6d57d250f88e` | Extend the GitCode release-sync workflow by normalizing asset filenames and publishing a bootstrap asset plus an asset matrix as workflow job outputs | internal | low |
| `f3ab2cff36b3` | Enhance the GitCode release-sync workflow to query the GitCode API (using GITCODE_TOKEN/GITCODE_REPOSITORY) and expose has_bootstrap_asset/has_matrix_assets job outputs so bootstrap and matrix asset handling can be driven by what already exists on GitCode rather than purely local file inspection. | internal | low |
| `a043eef559a9` | Add Gemini-to-OpenAI chat streaming response conversion and ensure converted SSE streams emit proper terminal events (finish_reason/[DONE] for OpenAI, message_start/content_block_stop/message_delta/message_stop for Claude) with usage propagation across Gemini→OpenAI, Gemini→Claude, and OpenAI Responses→Claude paths. | feature | medium |
| `b27b2b1d6f72` | Fix iPad login sessions being misidentified as macOS by prioritizing iPad detection in the user-agent parsing of sessionDevice (iPad Safari UAs contain 'Mac OS X'), and relocate the utility tests into a __tests__ directory. | bugfix | low |
| `e99a9bd86fb2` | Introduces per-channel HTTP transport controls (protocol selection and HTTP/2 connection sharding) with validation, a sharded transport implementation in the HTTP client layer, relay adapters passing channel settings through, and corresponding web UI/i18n support. | feature | medium |
## 2026-08-24 — 10 commits from new-api#main

| SHA | Intent | Type | Risk |
|-----|--------|------|------|
| `ae17f2749d16` | Revert opaque background/z-index on the JSON code editor's line-number layer that was overlaying and hiding highlighted editor content, accepting the prior horizontal-scroll gutter overlap as a known cosmetic tradeoff | bugfix | low |
| `8b41defbe0d9` | Register the new Gemini GA image models 'gemini-3-pro-image' and 'gemini-3.1-flash-image' in the channel model list and mark them as image-generation models in default Gemini settings | feature | medium |
| `08f88d25e588` | Add key-format-based dispatch for the Tencent channel so single-segment TokenHub API keys are routed through the OpenAI-compatible adaptor (with the TokenHub base URL) while legacy three-segment ak/sk keys continue using the native TC3 adaptor. | feature | medium |
| `ab65d2582feb` | Fix the wallet recharge form so the topup amount field can be fully cleared (empty string no longer reset to "0" by the prop-sync effect), and enforce the global minimum topup per payment method via Math.max with getMinTopupAmount. | bugfix | low |
| `3e1e72827988` | Add a 500ms debounce to the users table search input to avoid triggering filtering/fetching on every keystroke (performance improvement) | feature | low |
| `2d23cdf29154` | Adds admin-configurable per-tool pricing with cross-provider surcharge settlement in billing, a new Sub2API upstream channel type, a /v1/alpha/search relay endpoint, and surcharge breakdown display in the usage-log UI. | feature | medium |
| `bc14c18f6024` | Removes the legacy unrefunded-failed-task reconciliation sweep and its CAS tests, advances the refund legacy cutoff to 2026-02-22, and gates the async-task poller on unfinished sync tasks instead of the old polling-work check. | breaking | high |
| `398cdafecf29` | Adds a new 'New API' upstream channel type with its own relay adaptor, API/channel type constants, an OpenAI ResponseCompact endpoint type, and corresponding web UI configuration, refactoring shared logic out of the Sub2API adaptor. | feature | medium |
| `f51dd4d808d1` | Allow the Advanced Custom API type to use Responses Compact by adding APITypeAdvancedCustom to the IsResponsesCompactAPIType allowlist | bugfix | medium |
| `86ac0f7745cc` | Extract the protocol conversion layer (dto, types, relayconvert, reasonmap) into a standalone, dependency-free 'relaykit' Go submodule, decoupling converters from gin/RelayInfo/settings via new convmeta.Meta/convmeta.Options and kitutil abstractions, with byte-level conversion behavior pinned by golden snapshot tests | refactor | high |
This file is auto-generated by [upstream-semantic-sync](https://github.com/upstream-semantic-sync). It records every upstream commit that was adopted into this downstream fork.

## 2026-08-21 — 10 commits from new-api#main

| SHA | Intent | Type | Risk |
|-----|--------|------|------|
| `cb96ab0208bc` | Migrate GitHub issue templates (bug report and feature request, in both Chinese and English) from Markdown templates to structured YAML issue forms with required fields, while also updating the pre-submission guidance text. | internal | low |
| `cbd9b30aa487` | Remove empty `title` and `assignees` fields from GitHub issue form templates, whose empty values invalidated the forms and hid them from the issue chooser | internal | low |
| `84a79b6807ac` | Log the raw upstream response body when an error response parses as valid JSON but yields an empty error message, so the failure remains diagnosable instead of surfacing a bare status-code error. | bugfix | low |
| `27235a277a0f` | Fix the model create/edit drawer so submitting the form no longer deletes existing pricing entries for model names whose pricing was never loaded into the form, by tracking the actually-prefilled model name and scoping the delete-then-readd to it via a shared readPricingConfig helper. | bugfix | medium |
| `bf8cfcc51267` | Fix model-mutate drawer so react-query refetches of system options don't reset the form mid-edit, by keying the load effect off a boolean and reading modelSettings via a ref instead of depending on its unstable object identity. | bugfix | medium |
| `a0d0e5049e2d` | Stabilize inline channel priority editing in the web UI by debouncing updates, committing spinner edits only on Enter or container blur (not on +/- button blur), and preserving table row identity when priority changes reorder the list. | bugfix | medium |
| `18b0b7631a99` | Generalizes the priority-only update scheduler and table cell into a generic channel-field update utility (supporting 'priority' and 'weight'), renaming createChannelPriorityUpdateScheduler to createChannelFieldUpdateScheduler with no behavior change. | refactor | low |
| `eb4a1bd19332` | Unify the admin JSON editing UX by replacing raw JSON textareas across system settings and channel workflows with a shared Yace-based code editor (syntax highlighting, copy, formatting, validation, cursor feedback, accessibility fixes), backed by extracted utilities and tests. | feature | medium |
| `257223be2675` | Update README badges: replace GoReportCard badge and remove Product Hunt badge in favor of AtomGit G-Star badges | internal | low |
| `5ede832d80d8` | Update README badges across localized docs: replace GoReportCard badge with AtomGit G-Star badge, update Trendshift repository ID to the QuantumNous org, add HelloGitHub feature badge, and remove Product Hunt badge | internal | low |
