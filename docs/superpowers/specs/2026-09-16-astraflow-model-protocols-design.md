# 星图(AstraFlow)按模型声明上游协议 — 设计规格

日期: 2026-09-16
状态: 待评审(方案在 2026-09-16 对话中确认: 人工配置 + 通配符,不做探测)

## 1. 背景与目标

星图(AstraFlow,渠道类型 59)是一个聚合型上游: 同一个渠道下的不同模型,原生支持的协议并不一致。但当前适配器对所有模型一视同仁:

- `ConvertClaudeRequest` 一律把 Anthropic 请求转成 OpenAI chat(`relay/channel/astraflow/adaptor.go:123-139`);
- `GetRequestURL` 对所有 Claude 格式请求强制打 `{base}/v1/chat/completions`(`:48-52`);
- `DoResponse` 无条件调用 chat 处理器(`:106-113`),**不看 RelayMode**——`/v1/responses` 的响应也被当成 chat 解析。

目标是让 agent 用任意协议访问 cuberouter 都能工作: **上游原生支持该协议就直连,不支持就降级转换**,把协议差异挡在网关里。客户端看到的协议与上游实际使用的协议解耦。

本仓库已有实现这套分发的先例: AdvancedCustom 适配器按 `Converter` 分发到原生直传 / `OaiChatToResponsesHandler` / `OaiResponsesToChatHandler` / claude / openai 处理器(`relay/channel/advancedcustom/adaptor.go:305-334`);relaykit 的转换器注册表里 chat↔responses、chat↔messages 双向的请求/响应/流式转换均已存在(`relaykit/relayconvert/text_converter_registry.go:116,132`)。因此本设计是**接线**,不是新增转换器。

### 实测记录(2026-09-16, `api.modelverse.cn`,仅用于说明差异真实存在)

| 模型 | chat | messages | responses |
|---|---|---|---|
| glm-5.3 | ✅ | ✅ 原生 | ✅ 原生 |
| deepseek-v4-pro-0813 | ✅ | ✅ 原生(带 thinking 块) | ✅ |
| qwen3.8-max | ✅ | ✅ | ✅ |
| kimi-k3 | ✅ | ✅ | ✅(响应 id 为 `resp_chatcmpl-…`,上游桥接) |
| gpt-5.5 | ✅ | ✅(响应 id 为 `chatcmpl-…`,上游桥接) | ✅ |
| claude-sonnet-5 | ✅ | ✅ | ❌ `500 not implemented` |
| claude-opus-5 | ✅ | ✅ | ❌ `500 not implemented` |
| gpt-image-2.5-flare | ❌ | ❌ | ❌ `400 The requested operation is unsupported` |

结论: 差异存在且可按模型区分;`/v1/messages` 的鉴权只需 `Authorization: Bearer`(单独发即可通过),`x-api-key` / `anthropic-version` 均非必需。上游 `/v1/models` 只返回 `id/created/owned_by/pricing`,**没有协议能力元数据**,故无法自动发现(这也是本方案选择人工配置的原因)。

## 2. 不变量与硬边界

- **配置缺失即零回归**: 未配置 `model_protocols`、或模型未命中任何规则时,行为与改动前完全一致。
- **chat 是兜底协议**: 任何降级都落到 chat;chat 的直连路径不做能力校验(与现有 default 分支一致)。
- **声明优先,运行时兜底**: 配置说"原生支持"就直连;上游明确拒绝(见 4.4 签名)时,进程内记住该组合并自动降级,不让同一个配置错误反复失败。
- **配置只影响 astraflow**: 本期只有渠道类型 59 读取该配置;其他渠道类型行为不变。
- **relaykit 独立可构建**: 匹配与校验逻辑放在 `relaykit/dto`(纯标准库),不引入 host 依赖;改完须 `cd relaykit && GOWORK=off go build ./...`。
- **不引入探测**: 能力表由运营按实测/文档配置,本期不做自动探测(见第 5 节)。

## 3. 现状关键点(实现依据)

