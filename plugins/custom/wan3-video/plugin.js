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
  key: "wan3-video",
  name: "Bailian Wan 3.0 Video",
  icon: "Qwen.Color",
  description: {
    en: "Wan 3.0 video generation (standard and prime modes) on an Alibaba Cloud Bailian MaaS workspace (text-to-video, first/last-frame image-to-video, multimodal reference-to-video).",
    zh: "阿里云百炼工作空间的万相 3.0 视频生成（标准版与 Prime 模式；文生视频、首尾帧图生视频、多模态参考生视频）。",
  },
  version: "1.0.2",
  author: { name: "Wei Liu" },
  models: ["wan3.0-video-A", "wan3.0-video-prime-A"],
  fetchMode: "per_task",
  usageSchema: {
    seconds: {
      type: "number",
      unit: "second",
      description: {
        en: "Requested output video duration in seconds.",
        zh: "请求的输出视频时长（秒）。",
      },
    },
    // 键名刻意避开请求体字段 resolution：请求校验会递归检查同名键，客户端传
    // 小写 "480p" 时命中 enum 校验会被误拒。
    resolution_tier: {
      enum: ["480P", "720P", "1080P"],
      description: {
        en: "Requested output resolution tier.",
        zh: "请求的输出分辨率档位。",
      },
    },
  },
  usageExamples: [
    { label: "720P · 5s", facts: { seconds: 5, resolution_tier: "720P" } },
    { label: "1080P · 10s", facts: { seconds: 10, resolution_tier: "1080P" } },
  ],
};

// 上游 = 阿里云百炼 MaaS 工作空间（{WorkspaceId}.cn-beijing.maas.aliyuncs.com）的
// DashScope 视频合成接口：提交 POST /api/v1/services/aigc/video-generation/video-synthesis
// （必须带 X-DashScope-Async: enable），查询 GET /api/v1/tasks/{task_id}。
// 文档：https://help.aliyun.com/zh/model-studio/wan3-video-generation-guide
const SUBMIT_PATH = "/api/v1/services/aigc/video-generation/video-synthesis";
const TASKS_PATH = "/api/v1/tasks/";

// 参数口径来自上游文档：duration 2~30 秒整数（-1 为智能时长，计费不可表达，拒绝）、
// resolution 只出 480P/720P/1080P、ratio 六种固定比例或 adaptive。
// 缺省分辨率显式下发 720P：上游缺省 1080P，而站点按秒表的缺省档是 720P，
// 不下发会按 720p 收费却出 1080p 片。
const MIN_DURATION = 2;
const MAX_DURATION = 30;
const DEFAULT_DURATION = 5;
const DEFAULT_RESOLUTION = "720P";
const RESOLUTIONS = ["480P", "720P", "1080P"];
const RATIOS = ["21:9", "16:9", "4:3", "1:1", "3:4", "9:16"];

// media 类型与数量上限（上游文档）：首帧/尾帧各 1；参考图 10、参考视频 5、
// 参考音频 5、文件/网页各 1；单次多模态素材合计最多 20 个。
const MEDIA_LIMITS = {
  first_frame: 1,
  last_frame: 1,
  reference_image: 10,
  reference_video: 5,
  reference_audio: 5,
  file: 1,
  link: 1,
};
const MAX_MEDIA = 20;
// 首/尾帧模式与多模态参考互斥（上游约束）。
const FRAME_MEDIA_TYPES = ["first_frame", "last_frame"];
const REFERENCE_MEDIA_TYPES = ["reference_image", "reference_video", "reference_audio", "file", "link"];

function trimmed(value) {
  if (value === undefined || value === null) return "";
  return String(value).trim();
}

function asObject(value) {
  if (!value || typeof value !== "object" || Array.isArray(value)) return null;
  return value;
}

// 时长取值与计费口径一致（ComputeVideoPriceRatiosForTaskSubmit 读 metadata.duration /
// 顶层 duration / seconds）：metadata > duration > seconds > 缺省 5 秒。
// 越界在预扣费之前拒绝，避免上游异步失败后走退款。
function outboundDuration(request, metadata) {
  const candidates = [metadata.duration, request.duration, request.seconds];
  for (const raw of candidates) {
    if (raw === undefined || raw === null || trimmed(raw) === "") continue;
    const seconds = Number(raw);
    if (!Number.isInteger(seconds) || seconds < MIN_DURATION || seconds > MAX_DURATION) {
      throw new Error(
        "duration must be an integer between " + MIN_DURATION + " and " + MAX_DURATION + " seconds",
      );
    }
    return seconds;
  }
  return DEFAULT_DURATION;
}

