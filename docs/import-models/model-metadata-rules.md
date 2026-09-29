# Model Metadata Rules

> 管理员在站点 yaml（`*.models.yaml`，如 `cn.models.yaml`）中提供 `api-key`（root PAT）与 `model-api-key`，元数据通过 REST API 配置

1. 说明信息使用中文
2. 图标参照 [model-icons.md](./model-icons.md)
3. 选择模型供应商（如果模型是 kimi-k3-dao，之类，按模型前缀决定）

   | 前缀 | 厂商 |
   |---|---|
   | `claude-*` | Anthropic |
   | `gpt-*` | OpenAI |
   | `deepseek-*` | DeepSeek |
   | `glm-*` | 智谱 |
   | `kimi-*` | Moonshot |
   | `qwen*` | 阿里巴巴 |
   | `vidu*` | Vidu |

   前缀不在表内时，先在此表新增映射再配置，不要临时造厂商。

4. 标签参照 [model-tags.md](./model-tags.md)
5. 匹配规则 使用 完全匹配

   变体型号（如 kimi-k3-a/b/j）不会继承主型号的元数据，各自需要独立记录。

6. 端点通过分析模型上游与使用管理员提供的 model-api-key 测试后配置

   **视频/图像/音频类模型例外：不探测、不显式配置 endpoints 字段**（留空走渠道类型自动推导），避免付费探测（视频生成按秒/次真扣费）。

   探测要点：
   - `max_tokens=1`，最小化花费
   - 个别模型 max_tokens 过小会 400，按错误文案判定：`not implemented` / `operation is unsupported` / `does not support [xxx] protocol` 才是协议不支持；`Unsupported parameter` 是参数问题，换大值重试（gpt 系列要 `>=512`）
   - 推理模型给小 max_tokens 会全花在 reasoning 上、`content` 为空——要看内容就把上限开到 200 左右
   - 首次调用可能 40s+，探针要重试 + 足够超时

   **HTTP 200 不等于可直连**（桥接型上游的典型坑，2026-09-21 星图 glm-5.3）：

   探针不能只看状态码，还要看响应体形状，并在消费日志 `/api/log/` 确认 `prompt_tokens`/`completion_tokens` 非 0。
   实测星图 glm-5.3 的非流式 `/v1/responses` 返回的是 SSE 信封 `{"type":"response.completed","response":{…}}` 而非标准 Response 对象：网关按形状透传，客户端解析不了，且 **usage 取不到、计费为 0**（日志标 `usage_billing_path: upstream` 但 token 全 0）。流式是标准事件序列，正常。
   处置：把该协议从渠道 `settings.model_protocols` 里去掉，让它走网关的降级转换（转换后非流式/流式都是标准形状且计费正常）；同渠道同类模型（如 `glm-5.2`）的声明可直接参考。
   注：`endpoints` 字段照实声明模型能力即可（与渠道协议声明是两回事，见渠道 `model_protocols`）。

   **endpoints 必须是对象格式**，不能是数组：
   `{"openai": {"path": "/v1/chat/completions", "method": "POST"}, "openai-response": {...}, "anthropic": {...}}`
   （数组格式的坑：模型列表能正常显示，但编辑抽屉的行模式显示"未配置端点"，且定价缓存合并时静默忽略——`model/pricing.go` 按 `map[string]interface{}` 解析。模板见编辑抽屉的 ENDPOINT_TEMPLATES 下拉。）

7. 不动定价配置
8. 默认选择启用模型
9. 官方同步始终选择关闭

   即不使用官方同步功能，所有模型 `sync_official=0`。注意该开关只挡"覆盖已有记录"，挡不住同步的"新建"路径——若误跑同步，缺失模型仍会被自动创建为英文描述 + 上游 tag 的记录，违反第 1/4 条。
