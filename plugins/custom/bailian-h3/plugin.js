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
  key: "bailian-h3",
  name: "Bailian MiniMax H3",
  icon: "Minimax.Color",
  description: {
    en: "MiniMax-H3 video generation on Alibaba Cloud Bailian (text-to-video, first/last-frame image-to-video, multimodal reference-to-video).",
    zh: "阿里云百炼 MiniMax-H3 视频生成（文生视频、首尾帧图生视频、多模态参考生视频）。",
  },
  version: "1.0.1",
  author: { name: "Wei Liu" },
  models: ["minimax-h3-a"],
  fetchMode: "per_task",
  usageSchema: {
    seconds: {
      type: "number",
      unit: "second",
      description: {
        en: "Upstream billed seconds: output seconds plus reference-video input seconds.",
        zh: "上游计费总秒数（输出秒数 + 参考视频输入秒数）。",
      },
    },
    resolution: {
      enum: ["768P", "2K"],
      description: {
        en: "Output resolution tier billed by upstream.",
        zh: "计费命中的输出分辨率档位。",
      },
    },
    input_images: {
      type: "number",
      unit: "count",
      description: {
        en: "Billed input image count: the first five input images are free, this is the excess.",
        zh: "计费输入图片数量（输入图片 5 张以内免费，这里是超出部分）。",
      },
    },
  },
  usageExamples: [
    { label: "768P · 5s", facts: { seconds: 5, resolution: "768P", input_images: 0 } },
    { label: "2K · 5s", facts: { seconds: 5, resolution: "2K", input_images: 0 } },
    { label: "2K · 5s · 7 images", facts: { seconds: 5, resolution: "2K", input_images: 2 } },
    { label: "768P · 5s + 15s reference video", facts: { seconds: 20, resolution: "768P", input_images: 0 } },
  ],
};

// 上游 = 阿里云百炼 MaaS 工作空间（{WorkspaceId}.cn-beijing.maas.aliyuncs.com）的
// DashScope 视频合成接口：提交 POST /api/v1/services/aigc/video-generation/video-synthesis
// （必须带 X-DashScope-Async: enable），查询 GET /api/v1/tasks/{task_id}。
// 参考文档：https://docs.bailian.console.aliyun.com/zh/model-studio/minimax-video-generation-api-reference
const SUBMIT_PATH = "/api/v1/services/aigc/video-generation/video-synthesis";
const TASKS_PATH = "/api/v1/tasks/";

// 参数口径全部来自上游文档：duration 为 4~15 秒整数、resolution 只出 768P/2K、
// 文生视频必须显式指定 ratio 且不能为 adaptive。插件在这里补齐缺省值并把不合法的
// 值挡在预扣费之前，保证"发给上游的参数"与"用于计费的 usage facts"是同一份归一结果。
const MIN_DURATION = 4;
const MAX_DURATION = 15;
const DEFAULT_DURATION = 5;
// 缺省走最便宜的 768P，避免客户端没写分辨率时被按 2K 档计费
const DEFAULT_RESOLUTION = "768P";
// 文生视频必填比例，缺省补 16:9；参考场景缺省 adaptive（上游默认，省略即可）
const DEFAULT_RATIO = "16:9";
const RATIOS = ["16:9", "4:3", "1:1", "3:4", "9:16", "21:9"];

// media 各类型的数量上限（上游文档：首帧/尾帧各 1 张；参考图 9、参考视频 3、参考音频 3）
const MEDIA_LIMITS = {
  first_frame: 1,
  last_frame: 1,
  image_url: 9,
  feature: 3,
  driving_audio: 3,
};
// 计入计费图片数量的类型（上游 usage.image_count 的口径：输入图 5 张以内免费）
const IMAGE_MEDIA_TYPES = ["first_frame", "last_frame", "image_url"];
const FREE_INPUT_IMAGES = 5;
// 图生视频与多模态参考互斥（上游约束）：首/尾帧组合与 image_url/feature/driving_audio 不能混用
const FRAME_MEDIA_TYPES = ["first_frame", "last_frame"];
const REFERENCE_MEDIA_TYPES = ["image_url", "feature", "driving_audio"];

function trimmed(value) {
  if (value === undefined || value === null) return "";
  return String(value).trim();
}