// 分辨率归一：档位字面量直接命中；尺寸写法（832x480 / 1920*1080）按长边归档，
// 与站点视频按秒表的档位归一规则一致。无法识别直接拒绝——绝不让"上游出的档位"
// 与"计费的档位"出现分歧。
function outboundResolution(request, metadata) {
  const raw = trimmed(metadata.resolution) || trimmed(request.resolution) || trimmed(request.size);
  if (!raw) return DEFAULT_RESOLUTION;
  const normalized = raw.toUpperCase();
  if (RESOLUTIONS.indexOf(normalized) >= 0) return normalized;
  const tier = normalized.match(/^(\d{3,4})P$/);
  if (tier) {
    const longSide = Number(tier[1]);
    if (longSide >= 1920) return "1080P";
    if (longSide >= 1280) return "720P";
    if (longSide >= 640) return "480P";
  }
  const size = normalized.match(/^(\d{2,5})\s*[*X×]\s*(\d{2,5})$/);
  if (size) {
    const longSide = Math.max(Number(size[1]), Number(size[2]));
    if (longSide >= 1920) return "1080P";
    if (longSide >= 1280) return "720P";
    if (longSide >= 640) return "480P";
  }
  throw new Error("unsupported resolution " + raw + "; Wan 3.0 supports 480P, 720P or 1080P");
}

function mediaURL(value) {
  if (typeof value === "string") return trimmed(value);
  const object = asObject(value);
  return object ? trimmed(object.url) : "";
}

// content 数组：角色标注优先（first_frame / last_frame / reference_image），
// 视频/音频项按 reference_video / reference_audio 归类；无角色图片稍后按图数推导。
function contentMedia(request) {
  const content = Array.isArray(request.content) ? request.content : [];
  const media = [];
  for (const raw of content) {
    const item = asObject(raw);
    if (!item) continue;
    const type = trimmed(item.type);
    if (type === "image_url") {
      const url = mediaURL(item.image_url);
      if (!url) continue;
      const role = trimmed(item.role);
      const mediaType =
        role === "first_frame" || role === "last_frame" || role === "reference_image" ? role : "";
      media.push({ type: mediaType, url: url, image: true });
    } else if (type === "video_url") {
      const url = mediaURL(item.video_url);
      if (url) media.push({ type: "reference_video", url: url });
    } else if (type === "audio_url") {
      const url = mediaURL(item.audio_url);
      if (url) media.push({ type: "reference_audio", url: url });
    }
  }
  return media;
}

// 素材来源：metadata.media 是完整直通口（file/link 只能由此下发）；否则按
// content 角色标注，content 缺省时退回 images/image 的图数推导：
// 1 张 = 首帧、2 张 = 首尾帧、3 张及以上 = 参考图（与网关 Go 适配器一致）。
function buildMedia(request, metadata) {
  const declared = metadata.media;
  if (declared !== undefined) {
    if (!Array.isArray(declared) || declared.length === 0) {
      throw new Error("metadata.media must be a non-empty array");
    }
    const items = [];
    for (const raw of declared) {
      const item = asObject(raw);
      const type = trimmed(item && item.type);
      const url = trimmed(item && item.url);
      if (!item || !MEDIA_LIMITS[type] || !url) {
        throw new Error(
          "metadata.media entries must be {type, url} with type one of " +
            Object.keys(MEDIA_LIMITS).join(", "),
        );
      }
      items.push({ type: type, url: url });
    }
    validateMedia(items);
    return items;
  }

  let candidates = contentMedia(request);
  if (candidates.length === 0) {
    const sources = [];
    const single = trimmed(request.image);
    if (single) sources.push(single);
    if (Array.isArray(request.images)) {
      for (const image of request.images) {
        const url = trimmed(image);
        if (url) sources.push(url);
      }
    }
    candidates = sources.map(function (url) {
      return { type: "", url: url, image: true };
    });
  }

  const imageTotal = candidates.filter(function (item) {
    return item.image;
  }).length;
  let imageOrdinal = 0;
  const media = [];
  for (const candidate of candidates) {
    let type = candidate.type;
    if (candidate.image) {
      if (!type) {
        if (imageTotal <= 1) type = "first_frame";
        else if (imageTotal === 2) type = imageOrdinal === 0 ? "first_frame" : "last_frame";
        else type = "reference_image";
      }
      imageOrdinal++;
    }
    media.push({ type: type, url: candidate.url });
  }
  validateMedia(media);
  return media;
}

