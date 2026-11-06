# dsh-api (vendored)

DeepSeek Harness 的对外 HTTP API 插件，CubeRouter 的 DSH 任务渠道通过它提交
任务、轮询结果并下载产物。

- 上游：https://github.com/cuteG41cute/dsh-api（MIT，见 `LICENSE`）
- 版本：1.2.0（在上游 1.1.0 基础上增加 `GET /api/v1/tasks/:taskId/files/:key`
  产物下载路由、CORS 预检应答与 `maxArtifactBytes` 上限）
- 安装位置：镜像内 `/opt/dsh-plugins/dsh-api`，由 `dsh plugin --profile web add`
  装入 profile；运行时通过 `overlay.yml` 中的 `dsh-api` 行挂载

| 配置键 | 默认 | 说明 |
| --- | --- | --- |
| `prefix` | `/api/v1` | 路由前缀 |
| `apiKey` | 空（关闭鉴权） | 必填密钥；容器里从 `DSH_API_KEY` 注入 |
| `defaultCwd` | `process.cwd()` | 未指定时任务的工作目录，容器里指向 `/data/workspace` |
| `maxArtifactBytes` | 67108864 | 单个产物下载上限（64 MiB） |
