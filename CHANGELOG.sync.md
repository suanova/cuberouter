# Upstream Sync Changelog

## 2026-08-24 — 10 commits from new-api#main

| SHA | Intent | Type | Risk |
|-----|--------|------|------|
| `53a8739eedbf` | Dependabot-бот обновляет косвенную dev-зависимость fast-uri с 3.1.4 до 3.1.5, обновляя только записи версии/integrity в electron/package-lock.json без изменений кода. | internal | low |
| `e5efc73cdb49` | Dependabot bump of the indirect dev dependency tar from 7.5.16 to 7.5.22 in the electron workspace (lockfile-only refresh, picking up upstream bug/security fixes in node-tar). | internal | low |
| `2a0ce3475c2d` | Add pre-payment validation that rejects top-up orders whose amount would credit zero or unrepresentable quota, applied consistently across all payment providers (epay, Stripe, Creem, Waffo, Waffo-pancake) instead of failing after payment | bugfix | medium |
| `cf38105a9946` | Dependabot lockfile-only bump of the indirect dev dependency js-yaml from 4.3.0 to 4.3.1 in the electron package | internal | low |
| `bbf67df0499c` | Dependabot bump of the Electron devDependency from 39.8.5 to 39.8.10 (patch-level update pulling in upstream Electron runtime fixes) | internal | low |
| `47ba9d2c63d6` | Add wallet quota capacity validation across all top-up payment flows so recharges that would exceed the user's quota limit are rejected before payment, with validation helpers now returning the credited quota amount for capacity checks. | bugfix | medium |
| `7d09c6954ef3` | Forward the prompt_cache_key field when converting OpenAI Chat Completions requests to OpenAI Responses requests, so prompt caching hints are no longer silently dropped during protocol conversion. | bugfix | medium |
| `e90a7c48e5e4` | Adds field passthrough controls for gateway channels by generalizing hardcoded channel-type checks into exported sets of passthrough-capable channel types (OpenAI-compatible and Claude) used by the channel mutation drawer and form logic. | feature | medium |
| `4442bb302898` | Fix the OpenAI-chat-to-Claude-messages converter to omit the `tools` field entirely (instead of serializing an empty array) when the incoming request has no tools, since Claude rejects/behaves badly on empty tools arrays. | bugfix | medium |
| `116255f076a3` | Align frontend custom OAuth binding types with backend responses (provider_id as number, shared CustomOAuthBinding type moved to lib/oauth with an indexer helper) and restore access-policy guidance/templates in the OAuth provider form dialog. | bugfix | medium |
## 2026-08-24 — 10 commits from new-api#main

