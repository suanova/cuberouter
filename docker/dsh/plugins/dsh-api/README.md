# dsh-api (vendored)

DeepSeek Harness 的对外 HTTP API 插件，CubeRouter 的 DSH 任务渠道通过它提交
任务、轮询结果并下载产物。

- 上游：https://github.com/cuteG41cute/dsh-api（MIT，见 `LICENSE`）
- 版本：1.3.0（在上游 1.1.0 基础上增加 `GET /api/v1/tasks/:taskId/files/:key`
  产物下载路由、CORS 预检应答、`maxArtifactBytes` 上限、按用户归属的
  workspace 与会话隔离）
- 安装位置：镜像内 `/opt/dsh-plugins/dsh-api`，由 `dsh plugin --profile web add`
  装入 profile；运行时通过 `overlay.yml` 中的 `dsh-api` 行挂载

| 配置键 | 默认 | 说明 |
| --- | --- | --- |
| `prefix` | `/api/v1` | 路由前缀 |
| `apiKey` | 空（关闭鉴权） | 必填密钥；容器里从 `DSH_API_KEY` 注入 |
| `defaultCwd` | `process.cwd()` | 未指定时任务的工作目录，容器里指向 `/data/workspace` |
| `questionTimeoutMs` | 600000 | `ask_user_question` 等待人工回答的时长；超时按“用户跳过”结算让任务继续，`<= 0` 表示一直等 |
| `imageReviewLimit` | 0 | 单个任务允许调用 `read_image` 的次数；超出后拒绝并提示改用文本层面的结构检查。`0` 表示不限制（保持原有“渲染→看图→改版”循环） |
| `maxArtifactBytes` | 67108864 | 单个产物下载上限（64 MiB） |

## 用户归属

网关（CubeRouter 的 DSH 任务渠道）会在提交请求的 `X-DSH-User-Id` 头和
`userId` 字段里带上任务提交者的用户 ID。

- 带 userId 的新任务，cwd 为 `<defaultCwd>/<userId>`，目录不存在时自动创建；
  显式 `cwd` 必须位于该用户目录内，否则 400 `bad_cwd`。
- 续会话（带 `sessionId`）始终沿用首轮任务的 cwd；换 cwd 返回 400 `bad_cwd`，
  换用户返回 404 `unknown_session`（与未知会话同文案）。
- 查询、取消、下载产物都要带同一个 `X-DSH-User-Id`，否则返回 404；任务列表
  也只返回该用户自己的任务。不带 userId 的旧任务保持共享目录和旧的可见性。
- 会话文件本身按 cwd 分目录存放（`sessions/--<cwd 转义>--/<sessionId>/`），
  因此用户目录一分开，session 也随之分开。

注意：这是目录与归属层面的隔离，不是保密沙箱。agent 仍有 shell，可读取
工作目录以外的文件；真正的读写边界由 dsh 的 sandbox 策略提供。生产部署应设置
非空 `DSH_API_KEY` 且不要把 3080 直接暴露给不可信客户端——所有者身份是网关
对已鉴权请求的声明，dsh-api 无法独立验证。