function asObject(value) {
  if (!value || typeof value !== "object" || Array.isArray(value)) return null;
  return value;
}

// 时长取值与计费口径一致：顶层 duration > seconds，缺省 5 秒；越界直接拒绝
// （上游 4~15 秒，超出会在异步阶段失败，不如在预扣费之前挡下）。
function outboundDuration(request) {
  const candidates = [request && request.duration, request && request.seconds];
  for (const raw of candidates) {
    if (raw === undefined || raw === null || trimmed(raw) === "") continue;
    const seconds = Number(raw);
    if (!Number.isInteger(seconds) || seconds < MIN_DURATION || seconds > MAX_DURATION) {
      throw new Error("duration must be an integer between " + MIN_DURATION + " and " + MAX_DURATION + " seconds");
    }
    return seconds;
  }
  return DEFAULT_DURATION;
}

// 分辨率归一：字面量 768P/2K 直接命中；"720p"/"1080p" 这类档位与 "1344x768" 这类
// 尺寸按短边归档（≥1440 → 2K，≥640 → 768P），比 640 更小或无法识别则拒绝——
// 绝不让"上游出的档位"与"计费的档位"出现分歧。
function outboundResolution(request) {
  const raw = trimmed(request && request.size) || trimmed(request && request.resolution);
  if (!raw) return DEFAULT_RESOLUTION;
  const normalized = raw.toUpperCase();
  if (normalized === "768P" || normalized === "2K") return normalized;
  const tier = normalized.match(/^(\d{3,4})P$/);
  if (tier) {
    const shortSide = Number(tier[1]);
    if (shortSide >= 1440) return "2K";
    if (shortSide >= 640) return "768P";
  }
  const size = normalized.match(/^(\d{2,5})\s*[*X×]\s*(\d{2,5})$/);
  if (size) {
    const shortSide = Math.min(Number(size[1]), Number(size[2]));
    if (shortSide >= 1440) return "2K";
    if (shortSide >= 640) return "768P";
  }
  throw new Error("unsupported resolution " + raw + "; MiniMax-H3 supports 768P or 2K");
}

// 素材来源：metadata.media 是完整直通口（参考视频/参考音频只有这里能传），未提供时按
// image/images/input_reference 的图数推导：1 张 = 首帧，2 张 = 首尾帧，3 张及以上 =
// 参考图（多模态参考生视频，上游最多 9 张）。multipart 上传的文件（image /
// input_reference 字段）只在没有 URL 素材时使用：宿主把占位符内联成 data URL 后发给
// 上游（上游文档要求 HTTP(S) URL，data URL 是否可接受由上游裁决）。
function mediaSources(request, files) {
  const sources = [];
  const candidates = [trimmed(request && request.image)];
  if (Array.isArray(request && request.images)) {
    for (const image of request.images) candidates.push(trimmed(image));
  }
  candidates.push(trimmed(request && request.input_reference));
  for (const candidate of candidates) {
    if (candidate && sources.indexOf(candidate) < 0) sources.push(candidate);
  }
  if (sources.length === 0) {
    for (const file of files || []) {
      const field = trimmed(file && file.field);
      const ref = trimmed(file && file.ref);
      if (!ref || (field !== "image" && field !== "input_reference")) continue;
      sources.push({ __fileRef: ref, encoding: "dataUrl" });
    }
  }
  return sources;
}

function isMediaUrl(value) {
  if (typeof value === "string") return trimmed(value) !== "";
  return !!asObject(value) && typeof value.__fileRef === "string";
}

function buildMedia(request, files) {
  const metadata = asObject(request && request.metadata) || {};
  const items = [];
  const declared = metadata.media;
  if (declared !== undefined) {
    if (!Array.isArray(declared) || declared.length === 0) {
      throw new Error("metadata.media must be a non-empty array");
    }
    for (const raw of declared) {
      const item = asObject(raw);
      const type = trimmed(item && item.type);
      const url = item && item.url;
      if (!item || !MEDIA_LIMITS[type] || typeof url !== "string" || !trimmed(url)) {
        throw new Error("metadata.media entries must be {type, url} with type one of " + Object.keys(MEDIA_LIMITS).join(", "));
      }
      items.push({ type: type, url: trimmed(url) });
    }
  } else {
    const sources = mediaSources(request, files);
    if (sources.length === 1) {
      items.push({ type: "first_frame", url: sources[0] });
    } else if (sources.length === 2) {
      items.push({ type: "first_frame", url: sources[0] });
      items.push({ type: "last_frame", url: sources[1] });
    } else {
      for (const source of sources) items.push({ type: "image_url", url: source });
    }
  }
  validateMedia(items);
  return items;
}