| SHA | Intent | Type | Risk |
|-----|--------|------|------|
| `ffeb1b24ef85` | Fix stale single-use Cloudflare Turnstile tokens on failed login by clearing the token and remounting the widget (via a key bump) before each submit, and clearing the token when it expires, so retry attempts always present a fresh token. | bugfix | medium |
| `3d5dc36f1d85` | Fix Gemini-style model listing on /v1/models: accept key via query param for the exact /v1/models path in TokenAuth, and route Gemini-authenticated list requests to ListModels instead of RetrieveModel. | bugfix | medium |
| `d7992672a606` | Fix a race in OAuth/WeChat account binding where persisting the provider user ID rewrote the entire user row, clobbering concurrent changes to role/status/group; binding now updates only the provider-ID column via model.UpdateUserBindColumn, keyed by a new ProviderUserIDColumn method on the oauth.Provider interface. | bugfix | medium |
| `50e5377ea5fe` | Make recharge/topup order settlement atomic by wrapping order status update, quota crediting, and user-cache refresh in a single transaction (with quota reservation and a strict int32-boundary check), preventing partial or duplicate settlement from concurrent payment webhooks. | bugfix | medium |
| `ccd535ef8e50` | Harden concurrent quota reservation and channel status persistence (narrow status-only updates, removed whole-row SaveWithoutKey, safer quota reservation flow) and fix float-to-int quota conversions. | bugfix | medium |
| `58d4e9bd3bb0` | Fix async-task refund accounting so refunds decrement user (and channel) used_quota in addition to restoring quota, preventing total quota (quota + used_quota) from inflating with each refund; consolidates Midjourney failure refunds into a shared service helper with billing channel/token tracking on tasks. | bugfix | medium |
| `15cfdeddef46` | Fix fetched-models dialog drifting out of sync with the channel form by always passing the current parsed models array, and enforce at the type level that onModelsSelected and existingModelsOverride are provided together via a discriminated union. | bugfix | medium |
| `93d2df85f824` | Fix Ali (DashScope) channel to select image endpoints/protocols and async headers based on the mapped upstream model name (e.g. Qwen Image 3) instead of the original pre-mapping request model name. | bugfix | medium |
| `626058075524` | Dependabot bump of electron-builder (26.7.0 → 26.15.3) and its transitive builder-util-runtime (9.5.1 → 9.7.0) in the electron packaging toolchain | internal | low |
| `f250f3b589c8` | Dependabot bumps the dompurify dependency (both the direct dependency pin and the npm override) from 3.4.11 to 3.4.13 in the web workspace, pulling in upstream patch fixes for the HTML/XSS sanitizer. | internal | low |
## 2026-08-24 — 10 commits from new-api#main

| SHA | Intent | Type | Risk |
|-----|--------|------|------|
| `7dd1000a190d` | Debounce search inputs across admin tables, dialogs, and data hooks (via a searchDebounceMs prop and useDebounce) to reduce server round-trips and improve large-list search performance. | feature | low |
| `eab18a835791` | Centralize reasoning-effort capture through a new RelayInfo.SetReasoningEffort setter (plus override/normalization logic) so the effective reasoning effort is recorded consistently in usage logs across DeepSeek/OpenAI/xAI adaptors and the Claude handler, and displayed in the web usage-log details UI. | bugfix | medium |
| `85feb7a345d2` | Expose authenticated user identity and group context (user_id, user_group, token_group, using_group) as built-in variables for parameter override conditions, enabling user/group-conditional request param overrides. | feature | medium |
| `8ad159a3bbc2` | Fix Ollama relay to preserve thinking/reasoning content and tool-call IDs across message conversions, and always emit the 'stream' field in requests. | bugfix | medium |
| `d49160f0e543` | Fix backend console-settings length validation to count UTF-16 code units (matching frontend/JS character semantics) instead of raw bytes, so multi-byte (e.g., CJK/emoji) content is no longer falsely rejected; also switch JSON parsing to common.UnmarshalJsonStr. | bugfix | low |
| `4cf9107f0437` | Trace which conditional request-based multipliers matched during billing expression evaluation and surface that trace (condition, multiplier, matched) in usage logs and the web UI so applied price adjustments are highlighted. | feature | medium |
| `9c97e78aced5` | Require explicit user confirmation before generating/rotating an access token, stopping automatic token generation when the dialog opens and clearing the token on close. | bugfix | medium |
| `253a74dd1b47` | Wire presence_penalty and frequency_penalty through the OpenAI Responses request DTO and the Chat↔Responses request converters so the values survive format conversion, while the Codex adaptor explicitly strips them because the Codex backend rejects those fields. | bugfix | medium |
| `bb234ff41861` | Removes the '-compact' model-name suffix convention that auto-routed suffixed models to the /v1/responses/compact endpoint, consolidating compact support detection into a channel-aware SupportsResponsesCompact(channelType, apiType) check. | breaking | high |
| `4eaeefbdf5b9` | Fix mobile sidebar UX by disabling TanStack Router link preloading on mobile and removing/adjusting touch-event handling in the shared sidebar components so iOS taps on sidebar items are no longer swallowed. | bugfix | medium |
## 2026-08-24 — 10 commits from new-api#main

