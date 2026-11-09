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

// DeepSeek Harness agent turns through the dsh-api task interface:
// POST /api/v1/tasks (prompt, optional preset/sessionId) returns
// 202 {ok,data:{taskId,sessionId,status}}, and GET /api/v1/tasks/:taskId
// carries the status, reply, transcript and tool calls while the turn runs.
// The harness owns the LLM route, so the gateway model is a billing identity
// only and is never forwarded upstream.
export const meta = {
  apiVersion: 1,
  key: "dsh",
  name: "DSH Agent",
  icon: "text",
  description: {
    en: "DeepSeek Harness agent turns through the dsh-api task interface.",
    zh: "通过 dsh-api 任务接口调用 DeepSeek Harness 智能体。",
  },
  version: "1.3.0",
  author: { name: "CubeRouter" },
  models: ["dsh"],
  fetchMode: "per_task",
};

const API_PREFIX = "/api/v1/tasks";
const ACTION = "agent";

// 透传字段：客户端在统一任务体里给出的、dsh-api 直接支持的选项。`model` 与
// `provider` 不在其列——网关模型名不是 harness 的模型 id，转发会覆盖掉 harness
// 自身配置的模型路由。
const STRING_OPTIONS = ["preset", "cwd", "reasoningEffort", "system"];
const NUMBER_OPTIONS = ["maxTokens", "timeoutMs"];

// dsh-api 的任务接口没有上传字段：网关先把附件写进两边共享的数据卷，这里只把
// 清单拼进提示词，智能体再用自己的文件工具读取。
const MAX_ATTACHMENTS = 10;

function attachmentManifest(body) {
  const attachments = Array.isArray(body.attachments) ? body.attachments : [];
  const lines = [];
  for (const attachment of attachments) {
    if (!attachment || typeof attachment !== "object") continue;
    const path = typeof attachment.path === "string" ? attachment.path.trim() : "";
    // 只接受服务端生成的绝对路径；换行会破坏清单结构，一律丢弃。
    if (!path.startsWith("/") || /[\r\n]/.test(path)) continue;
    const name =
      typeof attachment.name === "string" && attachment.name.trim()
        ? attachment.name.trim()
        : path.split("/").pop();
    lines.push("- " + path + " (" + name + ")");
    if (lines.length >= MAX_ATTACHMENTS) break;
  }
  if (!lines.length) return "";
  return (
    "\n\n[Uploaded files]\n" +
    "The user attached these files; they are readable on disk:\n" +
    lines.join("\n")
  );
}

// dsh 自己会区分「被策略拦停/超时/截断」等结束原因。网关只有四种轮询状态，
// max-tokens 与 canceled 仍保留 reply，按成功结算（页面据 data.status 区分展示）；
// failed/timeout 没有可用回复，按失败结算并退还预扣额度。
const STATUS_MAP = {
  running: "IN_PROGRESS",
  completed: "SUCCESS",
  "max-tokens": "SUCCESS",
  canceled: "SUCCESS",
  failed: "FAILURE",
  timeout: "FAILURE",
};

function requestBase(ctx) {
  const base = String((ctx && ctx.baseUrl) || "")
    .trim()
    .replace(/\/+$/, "");
  if (!base) throw new Error("dsh channel is missing a base URL");
  return base;
}

function requestHeaders(ctx) {
  const headers = {
    "Content-Type": "application/json",
    Accept: "application/json",
  };
  // 无鉴权部署允许空密钥；有密钥时 dsh-api 接受 X-API-Key 或 Bearer。
  const apiKey = String((ctx && ctx.apiKey) || "").trim();
  if (apiKey) headers["X-API-Key"] = apiKey;
  const userId = userIdOf(ctx);
  if (userId) headers["X-DSH-User-Id"] = userId;
  return headers;
}

// 网关按任务提交者写入的稳定身份。dsh-api 用它把任务固定到 workspace/<userId>
// 并拒绝跨用户续会话；匿名或旧调用没有该字段，保持原有共享目录行为。
function userIdOf(ctx) {
  const value = ctx && ctx.userId;
  if (typeof value !== "number" || !Number.isInteger(value) || value <= 0) return "";
  return String(value);
}

function requestBodyObject(ctx) {
  const body = ctx && ctx.requestBody;
  if (!body || typeof body !== "object" || Array.isArray(body)) {
    throw new Error("request body must be a JSON object");
  }
  return body;
}

