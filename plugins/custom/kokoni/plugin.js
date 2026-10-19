export const meta = {
  apiVersion: 1,
  key: "kokoni",
  name: "Kokoni",
  icon: "text:魔芯",
  description: {
    en: "Moxin Kokoni platform async video tasks (new-api style /v1/video/generations).",
    zh: "魔芯 Kokoni 平台异步视频任务（new-api 风格 /v1/video/generations）。",
  },
  version: "1.0.1",
  author: { name: "Wei Liu" },
  models: ["moworld-t2v", "moworld-i2v"],
  fetchMode: "per_task",
  usageSchema: {
    seconds: {
      type: "number",
      unit: "second",
      description: {
        en: "Requested video duration in seconds.",
        zh: "请求的视频时长，单位为秒。",
      },
    },
    resolution: {
      enum: ["480P", "720P", "1080P"],
      description: {
        en: "Requested output video resolution tier.",
        zh: "请求的输出视频分辨率档位。",
      },
    },
  },
  usageExamples: [
    { label: "1080P · 5s", facts: { seconds: 5, resolution: "1080P" } },
    { label: "720P · 8s", facts: { seconds: 8, resolution: "720P" } },
  ],
};

// 上游是 new-api 家族的平台（魔芯/Kokoni），公开入口就是它自己的异步视频任务接口：
// 提交 POST {baseUrl}/v1/video/generations，查询 GET {baseUrl}/v1/video/generations/:task_id。
const SUBMIT_PATH = "/v1/video/generations";
const DEFAULT_DURATION = 5;
// 与网关视频按秒表的缺省档位(720p)保持一致
const DEFAULT_RESOLUTION = "720P";

// "1080p"/"720P" 与 "1920x1080"/"1080*1920" 两种写法上游都收；提交流程按它自己的
// 规范再归一一次：档位大写化、尺寸改写成 "*" 分隔。
const TIER_PATTERN = /^(\d{3,4})p$/i;
const SIZE_PATTERN = /^(\d{2,5})\s*[*x×]\s*(\d{2,5})$/i;

// 计费按秒表的档位字面量（上游文档：480P/720P/1080P），其余值只做透传、不进 usage facts。
const TIERS = ["480P", "720P", "1080P"];

// 这些键由本插件显式映射（且与网关计费的取值口径一致），不再从 metadata 原样透传，
// 避免同一个参数出现两份、或 metadata 里的时长/分辨率与计费口径不一致。
const MAPPED_KEYS = {
  model: true,
  prompt: true,
  duration: true,
  seconds: true,
  resolution: true,
  size: true,
  image: true,
  images: true,
  content: true,
  mode: true,
  input_reference: true,
  metadata: true,
};

// 上游状态口径：外层 TaskDto 用 IN_PROGRESS/SUCCESS/FAILURE，内层万相原始状态用
// RUNNING/SUCCEEDED，两者都要认。
const STATUS_MAP = {
  NOT_START: "NOT_START",
  SUBMITTED: "SUBMITTED",
  QUEUED: "QUEUED",
  PENDING: "QUEUED",
  IN_PROGRESS: "IN_PROGRESS",
  RUNNING: "IN_PROGRESS",
  PROCESSING: "IN_PROGRESS",
  SUCCESS: "SUCCESS",
  SUCCEEDED: "SUCCESS",
  COMPLETED: "SUCCESS",
  FAILURE: "FAILURE",
  FAILED: "FAILURE",
  ERROR: "FAILURE",
  CANCELED: "FAILURE",
  CANCELLED: "FAILURE",
};

function trimmed(value) {
  if (value === undefined || value === null) return "";
  return String(value).trim();
}

function asObject(value) {
  if (!value || typeof value !== "object" || Array.isArray(value)) return null;
  return value;
}

function metadataOf(request) {
  return asObject(request && request.metadata) || {};
}

