/*
Copyright (C) 2023-2026 QuantumNous
Copyright (C) 2026 CubeRouter

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/

export const meta = {
  apiVersion: 1,
  key: "cuberouter-suno",
  name: "CubeRouter Suno",
  icon: "Suno",
  description: {
    en: "CubeRouter Suno music, lyrics, style and vocal separation generation (AstraFlow upstream).",
    zh: "CubeRouter Suno 音乐生成（星图上游）：音乐、歌词、风格与分轨。",
  },
  version: "1.0.3",
  author: { name: "Wei Liu" },
  models: [
    "suno-v6",
    "suno-v6-mini",
    "suno-v6-wild",
    "suno-v5.5",
    "suno-v5",
    "suno-v4.5-all",
    "suno-v4.5+",
    "suno-v4.5",
    "suno-v4",
    "suno-generate-lyrics",
    "suno-boost-music-style",
    "suno-vocal-separation",
  ],
  fetchMode: "per_task",
  usageSchema: {
    request_count: {
      type: "number",
      unit: "count",
      description: {
        en: "Upstream requests billed for this task (one per submit).",
        zh: "本任务的上游请求次数（每次提交计 1）。",
      },
    },
    task: {
      enum: ["create", "cover", "extend", "upload-extend", "lyrics", "style", "separation"],
      description: {
        en: "Suno task type.",
        zh: "Suno 任务类型。",
      },
    },
  },
  usageExamples: [
    { label: "create", facts: { request_count: 1, task: "create" } },
    { label: "cover", facts: { request_count: 1, task: "cover" } },
    { label: "extend", facts: { request_count: 1, task: "extend" } },
    { label: "upload-extend", facts: { request_count: 1, task: "upload-extend" } },
    { label: "lyrics", facts: { request_count: 1, task: "lyrics" } },
    { label: "style", facts: { request_count: 1, task: "style" } },
    { label: "separation", facts: { request_count: 1, task: "separation" } },
  ],
  // Suno 风格入口：提交 /cuberouter/suno/submit/:action、查询 /cuberouter/suno/fetch/:task_id；
  // 另保留只读任务视图 /cuberouter-suno/tasks/:task_id（通用任务读取面不返回 data，
  // 歌词等输出正文靠它读取）。通用提交入口仍是 /v1/tasks/cuberouter-suno。
  routes: [
    { method: "POST", path: "/cuberouter/suno/submit/:action", type: "submit", decode: "decodeSubmit", render: "renderSubmit" },
    { method: "GET", path: "/cuberouter/suno/fetch/:task_id", type: "query", render: "renderFetchTask" },
    { method: "GET", path: "/cuberouter-suno/tasks/:task_id", type: "query", render: "taskView" },
  ],
};

// 上游 = 星图 / ModelVerse（https://api.modelverse.cn）任务接口：
// 提交 POST /v1/tasks/submit，查询 GET /v1/tasks/status?task_id=。
// 音乐模型的任务体是 Suno 方言（不是 Ark content 数组）：
//   灵感模式   input.gpt_description_prompt
//   自定义模式 input.prompt + parameters.tags/title
//   翻唱       parameters.task=cover + parameters.upload_url 等
//   续写       parameters.task=extend + parameters.audio_id/default_param_flag 等
//   上传续写   parameters.task=upload-extend + input.audio_url 等
// 结果统一在 output.urls（mp3/wav 等），状态 Pending/Running/Success/Failure。
const SUBMIT_PATH = "/v1/tasks/submit";
const STATUS_PATH = "/v1/tasks/status";

const MUSIC_TASKS = ["create", "cover", "extend", "upload-extend"];
const AUXILIARY_TASKS = ["lyrics", "style", "separation"];
const MUSIC_MODEL_NAMES = [
  "suno-v6",
  "suno-v6-mini",
  "suno-v6-wild",
  "suno-v5.5",
  "suno-v5",
  "suno-v4.5-all",
  "suno-v4.5+",
  "suno-v4.5",
  "suno-v4",
];

const LYRICS_MODEL = "suno-generate-lyrics";
const STYLE_MODEL = "suno-boost-music-style";
const SEPARATION_MODEL = "suno-vocal-separation";

function trimmed(value) {
  if (value === undefined || value === null) return "";
  return String(value).trim();
}

function asObject(value) {
  if (!value || typeof value !== "object" || Array.isArray(value)) return null;
  return value;
}

function parseMetadata(request) {
  let metadata = request && request.metadata;
  if (typeof metadata === "string") {
    try {
      metadata = JSON.parse(metadata);
    } catch (err) {
      metadata = null;
    }
  }
  return asObject(metadata) || {};
}

function upstreamBase(ctx) {
  return trimmed(ctx && ctx.baseUrl).replace(/\/+$/, "");
}

function isMusicModel(model) {
  return model !== LYRICS_MODEL && model !== STYLE_MODEL && model !== SEPARATION_MODEL;
}

// task 取值：音乐模型默认 create；辅助模型按模型固定。非法值在预扣费之前拒绝，
// 避免把未知任务类型发到上游后按次扣费。
function normalizedTask(model, metadata) {
  if (model === LYRICS_MODEL) return "lyrics";
  if (model === STYLE_MODEL) return "style";
  if (model === SEPARATION_MODEL) return "separation";
  const task = trimmed(metadata.task) || "create";
  if (!MUSIC_TASKS.includes(task)) {
    throw new Error("task must be one of " + MUSIC_TASKS.join(", "));
  }
  return task;
}

function descriptor(ctx, model, input, parameters, action) {
  const body = { model: model };
  if (Object.keys(input).length > 0) body.input = input;
  if (Object.keys(parameters).length > 0) body.parameters = parameters;
  return {
    url: upstreamBase(ctx) + SUBMIT_PATH,
    method: "POST",
    headers: {
      "Content-Type": "application/json",
      Accept: "application/json",
      Authorization: "Bearer " + ctx.apiKey,
    },
    body: body,
    action: action,
  };
}

export function buildSubmitRequest(ctx) {
  const request = asObject(ctx.requestBody) || {};
  const model = trimmed(ctx.upstreamModel) || trimmed(request.model);
  if (!model) throw new Error("model is required");
  const metadata = parseMetadata(request);
  const prompt = trimmed(request.prompt);

  // 高级参数直通：metadata.input / metadata.parameters 里的字段原样合并，
  // 覆盖上游新增参数时无需改插件。
  const input = Object.assign({}, asObject(metadata.input) || {});
  const parameters = Object.assign({}, asObject(metadata.parameters) || {});

  const task = normalizedTask(model, metadata);

  if (model === SEPARATION_MODEL) {
    const audioURL = trimmed(metadata.audio_url) || trimmed(input.audio_url);
    if (!audioURL) throw new Error("metadata.audio_url is required for vocal separation");
    input.audio_url = audioURL;
    return descriptor(ctx, model, input, parameters, "VOCAL_SEPARATION");
  }

  if (model === LYRICS_MODEL) {
    if (!prompt) throw new Error("prompt is required for lyrics generation");
    input.prompt = prompt;
    return descriptor(ctx, model, input, parameters, "LYRICS");
  }

  if (model === STYLE_MODEL) {
    if (prompt) input.gpt_description_prompt = prompt;
    if (trimmed(metadata.tags)) parameters.tags = trimmed(metadata.tags);
    if (trimmed(metadata.title)) parameters.title = trimmed(metadata.title);
    if (metadata.make_instrumental === true) parameters.make_instrumental = true;
    return descriptor(ctx, model, input, parameters, "STYLE");
  }

  if (task === "create") {
    const explicitMode = trimmed(metadata.mode).toLowerCase();
    const customMode =
      metadata.custom_mode === true ||
      explicitMode === "custom" ||
      trimmed(metadata.tags) !== "" ||
      trimmed(metadata.title) !== "";
    if (customMode) {
      input.prompt = trimmed(metadata.lyrics) || prompt;
      if (trimmed(metadata.tags)) parameters.tags = trimmed(metadata.tags);
      if (trimmed(metadata.title)) parameters.title = trimmed(metadata.title);
      if (!trimmed(parameters.tags)) {
        throw new Error("metadata.tags is required for custom mode");
      }
    } else {
      const description = trimmed(metadata.gpt_description_prompt) || prompt;
      if (!description) throw new Error("prompt is required for inspiration mode");
      input.gpt_description_prompt = description;
    }
  } else {
    // cover / extend / upload-extend：必需字段由上游校验（提交期 400 免费）。
    parameters.task = task;
    if (trimmed(metadata.tags)) parameters.tags = trimmed(metadata.tags);
    if (trimmed(metadata.title)) parameters.title = trimmed(metadata.title);
    if (task === "cover") {
      parameters.generation_type = trimmed(parameters.generation_type) || "TEXT";
      if (!trimmed(parameters.upload_url)) {
        throw new Error("metadata.parameters.upload_url is required for cover");
      }
      if (prompt) input.prompt = prompt;
    } else if (task === "extend") {
      if (!trimmed(parameters.audio_id)) {
        throw new Error("metadata.parameters.audio_id is required for extend");
      }
      if (prompt) input.prompt = prompt;
    } else if (task === "upload-extend") {
      if (!trimmed(input.audio_url)) {
        throw new Error("metadata.input.audio_url is required for upload-extend");
      }
      if (prompt) input.prompt = prompt;
    }
  }

  if (metadata.make_instrumental === true) parameters.make_instrumental = true;
  if (metadata.make_instrumental === false) parameters.make_instrumental = false;

  return descriptor(ctx, model, input, parameters, "MUSIC");
}

export function parseSubmitResponse(ctx, resp) {
  const body = asObject(resp && resp.body);
  if (!body) throw new Error("unexpected submit response");
  const output = asObject(body.output) || {};
  const taskId = trimmed(output.task_id);
  if (taskId) return { taskId: taskId, taskData: body };
  const error = asObject(body.error) || {};
  const message = trimmed(error.message) || trimmed(body.message);
  throw new Error(message || "submit response has no task id");
}

export function buildQueryRequest(ctx) {
  return {
    url: upstreamBase(ctx) + STATUS_PATH + "?task_id=" + encodeURIComponent(ctx.taskId),
    method: "GET",
    headers: { Accept: "application/json", Authorization: "Bearer " + ctx.apiKey },
  };
}

export function parseTaskResult(ctx, body) {
  const envelope = asObject(body) || {};
  const output = asObject(envelope.output) || {};
  const status = trimmed(output.task_status).toUpperCase();
  if (status === "PENDING") return { status: "QUEUED" };
  if (status === "RUNNING") return { status: "IN_PROGRESS" };
  if (status === "SUCCESS") {
    const urls = resultURLs(envelope);
    return urls.length > 0 ? { status: "SUCCESS", url: urls[0] } : { status: "SUCCESS" };
  }
  if (status === "FAILURE" || status === "FAILED" || status === "EXPIRED") {
    return { status: "FAILURE", reason: trimmed(output.error_message) || "task failed" };
  }
  return { status: "UNKNOWN", reason: "unrecognized task status: " + trimmed(output.task_status) };
}

function decodeData(data) {
  if (typeof data === "string") {
    try {
      return JSON.parse(data);
    } catch (err) {
      return null;
    }
  }
  return asObject(data);
}

function resultURLs(envelope) {
  const body = asObject(envelope) || {};
  const output = asObject(body.output) || {};
  const urls = Array.isArray(output.urls) ? output.urls : [];
  const unique = [];
  for (const value of urls) {
    const url = trimmed(value);
    if (url && !unique.includes(url)) unique.push(url);
  }
  return unique;
}

function artifactMimeType(url) {
  const lower = url.toLowerCase().split("?")[0];
  if (lower.endsWith(".wav")) return "audio/wav";
  if (lower.endsWith(".flac")) return "audio/flac";
  if (lower.endsWith(".m4a")) return "audio/mp4";
  if (lower.endsWith(".ogg") || lower.endsWith(".opus")) return "audio/ogg";
  return "audio/mpeg";
}

export function listArtifacts(task) {
  if (!task || task.status !== "SUCCESS") return [];
  const data = decodeData(task.data);
  const artifacts = [];
  for (const url of resultURLs(data)) {
    artifacts.push({
      key: "audio-" + utils.hmacSHA256(url, "new-api:cuberouter-suno:artifact-key"),
      type: "audio",
      mimeType: artifactMimeType(url),
    });
  }
  return artifacts;
}

export function buildContentRequest(ctx) {
  const data = decodeData(ctx.data);
  const url = resultURLs(data).find(function (candidate) {
    return "audio-" + utils.hmacSHA256(candidate, "new-api:cuberouter-suno:artifact-key") === ctx.artifactKey;
  });
  if (!url) throw new Error("artifact_not_found");
  return { url: url, method: ctx.clientRequest.method, credentialless: true };
}

// Suno 风格入口：POST /cuberouter/suno/submit/:action。
// action 映射：music → 音乐模型（默认 suno-v6）、lyrics → suno-generate-lyrics、
// separation → suno-vocal-separation、style → suno-boost-music-style。
// 把 Suno-API 风格的顶层字段归一化成统一任务请求（prompt + metadata），
// 再复用 buildSubmitRequest；这样两条入口共享同一份上游成形与计费逻辑。
function decodeNativeSubmit(ctx) {
  const params = asObject(ctx && ctx.params) || {};
  const rawAction = trimmed(params.action).toLowerCase();
  let action;
  if (rawAction === "music" || rawAction === "song" || rawAction === "create") {
    action = "music";
  } else if (rawAction === "lyrics" || rawAction === "lyric") {
    action = "lyrics";
  } else if (rawAction === "separation" || rawAction === "vocal-separation" || rawAction === "separate") {
    action = "separation";
  } else if (rawAction === "style") {
    action = "style";
  } else {
    throw new Error("unsupported action: " + rawAction);
  }

  const body = asObject(ctx && ctx.body) || {};
  if (body.kind !== "json") throw new Error("JSON body required");
  const value = asObject(body.value) || {};

  let model;
  if (action === "lyrics") {
    model = LYRICS_MODEL;
  } else if (action === "style") {
    model = STYLE_MODEL;
  } else if (action === "separation") {
    model = SEPARATION_MODEL;
  } else {
    const requested = trimmed(value.model);
    if (requested) {
      if (!MUSIC_MODEL_NAMES.includes(requested)) {
        throw new Error("unsupported model: " + requested);
      }
      model = requested;
    } else {
      const mv = trimmed(value.mv);
      model = MUSIC_MODEL_NAMES.includes(mv) ? mv : "suno-v6";
    }
  }

  const prompt = trimmed(value.prompt) || trimmed(value.gpt_description_prompt);
  const metadata = {};
  const incoming = asObject(value.metadata);
  if (incoming) Object.assign(metadata, incoming);
  if (trimmed(value.gpt_description_prompt)) metadata.gpt_description_prompt = trimmed(value.gpt_description_prompt);
  if (trimmed(value.tags)) metadata.tags = trimmed(value.tags);
  if (trimmed(value.title)) metadata.title = trimmed(value.title);
  if (value.make_instrumental === true || value.make_instrumental === false) {
    metadata.make_instrumental = value.make_instrumental;
  }
  if (value.custom_mode === true || value.custom_mode === false) {
    metadata.custom_mode = value.custom_mode;
  }
  if (trimmed(value.task)) metadata.task = trimmed(value.task);
  if (trimmed(value.audio_url)) metadata.audio_url = trimmed(value.audio_url);
  if (asObject(value.parameters)) metadata.parameters = value.parameters;
  if (asObject(value.input)) metadata.input = value.input;

  if (action === "lyrics" && !prompt) {
    throw new Error("prompt is required for lyrics");
  }
  if (action === "separation") {
    const nestedInput = asObject(metadata.input) || {};
    if (!trimmed(metadata.audio_url) && !trimmed(nestedInput.audio_url)) {
      throw new Error("audio_url is required for separation");
    }
  }

  const requestBody = { model: model, prompt: prompt };
  if (Object.keys(metadata).length > 0) requestBody.metadata = metadata;
  const taskAction = action === "music" ? "MUSIC" : action === "lyrics" ? "LYRICS" : action === "separation" ? "VOCAL_SEPARATION" : "STYLE";
  return { kind: "submit", model: model, action: taskAction, requestBody: requestBody };
}

function renderNativeTask(task) {
  return {
    task_id: task.task_id || "",
    platform: task.platform || "cuberouter-suno",
    status: task.status || "",
    progress: task.progress || "",
    fail_reason: task.fail_reason || "",
    submit_time: task.created_at || 0,
    finish_time: task.finished_at || 0,
    data: task.data === undefined ? null : task.data,
  };
}

export const native = {
  decodeSubmit: decodeNativeSubmit,
  renderSubmit: function (ctx, task) {
    return { code: "success", message: "", data: String(task.task_id || "") };
  },
  renderFetchTask: function (ctx, task) {
    return { code: "success", message: "", data: renderNativeTask(task) };
  },
  error: function (ctx, error) {
    const value = asObject(error) || {};
    return {
      code: trimmed(value.code) || "error",
      message: trimmed(value.message) || "request failed",
      data: null,
    };
  },
  taskView: function (ctx, task) {
    return {
      task_id: task.task_id || "",
      platform: task.platform || "cuberouter-suno",
      status: task.status || "",
      progress: task.progress || "",
      fail_reason: task.fail_reason || "",
      created_at: task.created_at || 0,
      finished_at: task.finished_at || 0,
      data: task.data === undefined ? null : task.data,
    };
  },
};

export function extractUsage(ctx) {
  // 按次计费走 ModelPrice；billing_ratios 返回 null，避免系数乘到按次价格上。
  if (ctx.usagePurpose === "billing_ratios") return null;
  const request = asObject(ctx.requestBody) || {};
  const metadata = parseMetadata(request);
  const model = trimmed(ctx.upstreamModel) || trimmed(request.model);
  return { request_count: 1, task: normalizedTask(model, metadata) };
}