- 配置管道现成: `dto.ChannelOtherSettings` 存在渠道 `settings` 列(API 字段 `settings`),运行时由 `relay/common/relay_info.go:240-242` 填入 `ChannelMeta.ChannelOtherSettings`,适配器可直接读 `info.ChannelOtherSettings`(同类字段 `claude_beta_query`、`azure_responses_version` 已如此使用)。
- 校验钩子现成: `model.Channel.ValidateSettings()`(`model/channel.go:971`)已负责 `ChannelOtherSettings` 的字段校验(先例 `ValidateToolLossPolicy`),由 `controller/channel.go` 的 `validateChannel` 在保存时调用。
- 模型模式匹配先例: `matchAdvancedCustomRouteModelRule`(`relaykit/dto/channel_settings.go:333`)是精确 + `re:` 正则,并带 regex 缓存(热路径不重复编译)。
- 上游非 2xx 的观测点: 各 handler 在 `adaptor.DoResponse` **之前**短路(`relay/claude_handler.go:140`、`relay/responses_handler.go:135`),统一走 `service.RelayErrorHandler`(该函数没有 RelayInfo,不适合挂渠道级钩子);而 `channel.DoApiRequest` 对非 2xx 返回 `(resp, nil)`(`relay/channel/api_request.go:340`),因此适配器自己的 `DoRequest` 是唯一能看到"状态码 + 错误体 + RelayInfo"的位置。
- 响应侧处理器现成: `openai.OaiResponsesHandler` / `OaiResponsesStreamHandler`(原生 responses)、`openai.OaiChatToResponsesHandler` / `OaiChatToResponsesStreamHandler`(chat 上游 → responses 客户端,`relay/channel/openai/responses_via_chat.go:19,65`)、`claude.Adaptor.DoResponse`(原生 Anthropic)。

## 4. 设计

### 4.1 配置结构

`dto.ChannelOtherSettings` 新增(键值均为字符串,无 host 依赖):

```json
{
  "model_protocols": {
    "*": ["chat"],
    "claude-*": ["chat", "messages"],
    "gpt-*": ["chat", "messages", "responses"],
    "gpt-image-*": [],
    "deepseek-*": ["chat", "messages", "responses"],
    "glm-*": ["chat", "messages", "responses"],
    "qwen-image-*": [],
    "re:^kimi-k[23]": ["chat", "messages", "responses"]
  }
}
```

- 值: `"chat"` / `"responses"` / `"messages"`;空数组 `[]` 表示"该模型没有文本协议"(图像/音频模型),此时所有文本请求按降级处理(仍打到 chat,但不会声称原生支持)。
- 键:
  - 模型名精确值——最高优先级;
  - 末尾单个 `*` 的前缀通配,如 `claude-*`(最长前缀优先,用于 `gpt-image-*` 覆盖 `gpt-*`);
  - `re:` 前缀的正则(逃生舱);
  - `*` 作为兜底。
- 匹配顺序: **精确 > 最长前缀通配 > 正则 > `*`**;都没命中则视为"未配置"。

命名取舍: 用通用的 `model_protocols` 而非 `astraflow_model_protocols`,因为语义(某模型上游原生支持哪些协议)与渠道无关,百炼等聚合渠道后续可直接复用同一份形状;本期消费方只有 astraflow,字段注释与本文档都写明这一点。

### 4.2 匹配与校验

- `ChannelOtherSettings.ResolveModelProtocols(model) ([]string, bool)`: 按 4.1 顺序返回命中的协议列表与是否命中;trim + 去重;正则编译走 `sync.Map` 缓存(对齐 AdvancedCustom 的做法)。
- `ChannelOtherSettings.ModelSupportsProtocol(model, protocol) bool`: 命中且列表包含该协议。未命中返回 false(=按现状处理)。
- `ChannelOtherSettings.ValidateModelProtocols() error`(保存时): 协议名白名单;键非空;`re:` 正则必须可编译;`*` 只允许出现在末尾且最多一个;重复值不报错但归一化。

### 4.3 请求路由决策

设 `requested` 为客户端请求的协议(`messages` / `responses`)。适配器四处按同一判断分流:

**必须区分「未命中」与「命中但不含该协议」**——两者现状基线不同(messages 的现状是被转成 chat,responses 的现状是原样直连):

| 客户端 | 声明命中且含 requested | 声明命中但不含 | 未命中/未配置 |
|---|---|---|---|
| messages | 原样直连 `{base}/v1/messages`,响应走 `claude.Adaptor.DoResponse` | Claude→chat 转换,打 `{base}/v1/chat/completions`,响应由 openai chat 处理器转回 Anthropic | 同"命中但不含"(现状) |
| responses | 原样直连 `{base}/v1/responses`,响应走 `OaiResponsesHandler` / `OaiResponsesStreamHandler` | Responses→chat 转换,打 `{base}/v1/chat/completions`,响应由 `OaiChatToResponsesHandler` / `…StreamHandler` 转回 | 同"命中且含"(现状: 原样直连) |
| chat | — | — | 原样直连 `{base}/v1/chat/completions`,不做能力校验(现状) |