| SHA | Intent | Type | Risk |
|-----|--------|------|------|
| `ea4f021012cd` | Move transport retry/replay metadata off RelayInfo and onto outbound request bodies via a new ReplayableBody interface, replacing the ReaderOnly wrapper. | refactor | medium |
| `0cd9dc85e334` | Fix lost-update race in access token generation by replacing read-modify-write of the full user record with a targeted single-column UPDATE (new UpdateUserAccessToken), and harden User.Update semantics with expanded tests. | bugfix | medium |
| `c9bc038649d1` | Extract fetched-model categorization logic from the fetch-models dialog into a new dedicated model-categories lib module with refined grouping rules (including edge-case fixes such as hy3 models), and tidy dialog code (null-safe channel guard, simplified name normalization). | feature | medium |
| `b941253aea6b` | Fix channel connectivity testing to dispatch Claude and Gemini endpoint channels using their native request formats (including Gemini streaming via the :streamGenerateContent URL action) instead of only OpenAI-format requests | bugfix | medium |
| `1da23d6b3342` | Add a per-user critical rate-limit middleware (UserCriticalRateLimit with a 'UC:' scope prefix) and apply it to the access-token generation and affiliate quota transfer routes, complementing the existing global critical rate limit to curb abuse by authenticated users. | feature | medium |
| `e926e5cacee2` | Fix precision loss when entering/displaying redemption code quota amounts by hardening shared currency/format helpers and reworking the redemption mutate drawer form, with new tests guarding update-data integrity. | bugfix | medium |
| `5c3abffe8572` | Convert the GitCode release-sync workflow to manual dispatch only (dropping automatic tag-push triggers) and add an optional `sync_files` input, merging asset preparation into the create/update-release job. | internal | low |
| `2399de97daf6` | Ali (DashScope) adaptor no longer injects a near-greedy top_p into requests that omit it, and clamps explicit top_p boundary values to two decimals (0.99/0.01) for platform compatibility | bugfix | medium |
| `823e26304a39` | Fix model categorization so Qwen TTS models (e.g., qwen-tts-*) are no longer misclassified as OpenAI by moving 'tts-' from a loose substring keyword into the anchored regex that only matches at the start or after a '/', '.', or ':' delimiter. | bugfix | low |
| `5d3423bec13f` | Adds a new 'auto-ban-only' automatic channel test mode that restricts scheduled channel testing to channels with auto-disable (AutoBan) enabled, with settings validation, UI option, and i18n support. | feature | medium |
## 2026-08-24 — 10 commits from new-api#main