function optionalString(body, field) {
  const value = body[field];
  if (value === undefined || value === null || value === "") return undefined;
  if (typeof value !== "string") throw new Error(field + " must be a string");
  const trimmed = value.trim();
  return trimmed ? trimmed : undefined;
}

function optionalNumber(body, field) {
  const value = body[field];
  if (value === undefined || value === null || value === "") return undefined;
  const number = typeof value === "string" ? Number(value) : value;
  if (typeof number !== "number" || !Number.isFinite(number) || number <= 0) {
    throw new Error(field + " must be a positive number");
  }
  return number;
}

function sessionIdOf(data) {
  const value = data && data.sessionId;
  return typeof value === "string" && value.trim() ? value.trim() : "";
}

// dsh-api 的错误信封是 {ok:false,error:{code,message}}；HTTP 状态码兜底，
// 避免把“上游没返回可读错误”误报成插件故障。
function upstreamError(body, statusCode) {
  const error = body && typeof body === "object" && body.error && typeof body.error === "object" ? body.error : {};
  const code = String(error.code || "").trim();
  const message = String(error.message || "").trim();
  if (code && message) return "dsh " + code + ": " + message;
  if (message) return "dsh: " + message;
  if (code) return "dsh " + code;
  return "dsh returned HTTP " + String(statusCode || "unknown");
}

// 只有 {ok:true,data:{...}} 才是 dsh 的成功回执；其余一律视为上游错误。
function envelopeData(body) {
  if (!body || typeof body !== "object" || Array.isArray(body)) return undefined;
  if (body.ok !== true) return undefined;
  const data = body.data;
  if (!data || typeof data !== "object" || Array.isArray(data)) return undefined;
  return data;
}

export function buildSubmitRequest(ctx) {
  const body = requestBodyObject(ctx);
  const prompt = typeof body.prompt === "string" ? body.prompt : "";
  if (!prompt.trim()) throw new Error("prompt is required");

  const upstream = { prompt: prompt + attachmentManifest(body) };
  // dsh 只接受 server 端已存在且由本 API 创建的 sessionId；首轮不带。
  const sessionId = optionalString(body, "sessionId");
  if (sessionId) upstream.sessionId = sessionId;
  const userId = userIdOf(ctx);
  if (userId) upstream.userId = userId;
  for (const field of STRING_OPTIONS) {
    const value = optionalString(body, field);
    if (value !== undefined) upstream[field] = value;
  }
  for (const field of NUMBER_OPTIONS) {
    const value = optionalNumber(body, field);
    if (value !== undefined) upstream[field] = value;
  }

  return {
    url: requestBase(ctx) + API_PREFIX,
    method: "POST",
    headers: requestHeaders(ctx),
    body: upstream,
    action: ACTION,
  };
}

export function parseSubmitResponse(ctx, resp) {
  const response = resp || {};
  const body = response.body;
  const data = envelopeData(body);
  if (!data) throw new Error(upstreamError(body, response.statusCode));
  const taskId = typeof data.taskId === "string" ? data.taskId.trim() : "";
  if (!taskId) throw new Error("dsh did not return a task id");

  // 保留完整回执：轮询第一次覆盖 task.data 之前，页面就能从这里拿到 sessionId。
  const result = { taskId: taskId, taskData: body };
  const sessionId = sessionIdOf(data);
  if (sessionId) result.state = { sessionId: sessionId };
  return result;
}

export function buildQueryRequest(ctx) {
  const taskId = String((ctx && ctx.taskId) || "").trim();
  if (!taskId) throw new Error("dsh task id is missing");
  return {
    url: requestBase(ctx) + API_PREFIX + "/" + encodeURIComponent(taskId),
    method: "GET",
    headers: requestHeaders(ctx),
  };
}

function failureReason(data, status) {
  const error = data && data.error && typeof data.error === "object" ? data.error : {};
  const code = String(error.code || "").trim();
  const message = String(error.message || "").trim();
  if (code && message) return code + ": " + message;
  if (message) return message;
  if (code) return code;
  const turnEnd = String((data && data.turnEnd) || "").trim();
  if (turnEnd && turnEnd !== status) return "agent turn ended: " + turnEnd;
  if (status === "timeout") return "agent task timed out";
  return "dsh task ended with status " + status;
}