function validateMedia(items) {
  const counts = {};
  for (const item of items) {
    if (!isMediaUrl(item.url)) {
      throw new Error(item.type + " media requires a url");
    }
    counts[item.type] = (counts[item.type] || 0) + 1;
    if (counts[item.type] > MEDIA_LIMITS[item.type]) {
      throw new Error("too many " + item.type + " media entries (max " + MEDIA_LIMITS[item.type] + ")");
    }
  }
  const hasFrames = FRAME_MEDIA_TYPES.some(function (type) {
    return counts[type] > 0;
  });
  const hasReferences = REFERENCE_MEDIA_TYPES.some(function (type) {
    return counts[type] > 0;
  });
  if (hasFrames && hasReferences) {
    throw new Error("first_frame/last_frame cannot be mixed with image_url/feature/driving_audio media");
  }
}

// 计费图片数量：与上游 usage.image_count 同一口径（超出 5 张免费额度的部分）
function billedInputImages(items) {
  let count = 0;
  for (const item of items) {
    if (IMAGE_MEDIA_TYPES.indexOf(item.type) >= 0) count++;
  }
  return Math.max(0, count - FREE_INPUT_IMAGES);
}

// 比例规则：首/尾帧场景宽高比由输入图片决定（上游恒 adaptive，不传）；
// 参考场景可选（缺省 adaptive）；文生视频必填且不能 adaptive。
function outboundRatio(request, media) {
  const hasFrames = media.some(function (item) {
    return FRAME_MEDIA_TYPES.indexOf(item.type) >= 0;
  });
  if (hasFrames) return "";
  const hasReferences = media.length > 0;
  const raw = trimmed(request && request.ratio);
  if (!raw) return hasReferences ? "" : DEFAULT_RATIO;
  if (raw === "adaptive") {
    if (!hasReferences) {
      throw new Error("ratio must not be adaptive for text-to-video; pick one of " + RATIOS.join(", "));
    }
    return "";
  }
  if (RATIOS.indexOf(raw) < 0) {
    throw new Error("ratio must be one of " + RATIOS.join(", "));
  }
  return raw;
}

// 请求归一结果：submit 请求体与 submit 期计费 facts 都从这一份推导，避免两处口径分叉
function plan(request, files) {
  const prompt = trimmed(request && request.prompt);
  if (!prompt) throw new Error("prompt is required");
  const media = buildMedia(request, files);
  const hasFrames = media.some(function (item) {
    return FRAME_MEDIA_TYPES.indexOf(item.type) >= 0;
  });
  return {
    prompt: prompt,
    media: media,
    duration: outboundDuration(request),
    resolution: outboundResolution(request),
    ratio: outboundRatio(request, media),
    inputImages: billedInputImages(media),
    action: media.length === 0 ? "text_to_video" : hasFrames ? "image_to_video" : "reference_to_video",
  };
}

export function buildSubmitRequest(ctx) {
  const request = ctx.requestBody || {};
  const model = trimmed(ctx.upstreamModel) || trimmed(request.model);
  if (!model) throw new Error("model is required");
  const planned = plan(request, ctx.files);
  const parameters = { resolution: planned.resolution, duration: planned.duration };
  if (planned.ratio) parameters.ratio = planned.ratio;
  const metadata = asObject(request.metadata) || {};
  if (metadata.watermark === true) parameters.watermark = true;
  const input = { prompt: planned.prompt };
  if (planned.media.length) input.media = planned.media;
  return {
    url: ctx.baseUrl + SUBMIT_PATH,
    method: "POST",
    headers: {
      "Content-Type": "application/json",
      Accept: "application/json",
      Authorization: "Bearer " + ctx.apiKey,
      "X-DashScope-Async": "enable",
    },
    body: { model: model, input: input, parameters: parameters },
    action: planned.action,
  };
}