| SHA | Intent | Type | Risk |
|-----|--------|------|------|
| `84834eee859f` | Expose the stream_status field (stream end status/reason) to regular log owners instead of admins only, by no longer stripping it from user log serialization and removing the admin-only check in the web details dialog. | feature | medium |
| `8461e5339d48` | Fix New API channel image edits by delegating ConvertImageRequest to the embedded OpenAI adaptor instead of passing the request through unchanged, so multipart image-edit payloads are correctly converted/preserved. | bugfix | medium |
| `e78e1db1e4ed` | Fix OAuth callback misclassifying plain logins as account-bind flows when the tab has a foreign window.opener, by requiring a sessionStorage stamp that only the same-origin bind popup can carry (ambiguity now resolves to 'login'). | bugfix | medium |
| `aa7d0d39a4a7` | Replace hardcoded text-[13px] with standard text-sm Tailwind class on public header nav links for visual consistency with other nav components | refactor | low |
| `9724ef1b248a` | Add DeepSeek channel support for the OpenAI Responses API: route RelayModeResponses to the /responses endpoint and implement ConvertOpenAIResponsesRequest to parse DeepSeek V4 thinking suffixes into reasoning effort settings. | feature | medium |
| `df43f801536b` | Fix tiered-expression billing so that when an auto-group retry switches groups, the frozen BillingSnapshot's group-dependent fields are refreshed from the final selected group—creating a billing session if the initial free group skipped pre-consume, raising the reservation for pricier groups, and settling actual usage against the final group. | bugfix | medium |
| `cfaba1dd6754` | Harden billing consistency for tiered auto-group retries: wallet reserve deltas are deducted unconditionally in arrears, FreeModel is cleared when switching to a paid group, and channel bookkeeping (use_channel, GroupRatioInfo) is reordered around billing preparation. | bugfix | medium |
| `bd585d78efd4` | Propagate the client request context into AWS Bedrock invocations so client disconnects cancel in-flight requests and skip retries, plus log the effective usage billing path in the billing service | bugfix | medium |
| `0ab02020603d` | Add per-token customizable auto-group ordering end-to-end: backend token auto-group storage with inheritance semantics, middleware context propagation (new ContextKeyTokenAutoGroups), auto-group-aware channel selection and model listing, plus a redesigned frontend Auto group order editor with compact inherited-order chips and border-flow highlight visuals. | feature | medium |
| `d6b5ce99de49` | Wire replayable Request.GetBody from BodyStorage into outbound relay requests so the HTTP/2 transport can transparently retry after upstream REFUSED_STREAM/GOAWAY resets; also stop following upstream redirects. | bugfix | medium |
## 2026-08-24 — 10 commits from new-api#main

| SHA | Intent | Type | Risk |
|-----|--------|------|------|
| `2cf3c8d71e92` | Adds a documentation rule to AGENTS.md requiring the relaykit Go module touch remain independently buildable (no imports from the root new-api module) and mandating verification via 'cd relaykit && GOWORK=off go build ./...'. | internal | low |
| `f01c13b0863f` | Adds a GitHub Actions CI workflow that builds and tests the Go backend (root and relaykit modules) and typechecks/tests the Bun frontend on every pull request. | internal | low |
| `c3db41407dd1` | Security fix: log slow/error SQL in parameterized form and sanitize driver error messages to prevent credentials/sensitive values leaking into logs, adding a validated SQL_SLOW_THRESHOLD_MS config for the slow-query threshold. | bugfix | medium |
| `8e2bfe278b86` | Removes the ineffective exported Mutex field from CustomEvent (a no-op since Render/WriteContentType use value receivers that copy the lock) and documents that streaming callers must serialize SSE writes themselves; also makes an SMTP test dial IPv6-safe via net.JoinHostPort. | refactor | medium |
| `1db6ae19576d` | Add `go vet` checks for the root Go module and the relaykit submodule to the backend CI job, and pin checkout to the PR base repo/ref so merged-PR workflow runs analyze the target branch. | internal | low |
| `afe16c64cd73` | Add README.md documentation for the standalone RelayKit Go module, describing its cross-protocol conversion capabilities (OpenAI Chat/Responses, Claude, Gemini), quality-level support matrix, installation, packages, and usage examples. | internal | low |
| `c27d1ef651c6` | Tune GitHub Linguist language detection: exclude TSX view files, mark electron/ as vendored, and mark the generated routeTree as generated so stats reflect the Go service. | internal | low |
| `cb4c8c02f81d` | Adds a configurable OIDC display name setting (falling back to "OIDC" when empty/whitespace) that is exposed via the status API, used as the OAuth provider name, and shown on login buttons and settings UIs in both web themes. | feature | medium |
| `66ee6b8f9889` | Fix Qwen thinking_budget passthrough so the parameter is preserved (including explicit zero values) when the upstream model supports it, and stripped when the upstream model does not; also removes dead adaptor code across channels. | bugfix | medium |
| `0f9f668c6076` | Add zstd (Content-Encoding: zstd) request-body decompression to DecompressRequestMiddleware, alongside existing gzip and brotli support | feature | medium |
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