function validateMedia(items) {
  const counts = {};
  for (const item of items) {
    if (!item.url) throw new Error(item.type + " media requires a url");
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
    throw new Error("first_frame/last_frame cannot be mixed with reference_* media");
  }
  let total = 0;
  for (const type of Object.keys(counts)) total += counts[type];
  if (total > MAX_MEDIA) {
    throw new Error("too many media entries (max " + MAX_MEDIA + ")");
  }
}

// 比例规则：首尾帧场景宽高比由输入图决定（缺省 adaptive）；参考场景可选；
// 文生视频不支持 adaptive，缺省交给上游（16:9）。
function outboundRatio(request, metadata, media) {
  const hasFrames = media.some(function (item) {
    return FRAME_MEDIA_TYPES.indexOf(item.type) >= 0;
  });
  const raw = trimmed(metadata.ratio) || trimmed(request.ratio);
  if (!raw) return hasFrames ? "adaptive" : "";
  if (raw === "adaptive") {
    if (!hasFrames && media.length === 0) {
      throw new Error("ratio must not be adaptive for text-to-video; pick one of " + RATIOS.join(", "));
    }
    return raw;
  }
  if (RATIOS.indexOf(raw) < 0) {
    throw new Error("ratio must be one of " + RATIOS.join(", ") + " or adaptive");
  }
  return raw;
}

// 请求归一结果：submit 请求体与 usage facts 都从这一份推导，避免两处口径分叉。
function plan(request) {
  const prompt = trimmed(request.prompt);
  if (!prompt) throw new Error("prompt is required");
  const metadata = asObject(request.metadata) || {};
  const media = buildMedia(request, metadata);
  const hasFrames = media.some(function (item) {
    return FRAME_MEDIA_TYPES.indexOf(item.type) >= 0;
  });
  return {
    prompt: prompt,
    media: media,
    duration: outboundDuration(request, metadata),
    resolution: outboundResolution(request, metadata),
    ratio: outboundRatio(request, metadata, media),
    action: media.length === 0 ? "text_to_video" : hasFrames ? "image_to_video" : "reference_to_video",
  };
}

export function buildSubmitRequest(ctx) {
  const request = ctx.requestBody || {};
  const model = trimmed(ctx.upstreamModel) || trimmed(request.model);
  if (!model) throw new Error("model is required");
  const planned = plan(request);
  const input = { prompt: planned.prompt };
  if (planned.media.length) input.media = planned.media;
  const parameters = { resolution: planned.resolution, duration: planned.duration };
  if (planned.ratio) parameters.ratio = planned.ratio;
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
    // UNKNOWN = task_id 不存在或已过有效期，按失败终态收敛，避免无限轮询
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
  // 计费走站点 VideoPrice 按秒表（table-wins），不提供 billing_ratios 系数。
  if (ctx.usagePurpose === "billing_ratios") return null;
  const planned = plan(ctx.requestBody || {});
  return { seconds: planned.duration, resolution_tier: planned.resolution };
}

// 完成态 facts：上游 usage.duration 是实际计费秒数；usage.SR 是实际输出档位。
// 按秒表计费不依赖它，但保留真实回显便于排查。
export function extractUsageOnComplete(task, taskResult, body) {
  const envelope = asObject(body) || {};
  const usage = asObject(envelope.usage) || {};
  const facts = {};
  const seconds = Number(usage.duration);
  if (Number.isFinite(seconds) && seconds > 0) facts.seconds = Math.min(seconds, 3600);
  const sr = Number(usage.SR);
  if (Number.isInteger(sr) && [480, 720, 1080].indexOf(sr) >= 0) facts.resolution_tier = sr + "P";
  return facts;
}

// 查询响应可能是上游直出 {output:{...}}，也可能是平台包装 {code, data:{output}}。
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