判定函数 `useNativeProtocol(info, protocol)`: 未命中配置 → 返回该协议的现状基线(仅 responses 为 true);命中 → 看声明是否包含该协议,再叠加 4.4 的运行时降级记忆。配置里显式写 `"*": ["chat"]` 即对未匹配到更长规则的模型统一采用 chat 基线(此时 responses 会被降级),这是运营的显式开关。

### 4.4 运行时兜底

已声明原生直连、但上游用明确的错误文案拒绝时,不能每次请求都撞一次:

- 触发点: astraflow 的 `DoRequest` 在 `channel.DoApiRequest` 返回后,若 `resp.StatusCode >= 400`,peek 响应体前 4KB(读完用 `io.MultiReader` 还原 `resp.Body`,不影响后续 `service.RelayErrorHandler`),匹配签名。
- 签名(小写子串,可后续扩展)必须是**协议级**措辞: `not implemented`、`operation is unsupported`、`does not support`、`unsupported protocol`、`unsupported endpoint`。不收录裸 `unsupported`——否则 `unsupported parameter: temperature` 这类与协议无关的 4xx 会误触发一次性、进程级的降级。`The requested operation is unsupported.` 是实测到的措辞(星图图像模型),它不含协议词,所以不能用"签名必须与协议名共现"的规则。
- 命中则: `common.SysLog` 输出 WARN 级记录(渠道 id、模型、协议、上游原文摘要),并在进程内 `sync.Map` 记住 `channelID|model|protocol` 及记录时间;此后该组合 `useNativeProtocol` 返回 false,自动降级为转换。
- **记忆带水位**(记录时间): 只有 `RelayInfo.StartTime` 晚于记录时间的请求才看得到降级。上游往返期间由并发请求写入的记忆必须留给后续请求,否则会改掉这次正在飞行中的请求的上游路径与响应处理器(一次成功响应可能被错误解析)。`StartTime` 在任何适配器调用之前就已设好(`relay/common/relay_info.go:92`)。
- 语义边界: 该记忆是**进程内、非持久**的(多副本部署时每个副本各自自愈一次);重启后回到配置声明。不改渠道配置、不写库。
- 首次失败的请求仍然返回上游错误(信息里包含上游原文,便于运营修配置),从第二次起自动降级。

### 4.5 与端点声明表的关系(本期不改)

`common.GetEndpointTypesByChannelType` 目前对 astraflow 一律返回 `[openai, openai-response]`,与本配置的能力表并不一致。让它跟随配置需要把渠道配置喂进 pricing(照 `loadPricingAdvancedCustomConfigs` 的范式),**不在本期范围**;本期只影响实际转发行为。

## 5. 非目标 / 后续

- **不做探测**: 不实现"逐个模型打三种端点自动生成能力表"(上游无元数据,探针需要调度、超时、付费、结果落库,单独立项)。
- **不做前端表单**: 本期用渠道 `settings` JSON(API/curl)配置;`buildSettingsJSON`(`web/src/features/channels/lib/channel-form.ts:652`)从原始 JSON 出发只增删已知字段,因此 UI 保存不会覆盖本字段。可视化编辑后续补。
- **不做声明表联动**(见 4.5)。
- **不做其它渠道**: 百炼(17)等渠道的同类缺口另开 PR;本配置结构可直接复用。
- **不做 feature 级能力**(工具调用/流式/thinking 是否可用),本期只声明协议级。

## 6. 验证方式

- 单元测试: relaykit 匹配/校验表驱动测试;`model.Channel.ValidateSettings` 保存校验;astraflow 适配器按 4.3 矩阵的链路测试(沿用 `relay/channel/astraflow/relay_test.go` 的假上游结构,断言上游路径与请求体),含流式 responses 回归(现状会误判为"不完整流")与 4.4 的降级记忆。
- 集成实测(本地实例 + 真实星图 key,或按需): 用 `/home/skeeey/workspace/cuberouter/astraflow.yaml` 的 base_url/key 配一个渠道,分别以 chat/messages/responses 调同一模型,确认 `claude-*` 走 chat 桥、`glm-*` 走原生 messages。
- 构建校验: `go build ./...`、`go test ./relay/channel/astraflow/ ./model/ ./relaykit/...`、`cd relaykit && GOWORK=off go build ./...`。