// 时长取值与网关计费一致：顶层 duration > seconds，缺省 5 秒。metadata.duration
// 不作数（计费不读它），否则会出现"生成 10 秒却按 5 秒计费"的漏计。
function outboundDuration(request) {
  const candidates = [request && request.duration, request && request.seconds];
  for (const raw of candidates) {
    if (raw === undefined || raw === null || trimmed(raw) === "") continue;
    const seconds = Number(raw);
    if (!Number.isFinite(seconds) || !Number.isInteger(seconds) || seconds <= 0) {
      throw new Error("duration must be a positive integer number of seconds");
    }
    return seconds;
  }
  return DEFAULT_DURATION;
}

// 分辨率取值与网关计费一致：顶层 size > resolution，缺省用 720P（与网关按秒表的
// 缺省档位一致，否则网关按 720p 计费、上游按自己的缺省出 1080P，等于少收钱）。
// 尺寸写法归一到档位字面量，保证上游出片档位与计费档位是同一个。
function outboundResolution(request) {
  const raw = trimmed(request && request.size) || trimmed(request && request.resolution);
  if (!raw) return DEFAULT_RESOLUTION;
  const tier = raw.match(TIER_PATTERN);
  if (tier) return tier[1] + "P";
  const sizeTier = billingTier(raw);
  if (sizeTier) return sizeTier;
  return raw;
}

// 把生效分辨率折算成 usage facts 的档位；无法归档时不写入该 fact。
function billingTier(resolution) {
  const raw = trimmed(resolution).toUpperCase();
  if (TIERS.indexOf(raw) >= 0) return raw;
  const size = raw.match(SIZE_PATTERN);
  if (!size) return "";
  const longest = Math.max(Number(size[1]), Number(size[2]));
  if (longest >= 1920) return "1080P";
  if (longest >= 1280) return "720P";
  if (longest >= 640) return "480P";
  return "";
}

// 图生视频的首帧：上游文档用 input.img_url。客户端可给 image / images[0] /
// input_reference；走 multipart 上传时文件只以占位符形式交给上游（宿主替换为 data URL）。
function firstImage(request, files) {
  const placeholders = [];
  const candidates = [request && request.image, request && request.input_reference];
  const images = request && request.images;
  if (Array.isArray(images)) {
    for (const item of images) candidates.push(item);
  }
  for (const candidate of candidates) {
    const object = asObject(candidate);
    if (object) {
      if (object.__fileRef) placeholders.push(object);
      continue;
    }
    if (trimmed(candidate)) return trimmed(candidate);
  }
  if (Array.isArray(files)) {
    for (const file of files) {
      if (!file || !trimmed(file.ref)) continue;
      if (trimmed(file.field) !== "input_reference" && trimmed(file.field) !== "image") continue;
      placeholders.push({ __fileRef: trimmed(file.ref), encoding: "dataUrl" });
    }
  }
  if (placeholders.length) return placeholders[0];
  return null;
}

function resultURL(data) {
  const body = asObject(data) || {};
  const direct = trimmed(body.result_url);
  if (direct) return direct;
  const output = asObject(asObject(body.data) && asObject(body.data).output);
  return trimmed(output && output.video_url);
}

// 查询响应可能是平台包装 {code,message,data:{...TaskDto}}，也可能是上游直出
// {output:{task_id,task_status}}；两种都取到同一层字段。
function taskPayload(envelope) {
  const body = asObject(envelope) || {};
  return asObject(body.data) || body;
}

