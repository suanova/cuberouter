# CubeRouter E2E Test Cases

本文件汇总当前 e2e 测试套件的全部用例（48 个），以及它们覆盖的业务场景。

## 测试基础设施

| 组件 | 说明 |
|---|---|
| 部署 | 全新部署（global setup 自动初始化 root，一次性闸门），SQLite |
| `e2e/global-setup.ts` | 运行前初始化部署（系统 setup 由它负责，spec 不再执行 setup） |
| MockLLM | `e2e/mockllm/launcher.py` 起的 OpenAI 兼容模拟上游（18000），支持 `/v1/models` 与确定性响应 |
| 环境变量 | 限流（CRITICAL/GLOBAL_API/GLOBAL_WEB）关闭、`USER_SESSION_ACTIVE_LIMIT=500`（见 `e2e/k8s/app.yaml`） |

运行方式：

```bash
cd e2e
npx playwright test                 # 全量 48 个
npx playwright test api-onboarding  # 指定文件
npx playwright test api-v1 api-v2   # 聚合 API v1+v2
npx playwright test channel.spec.ts # MockLLM 渠道/relay 套件
npx playwright test astraflow-channel.spec.ts # AstraFlow 渠道 UI
```

## 用例清单

### 1. `specs/journey.spec.ts` — 部署旅程冒烟（2 个）

| # | 用例 | 覆盖点 |
|---|---|---|
| 1 | API health is sane and bad credentials are rejected | `/api/status` 健康、错误凭据被拒 |
| 2 | sign-in and authenticated session work end to end | UI 登录 → 看板、API 登录、`/api/user/self` 会话校验 |

### 2. `specs/api-onboarding.spec.ts` — 对接流程（PPTX v4.5 认证规范，14 个）

PPTX 的 `/api/v2/*` onboarding 接口映射到产品真实接口（套餐/用户/订阅等），以下按 PPTX 的 API 编号与幻灯片标注。

| # | 用例 | PPTX 对应 | 覆盖点 |
|---|---|---|---|
| 1 | API#1 — admin creates the 3-month one-off plan | slide 3 | 创建一次性套餐（HKD 1288/3 个月/4 亿额度），字段校验、`No Reset→never`、货币归一 USD |
| 2 | API#1 — admin creates the 3-month top-up plan | slide 4 | 创建增值套餐（HKD 888/3 亿额度），sort_order 排序（高值靠前） |
| 3 | API#3 — admin creates the user and assigns the one-off plan | slide 6 | 建用户（邮箱即用户名）、绑定 one-off 套餐、3 个月有效期窗口断言 |
| 4 | API#6 — admin adds the top-up plan to the user | slide 8 | 追加 top-up 套餐；两个订阅均 active；FIFO 消费顺序（最早到期先扣） |
| 5 | API#2 — admin adjusts user quota | slide 5（标注 Not used） | 配额调整 +150 万，额度正确累加 |
| 6 | API#7 — admin suspends the user; the disabled user cannot sign in | slide 9 | 禁用用户；被挂起用户无法登录 |
| 7 | API#5 — admin reactivates the user; sign-in works again | slide 7 | 启用用户；恢复登录 |
| 8 | API#7/API#5 — the operator API_KEY stays valid through suspend and reactivate | slide 7/9 | **支持场景**：最新 API_KEY 有效期内可用；挂起后**用户的 sk- key 被拒（403）**；reactivate 后恢复；操作者 key 全程有效不被吊销 |
| 9 | API#4 — admin resets the user password | slide 7 | 管理员重置密码（旧密码失效、新密码可用） |
| 10 | slide 10 — user signs in and opens the dashboard | slide 10 | UI 登录 → 数据看板（Overview） |
| 11 | slide 11 — user changes the password in the UI and signs in with it | slide 11 | UI 改密码 → 新密码重新登录 |
| 12 | slide 12 — user views the one-off and top-up plans in the wallet | slide 12 | 钱包"我的订阅"显示 2 个 active 订阅、额度、FIFO 顺序 |
| 13 | slide 13 — user opens the playground and enters a question | slide 13 | 操练场入口可用（输入框/发送按钮） |
| 14 | appendix — plan input types are validated | slide 14 | 参数类型校验：缺 title、负价格、超 9999 上限均被拒 |

### 3. `specs/api-v1.spec.ts` — 聚合 API @ `/api/v1`（9 个）

### 4. `specs/api-v2.spec.ts` — 聚合 API @ `/api/v2`（9 个）

v1 与 v2 挂载同一组 handler（`router/api-router.go` 三前缀共享），用例一致、各自独立覆盖：