## 7. 2026-09-27 补充: gemini 协议(已实现)

新增第四个协议值 `gemini`,语义是"上游原生提供 Gemini 协议(`/{version}/models/{model}:generateContent`)"。

**基线与其他三个协议不同**: gemini 的现状基线既不是直连也不是降级,而是 `ConvertGeminiRequest` 直接 `not implemented` → 500 `convert_request_failed`(gemini 格式入站没有任何兜底)。本期把它改成与 chat/messages/responses 对称的两态:

| 客户端(gemini 原生格式, action 为 `generateContent`/`streamGenerateContent`) | 上游路径 | 响应处理器 |
|---|---|---|
| 声明命中且含 `gemini` | `{base}/{version}/models/{上游模型名}:{action}`(流式补 `alt=sse`) | gemini 适配器(`GeminiTextGeneration*Handler`,与渠道类型 24 一致) |
| 未命中声明 / 未配置 | `{base}/v1/chat/completions`(请求体转 chat) | openai 处理器按 `RelayFormatGemini` 转回 gemini 形状 |

- 请求体透传开关打开时(全局或渠道级),上游收到的本来就是客户端原始 gemini 报文,此时**未声明也按 gemini 端点收发**(与 responses 的 `responsesDowngradedToChat` 同一条规则)。透传时也**不记降级记忆**: 报文形状改不了,记了不会生效,只会在日志里误报"已降级"。
- URL 一律重建,客户端查询串不转发上游: gemini 客户端的 `?key=<网关令牌>` 属于网关鉴权,原样透传会把网关令牌送给上游;action 分隔符被转义成 `%3A` 时先解码再取 action(路由与请求校验用的都是解码后的路径)。
- `action` 取自客户端路径,**只有文本生成两种 action 参与分流**;`embedContent` / `batchEmbedContents` / `countTokens` 等非文本 action 一律保持改动前的行为(沿用客户端路径 + openai 处理器,请求转换快速失败——两条路本来都不通,不能把转换后的 chat 报文发到客户端的 gemini action 路径上)。
- 原生 gemini 流式会把 `info.DisablePing` 置为 true(与渠道类型 24 同一位置): gemini 客户端逐条 JSON 解析 SSE 事件,网关注入的 `: PING` 注释会让它解析失败。
- `sentProtocol` 对 gemini 会话返回 `gemini`,4.4 的降级记忆因此覆盖它(继承既有签名列表的暴露面: 参数级 4xx 若含 `does not support` 等同措辞也会被记忆——与 messages/responses 相同)。

实现侧顺带的两处共享修正(不在 astraflow 内,但由本特性暴露):

- `relay/channel/openai/helper.go` 的 `processTokenData` 原先只认 `RelayModeChatCompletions`/`RelayModeCompletions`,降级会话(gemini/messages → chat)的文本统计因此为空——上游若不回 usage,结算会拿空文本估算,completion 记 0。现改为"非 completions 的 relayMode 都按 chat 分块统计",解析失败只在 chat 会话里算错误。实测星图会回 usage(`include_usage` 生效),该缺口对它是潜伏的,对忽略该参数的网关是实际的。
- `responsesDowngradedToChat` 的两处透传判断与新代码共用 `bodyPassthroughEnabled`,行为不变(既有用例覆盖)。

实测(`gemini-3.8-flash`,2026-09-27):gemini 原生端点 `Bearer` 与 `x-goog-api-key` 均可用;非流式与 `:streamGenerateContent?alt=sse` 都返回标准 Gemini 形状,`usageMetadata` 完整(prompt/candidates/thoughts/total),网关按 `billing-usage-gemini` 计费,消费日志 token 与上游一致;同一上游的 `/v1/messages`(400)与 `/v1/responses`(500 `not implemented`)均不可用,故声明为 `[chat, gemini]`。

**未收录的拒绝签名**: 星图 gemini 端点会整体抖动——同一模型同一请求几分钟内 200/400 交替,期间所有模型一起返回 400 `Invalid param: get request mode`(实测触发过一次约 2 分钟的窗口)。这是上游瞬时故障,不是"该模型没有 gemini 协议",因此**不写进 `protocolUnsupportedMarkers`**: 否则一次抖动就会把已声明的原生 gemini 在进程内永久降级成有损 chat 转换。回归测试 `TestAstraflowKeepsNativeGeminiOnTransientRequestModeError` 锁住这条边界。