export function parseSubmitResponse(ctx, resp) {
  const body = asObject(resp && resp.body);
  if (!body) throw new Error("unexpected submit response");
  const output = asObject(body.output) || {};
  const taskId = trimmed(output.task_id);
  if (taskId) return { taskId: taskId, taskData: body };
  const code = trimmed(body.code) || trimmed(output.code);
  const message = trimmed(body.message) || trimmed(output.message);
  const detail = [code, message].filter(function (part) {
    return part !== "";
  });
  throw new Error(detail.length ? detail.join(": ") : "submit response has no task id");
}

export function buildQueryRequest(ctx) {
  return {
    url: ctx.baseUrl + TASKS_PATH + encodeURIComponent(ctx.taskId),
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
  if (status === "SUCCEEDED") {
    const url = trimmed(output.video_url);
    if (!url) return { status: "FAILURE", reason: "task succeeded without a video url" };
    return { status: "SUCCESS", url: url };
  }
  if (status === "FAILED" || status === "CANCELED" || status === "UNKNOWN") {
    // UNKNOWN = task_id 不存在或已过 24 小时有效期，按失败终态收敛，避免无限轮询
    const code = trimmed(output.code) || trimmed(envelope.code);
    const message = trimmed(envelope.message) || trimmed(output.message);
    let reason = message || code;
    if (code && message && message !== code) reason = code + ": " + message;
    if (status === "UNKNOWN" && !reason) reason = "task not found or expired";
    return { status: "FAILURE", reason: reason || "task failed" };
  }
  return { status: "UNKNOWN", reason: "unrecognized task status: " + trimmed(output.task_status) };
}

export function extractUsage(ctx) {
  // 计费走模型级 tiered_expr（网关按插件 usage facts 求值），不提供 billing_ratios 系数
  if (ctx.usagePurpose === "billing_ratios") return null;
  const request = ctx.requestBody || {};
  const planned = plan(request, ctx.files);
  return {
    seconds: planned.duration,
    resolution: planned.resolution,
    input_images: planned.inputImages,
  };
}

// 完成态 facts：秒数取上游计费口径（usage.duration = input_seconds + output_seconds，
// 参考视频的输入秒数同样计费）；图片数取 usage.image_count（已是超出免费额度的部分）。
// resolution 不覆盖——上游只接受 768P/2K，提交时的估算值即实际档位。
export function extractUsageOnComplete(task, taskResult, body) {
  const envelope = asObject(body) || {};
  const usage = asObject(envelope.usage) || {};
  const facts = {};
  const seconds = completionSeconds(usage);
  if (seconds !== null && seconds > 0) facts.seconds = Math.min(seconds, 3600);
  const images = usageNumber(usage.image_count);
  if (images !== null) facts.input_images = Math.min(Math.max(Math.floor(images), 0), 128);
  return facts;
}

function completionSeconds(usage) {
  const direct = usageNumber(usage.duration);
  if (direct !== null) return direct;
  const total = usageNumber(usage.total_seconds);
  if (total !== null) return total;
  const input = usageNumber(usage.input_seconds);
  const output = usageNumber(usage.output_seconds);
  if (input !== null && output !== null) return input + output;
  return null;
}

function usageNumber(value) {
  if (typeof value !== "number" || !Number.isFinite(value) || value < 0) return null;
  return value;
}

// 查询响应可能是上游直出 {output:{...}}，也可能是平台包装 {code, data:{output}}；
// 结果地址与状态都取到同一层字段。
function taskOutput(value) {
  const body = asObject(value) || {};
  const inner = asObject(body.data) || body;
  return asObject(inner.output) || inner;
}

export function listArtifacts(task) {
  if (!task || task.status !== "SUCCESS") return [];
  const output = taskOutput(task.data);
  return trimmed(output.video_url) ? [{ key: "video", type: "video" }] : [];
}

export function buildContentRequest(ctx) {
  if (ctx.artifactKey !== "video") throw new Error("artifact_not_found");
  const url = trimmed(taskOutput(ctx.data).video_url);
  if (!url) throw new Error("artifact_not_found");
  // 结果地址是 OSS 临时签名 URL（鉴权在查询串里），不带渠道凭证直接回源
  return { url: url, method: ctx.clientRequest.method, credentialless: true };
}