| # | 用例 | 覆盖点 |
|---|---|---|
| 1 | create plan (API#1) returns plan_id | `POST /plans` 创建套餐返回 `plan_id` |
| 2 | create user (API#3) returns user_id | `POST /users` 建用户（含 inviter_id）返回 `user_id` |
| 3 | suspend user (API#7) disables the account | `POST /users/:id/suspend` → `{status:success, status_code:2000}`；用户状态 `disabled`；无法登录 |
| 4 | reactivate user (API#5) restores the account | `POST /users/:id/reactivate` → 状态 `enabled`；恢复登录 |
| 5 | adjust quota (API#2) adds quota and reports totals | `POST /users/:id/adjust-quota` → `total_quota - current_quota == added_quota` |
| 6 | bind subscription (API#6) attaches the plan | `POST /users/:id/bind-subscription` → 状态接口 `plans` 中出现 active 订阅 |
| 7 | guards: root and missing users cannot be suspended | 禁 root（"不能禁用超级管理员"）、不存在用户、非法 id 均返回 fail |
| 8 | delete user (API#9) removes the account | `POST /users/:id/delete` → 用户无法登录、状态接口报"用户不存在" |
| 9 | get user status reports plan status, validity and quota | `GET /users/:id/status` → 绑定订阅返回 `status=active`；`validity_start_at`/`validity_end_at` 构成 3 个月有效期窗口；`plan_raw_quota`/`plan_remaining_quota` = 套餐 total_amount（未消耗） |

> 说明：`/api`、`/api/v1`、`/api/v2` 是同一套 dashboard API 的三个前缀（对外第三方稳定契约）；relay 的顶层 `/v1/*`（OpenAI 兼容）与 `/pg/*` 是另一棵路由树，不在此套件内。

### 5. `specs/channel.spec.ts` — MockLLM 渠道/API Key/relay/UI（14 个）

| # | 用例 | 覆盖点 |
|---|---|---|
| 1 | channel — admin creates an OpenAI channel pointing at MockLLM | 建 OpenAI 渠道（base_url 指向 mockllm），字段落库正确 |
| 2 | channel — connection test against MockLLM succeeds | `GET /api/channel/test/:id` 连通性测试通过 |
| 3 | channel — fetch_models pulls the OpenAI-compatible model list | `GET /api/channel/fetch_models/:id` 从 mockllm `/v1/models` 拉取模型列表（launcher workaround） |
| 4 | api key — create user, quota, token, and fetch the key | 建用户/配额/Token，`POST /api/token/:id/key` 取 sk- key |
| 5 | relay — chat completion returns the deterministic mock response | 经 relay 调用返回确定性响应 `pong from MockLLM`；用户 `used_quota` 增长 |
| 6 | relay — streaming returns the same deterministic content | SSE 流式拼接 delta 后内容一致（含 `data: [DONE]`） |
| 7 | relay — unconfigured prompts fall back to the mock default | 未配置 prompt 返回默认回退文案 |
| 8 | channel — update name and models | `PUT /api/channel/` 更新渠道 |
| 9 | channel — disabled channel makes the relay fail; re-enable recovers | 禁用渠道 → relay 报 "No available channel"；恢复后可用 |
| 10 | api key — disabled token is rejected; re-enable recovers | 禁用 Token → relay 401；恢复后可用 |
| 11 | api key — an invalid key is rejected | 非法 sk- key → 401 |
| 12 | playground — user sends a question in the UI and sees the mock reply | 操练场 UI 端到端：发消息 → 看到 mock 回复 |
| 13 | UI — usage logs show the responding channel and model | 管理员日志页显示响应的**渠道名 + #id** 与**模型名** |
| 14 | UI — add a model on the models page and verify it is listed | 模型管理页 Add Model → 表格显示该模型 |

### 6. `specs/astraflow-channel.spec.ts` — AstraFlow 渠道 UI（1 个）

| # | 用例 | 覆盖点 |
|---|---|---|
| 1 | UI — create an AstraFlow channel and see its Seedance models | 建 AstraFlow 渠道：type 59、base_url 留空走内置默认 `https://api.modelverse.cn`；MultiSelect 录入 Seedance 模型；落库校验（type=59、base_url 空、models 含 doubao-seedance-1-5-pro / doubao-seedance-2-0-260128、status=1） |

## 覆盖汇总

| 维度 | 用例数 |
|---|---|
| 部署旅程冒烟 | 2 |
| 对接（套餐/用户/订阅/配额/挂起/恢复/改密 + UI 流程） | 14 |
| 聚合 API v1 | 9 |
| 聚合 API v2 | 9 |
| MockLLM 渠道/API Key/relay/UI | 14 |
| **合计** | **48** |

关键回归点（值得重点关注）：
- **操作者 API_KEY 在 suspend/reactivate 全程有效**（不会被吊销）
- **被挂起用户的所有凭据失效**（登录拒绝 / 会话吊销 / sk- key 403），reactivate 后恢复
- **FIFO 订阅消费顺序**（最早到期先扣）
- **禁用渠道/Token 后 relay 立即失败**，恢复后可用
- **mockllm 流式内容与确定性响应**（含 launcher 对 `/v1/models` 的兼容 workaround）
