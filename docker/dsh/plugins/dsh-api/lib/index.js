/**
 * dsh-api — DeepSeek Harness external agent HTTP API.
 *
 * Loader entry for the `dsh-api` row in the profile composition. Wraps the
 * import-free core (lib/core.js) and exports a standard Cordis plugin.
 *
 * Config (row `config:` in cordis.patch.yml):
 *   prefix           route prefix, default '/api/v1'
 *   apiKey           required key (Authorization: Bearer <key> /
 *                    X-API-Key: <key>); empty disables auth
 *   defaultPreset    agent preset for tasks that name none (default: roster default)
 *   defaultCwd       working directory for tasks that name none
 *                    (default: process.cwd(), same as Web sessions); tasks
 *                    carrying a userId use <defaultCwd>/<userId> instead
 *   defaultTimeoutMs per-task budget, default 1200000 (20 min)
 *   maxTimeoutMs     hard cap for any task, default 7200000 (2 h)
 *   questionTimeoutMs how long ask_user_question waits for a human before the
 *                    task settles it as a skip and continues, default 600000
 *                    (10 min); 0 or less waits forever
 *   imageReviewLimit  how many read_image calls one task may dispatch before
 *                    further reads are denied, default 0 (unlimited); set a
 *                    small number to cap the render-and-inspect loop
 *   retentionMs      live-agent grace window after a task settles, default 600000
 *   taskTtlMs        finished task records are pruned after this long
 *                    (default 86400000 = 24 h; <= 0 disables pruning)
 *   maxConcurrent    concurrent task cap, default 8
 *   maxBodyBytes     request body cap, default 2097152 (2 MiB)
 *   maxPromptChars   prompt length cap, default 500000
 *   maxArtifactBytes per-artifact download cap, default 67108864 (64 MiB)
 */

import { mountApi } from './core.js'

const DEFAULTS = {
  prefix: '/api/v1',
  apiKey: '',
  defaultPreset: undefined,
  defaultCwd: undefined,
  defaultTimeoutMs: 1200000,
  maxTimeoutMs: 7200000,
  questionTimeoutMs: 600000,
  imageReviewLimit: 0,
  retentionMs: 600000,
  taskTtlMs: 86400000,
  maxConcurrent: 8,
  maxBodyBytes: 2097152,
  maxPromptChars: 500000,
  maxArtifactBytes: 67108864,
}

function normalize(cfg) {
  const c = { ...DEFAULTS, ...(cfg || {}) }
  c.prefix = String(c.prefix || '/api/v1').replace(/\/+$/, '')
  if (c.defaultCwd === undefined) c.defaultCwd = process.cwd()
  c.defaultTimeoutMs = Number(c.defaultTimeoutMs) || DEFAULTS.defaultTimeoutMs
  c.maxTimeoutMs = Math.max(Number(c.maxTimeoutMs) || DEFAULTS.maxTimeoutMs, c.defaultTimeoutMs)
  const questionTimeout = Number(c.questionTimeoutMs)
  c.questionTimeoutMs = Number.isFinite(questionTimeout) && questionTimeout >= 0 ? questionTimeout : DEFAULTS.questionTimeoutMs
  const imageReviewLimit = Number(c.imageReviewLimit)
  c.imageReviewLimit = Number.isFinite(imageReviewLimit) && imageReviewLimit >= 0 ? Math.floor(imageReviewLimit) : DEFAULTS.imageReviewLimit
  c.retentionMs = Number(c.retentionMs) || DEFAULTS.retentionMs
  const ttl = Number(c.taskTtlMs)
  c.taskTtlMs = Number.isFinite(ttl) && ttl >= 0 ? ttl : DEFAULTS.taskTtlMs
  c.maxConcurrent = Math.max(1, Number(c.maxConcurrent) || DEFAULTS.maxConcurrent)
  c.maxBodyBytes = Math.max(1024, Number(c.maxBodyBytes) || DEFAULTS.maxBodyBytes)
  c.maxPromptChars = Math.max(1, Number(c.maxPromptChars) || DEFAULTS.maxPromptChars)
  c.maxArtifactBytes = Math.max(1, Number(c.maxArtifactBytes) || DEFAULTS.maxArtifactBytes)
  return c
}

const plugin = {
  name: 'dsh-api',
  inject: ['webServer', 'agents', 'timer'],
  apply(ctx, config) {
    mountApi(ctx, normalize(config))
  },
}

export default plugin