export function parseTaskResult(ctx, body, resp) {
  const response = resp || {};
  const data = envelopeData(body);
  // 错误信封或非 2xx 由宿主计入轮询失败；这里抛错，避免把故障当成终态结算。
  if (!data) throw new Error(upstreamError(body, response.statusCode));

  const status = String(data.status || "").trim();
  const taskId = typeof data.taskId === "string" && data.taskId.trim() ? data.taskId.trim() : String((ctx && ctx.taskId) || "");
  const result = { taskId: taskId, status: STATUS_MAP[status] || "UNKNOWN" };
  if (result.status === "FAILURE") result.reason = failureReason(data, status);

  const sessionId = sessionIdOf(data);
  if (sessionId) result.state = { sessionId: sessionId };
  return result;
}

// dsh-api 在任务结束时给出 files 摘要：本轮智能体写出并通过文件系统校验的文件。
// 只有成功任务才有产物；键由 dsh-api 归一化，可直接作为网关的 artifact key。
//
// 只收录可交付的文件：智能体为了生成 PPT 临时写下的 build_deck.py / check.py
// 属于工作文件，列在「本轮生成的文件」里会让用户误以为它们是结果。扩展名白名单
// 与 dsh-api 的快照白名单保持一致；扩展名未知时保留，避免把正常的产物藏掉。
const DELIVERABLE_EXTENSIONS = new Set([
  "pptx", "ppt", "docx", "doc", "xlsx", "xls", "pdf",
  "md", "html", "htm", "csv", "json", "txt", "svg",
  "png", "jpg", "jpeg", "gif", "webp", "mp4", "webm", "mp3", "wav", "zip",
]);

function extensionOf(name) {
  const base = String(name || "");
  const dot = base.lastIndexOf(".");
  if (dot < 0 || dot === base.length - 1) return "";
  return base.slice(dot + 1).toLowerCase();
}

function artifactType(mimeType) {
  const value = String(mimeType || "").toLowerCase();
  if (value.startsWith("video/")) return "video";
  if (value.startsWith("audio/")) return "audio";
  if (value.startsWith("image/")) return "image";
  return "file";
}

export function listArtifacts(task) {
  const current = task || {};
  if (current.status !== "SUCCESS") return [];
  const data = envelopeData(current.data);
  const items = data && Array.isArray(data.artifacts) ? data.artifacts : [];
  const artifacts = [];
  const seen = new Set();
  for (const item of items) {
    if (!item || typeof item !== "object" || Array.isArray(item)) continue;
    const key = typeof item.key === "string" ? item.key.trim() : "";
    if (!key || seen.has(key)) continue;
    const name = typeof item.name === "string" ? item.name.trim() : "";
    const extension = extensionOf(name || key);
    if (extension && !DELIVERABLE_EXTENSIONS.has(extension)) continue;
    seen.add(key);
    const artifact = { key: key, type: artifactType(item.mimeType) };
    const mimeType = typeof item.mimeType === "string" ? item.mimeType.trim() : "";
    if (mimeType) artifact.mimeType = mimeType;
    if (name) artifact.name = name;
    artifacts.push(artifact);
  }
  return artifacts;
}

// 网关按需把下载转成对 dsh-api 的一次请求：文件端点与轮询端点同前缀、同鉴权。
export function buildContentRequest(ctx) {
  const taskId = String((ctx && ctx.upstreamTaskId) || "").trim();
  const artifactKey = String((ctx && ctx.artifactKey) || "").trim();
  if (!taskId || !artifactKey) throw new Error("dsh artifact request is missing identifiers");

  // 媒体预览会用 HEAD/Range 探测，头由网关统一补齐；这里只保留密钥与归属。
  const method = String((ctx && ctx.clientRequest && ctx.clientRequest.method) || "GET").toUpperCase();
  const headers = {};
  const apiKey = String((ctx && ctx.apiKey) || "").trim();
  if (apiKey) headers["X-API-Key"] = apiKey;
  const userId = userIdOf(ctx);
  if (userId) headers["X-DSH-User-Id"] = userId;

  return {
    url: requestBase(ctx) + API_PREFIX + "/" + encodeURIComponent(taskId) + "/files/" + encodeURIComponent(artifactKey),
    method: method === "HEAD" ? "HEAD" : "GET",
    headers: headers,
  };
}