export function buildSubmitRequest(ctx) {
  const request = ctx.requestBody || {};
  const model = trimmed(ctx.upstreamModel) || trimmed(request.model);
  if (!model) throw new Error("model is required");
  const prompt = trimmed(request.prompt);
  if (!prompt) throw new Error("prompt is required");

  // 厂商参数（negative_prompt / prompt_extend / watermark / seed / ratio / group …）
  // 经 metadata 原样透传；上面 MAPPED_KEYS 里的键不重复透传，显式字段在最后写入。
  const body = {};
  const metadata = metadataOf(request);
  for (const key of Object.keys(metadata)) {
    if (MAPPED_KEYS[key] === true) continue;
    body[key] = metadata[key];
  }
  body.model = model;
  body.prompt = request.prompt;
  body.duration = outboundDuration(request);
  const resolution = outboundResolution(request);
  if (resolution) body.resolution = resolution;
  else delete body.resolution;
  const ratio = trimmed(request.ratio);
  if (ratio) body.ratio = ratio;
  const image = firstImage(request, ctx.files);
  if (image) {
    const input = Object.assign({}, asObject(body.input));
    input.img_url = image;
    body.input = input;
  }
  const descriptor = {
    url: ctx.baseUrl + SUBMIT_PATH,
    method: "POST",
    headers: {
      "Content-Type": "application/json",
      Accept: "application/json",
      Authorization: "Bearer " + ctx.apiKey,
    },
    body: body,
  };
  const action = trimmed(ctx.action);
  if (action) descriptor.action = action;
  return descriptor;
}

export function parseSubmitResponse(ctx, resp) {
  const body = asObject(resp && resp.body);
  if (!body) throw new Error("unexpected submit response");
  const data = asObject(body.data) || {};
  const taskId = trimmed(body.task_id) || trimmed(data.task_id) || trimmed(body.id);
  if (!taskId) {
    throw new Error(trimmed(body.message) || trimmed(body.code) || "submit response has no task id");
  }
  return { taskId: taskId, taskData: body };
}

export function buildQueryRequest(ctx) {
  return {
    url: ctx.baseUrl + SUBMIT_PATH + "/" + String(ctx.taskId),
    method: "GET",
    headers: { Accept: "application/json", Authorization: "Bearer " + ctx.apiKey },
  };
}

export function parseTaskResult(ctx, body, _response) {
  const envelope = asObject(body) || {};
  const code = trimmed(envelope.code);
  // 平台自己的错误包装（如 task_not_exist）：没有任务体，直接判失败，避免白轮询 20 次
  if (code && code !== "success") {
    return { status: "FAILURE", reason: trimmed(envelope.message) || code };
  }
  const data = taskPayload(envelope);
  const raw = trimmed(data.status) || trimmed(data.task_status);
  const status = STATUS_MAP[raw.toUpperCase()];
  if (!status) {
    return { status: "UNKNOWN", reason: "unrecognized task status: " + (raw || code || "") };
  }
  const result = { status: status };
  const progress = trimmed(data.progress);
  if (progress) result.progress = progress;
  const reason = trimmed(data.fail_reason) || trimmed(envelope.message);
  if (status === "FAILURE" && reason) result.reason = reason;
  const url = resultURL(data);
  if (url) result.url = url;
  return result;
}

export function extractUsage(ctx) {
  // 视频按秒计费由网关的按秒价表负责（seconds × 分辨率档 × 错峰），插件不提供计费系数
  if (ctx.usagePurpose === "billing_ratios") return null;
  const request = ctx.requestBody || {};
  const facts = { seconds: outboundDuration(request) };
  const tier = billingTier(outboundResolution(request));
  if (tier) facts.resolution = tier;
  return facts;
}

export function listArtifacts(task) {
  if (!task || task.status !== "SUCCESS") return [];
  return resultURL(taskPayload(task.data)) ? [{ key: "video", type: "video" }] : [];
}

export function buildContentRequest(ctx) {
  if (ctx.artifactKey !== "video") throw new Error("artifact_not_found");
  const url = resultURL(taskPayload(ctx.data));
  if (!url) throw new Error("artifact_not_found");
  // 结果地址是 OSS 临时签名 URL（鉴权在查询串里），不带渠道凭证直接回源
  return { url: url, method: ctx.clientRequest.method, credentialless: true };
}
