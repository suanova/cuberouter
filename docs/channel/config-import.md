# cuberouter 渠道引导配置（channels import）

一个 YAML 文件就能完成「建渠道 → 拉取上游模型」。
cuberouter 直接读取该文件执行，适合首次部署和重复声明式同步。

```bash
# 先演练：解析 + 真实拉取上游模型，但不写数据库
./cuberouter channels import -f channels.yaml --dry-run

# 同时打印上游拉取到的模型列表
./cuberouter channels import -f channels.yaml --dry-run --verbose

# 正式执行
./cuberouter channels import -f channels.yaml
```

`import` 是 `channels import` 的简写，两种写法等价。

## setup 后自动执行（推荐）

在服务端进程设置环境变量即可，不需要手动执行 CLI：

```bash
CHANNEL_CONFIG_FILE=/data/channels.yaml
```

```yaml
# docker-compose 示例
services:
  cuberouter:
    environment:
      CHANNEL_CONFIG_FILE: /data/channels.yaml
    volumes:
      - ./channels.yaml:/data/channels.yaml:ro
```

- Web 完成首次 setup 后，服务端立即执行一次导入：建渠道、拉取模型。结果随 setup 响应返回（`channel_config`）。
- 文件不存在时静默忽略，setup 正常成功；其他组件稍后写入文件后会自动导入，不需要重新 setup。
- 服务端每 60 秒检查一次文件，内容变化时自动重新执行完整导入，无需重启 Pod（见下节 Kubernetes）；内容没变则不会重复写库。
- 渠道页「上游更新」的检测/应用（单个或全部）会更新模型列表，不会改动你在模型定价页面维护的价格。
- 文件在每次操作时重新读取，按渠道名匹配；文件里没有同名渠道时跳过并记录日志。
- 未设置 `CHANNEL_CONFIG_FILE` 时不会自动执行，只能手动 `channels import`。

### Kubernetes / ConfigMap

适配「其他组件写 ConfigMap，cuberouter 读取 channels.yaml」的部署方式：

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: cuberouter
spec:
  replicas: 1
  template:
    spec:
      containers:
        - name: cuberouter
          image: cuberouter:latest
          env:
            - name: CHANNEL_CONFIG_FILE
              value: /etc/cuberouter/channels.yaml
          volumeMounts:
            - name: channel-config
              mountPath: /etc/cuberouter
              readOnly: true
      volumes:
        - name: channel-config
          configMap:
            name: cuberouter-channels
```

要点：

- **挂载目录，不要用 `subPath`**：kubelet 通过原子替换 `..data` 符号链接来更新 ConfigMap，`subPath` 挂载不会收到更新。上面示例把 ConfigMap 挂到 `/etc/cuberouter`，文件即 `/etc/cuberouter/channels.yaml`。
- 文件不存在时不会报错，也不会写库；其他组件写入文件后，下一次检查会自动导入。
- ConfigMap 更新后，kubelet 刷新通常需要几十秒，再加上最多 60 秒的检查周期后生效。
- 每次内容变化都会重新执行**完整导入**：新增/更新渠道、重新拉取模型；内容没变则不会重复写库。
- 多副本时只有 `NODE_TYPE != "slave"` 的 master 节点执行文件监听，其他副本通过常规渠道同步循环从数据库感知；这与数据库迁移的单 master 约定一致。
- 解析或上游拉取失败只记录日志并等待下一个周期重试，不会退出进程；同一个错误内容只记录一次，避免刷屏。
- ConfigMap 大小受 etcd 限制（约 1 MiB），渠道很多时注意 YAML 体积。

## 执行顺序

1. 解析并校验 YAML（未知字段直接报错，避免拼写错误被静默忽略）。
2. **预检**：为每个渠道拉取上游 `/v1/models` 模型列表。任何一个渠道构建失败或拉取失败，整次导入在写库前终止。
3. 创建或更新渠道（按 `name` 匹配；同名已存在则更新，因此可重复执行）。

## 上游模型如何生效、如何查看

`models.fetch` 默认为 `true`，渠道的模型列表直接来自上游 `GET /v1/models`：

- `include` / `exclude` 只过滤**拉取结果**，语法是 glob（`*`、`?`）。
- `models.list` 是**手工追加**的模型，不参与过滤；上游没有 `/v1/models` 时用它兜底。
- 最终列表去重排序后写入渠道 `models` 字段，并同步重建 `abilities`；管理后台的渠道详情里看到的也是这份列表。
- 上游新增模型时重跑一次即可；上游删除的模型会从渠道列表移除。
- 渠道页的「上游更新」检测/应用（单个或全部）会直接更新模型列表，不需要重跑 CLI。

查看实际拉取结果：

```bash
./cuberouter channels import -f channels.yaml --dry-run --verbose
```

```
[updated] cubestack-local  type=64 (CubeStack)  models=2 (fetched 3)  id=1
        models (2):
          - Qwen3-32B
          - Qwen3-8B
        filtered out by include/exclude (1):
          - text-embedding-v3
```

不写 `--verbose` 时只显示汇总行（`models=2 (fetched 3)`），避免模型很多时刷屏。

## 完整示例

```yaml
version: 1

channels:
  - name: cubestack-local
    type: cubestack                 # 也支持 openai / anthropic / gemini / 64 等
    base_url: http://127.0.0.1:8000 # 即 api-address；cubestack 无默认地址，必填
    key_env: CUBESTACK_API_KEY
    group: default
    models:
      fetch: true                   # 自动请求上游 /v1/models
      exclude: ["*-embedding"]
    overrides:
      temperature: 0.7
```

执行成功后的输出：

```
[created] cubestack-local  type=64 (CubeStack)  models=2 (fetched 2)  id=1
```

## 字段说明

### 顶层

| 字段 | 类型 | 说明 |
|---|---|---|
| `version` | int | 目前只支持 `1`，省略时按 1 处理 |
| `channels` | list | 渠道列表，见下，必填且至少一个 |

### channels[]

| 字段 | 类型 | 说明 |
|---|---|---|
| `name` | string | 必填，渠道名兼幂等键；同名渠道会被更新 |
| `type` | int/string | 必填，渠道类型 ID 或名称（大小写、`-`/`_` 不敏感），如 `cubestack`、`openai`、`anthropic`、`gemini`、`64` |
| `base_url` | string | 上游 API 地址。留空时使用该类型的默认地址；没有默认地址的类型（如 cubestack）必填 |
| `key` / `keys` / `key_env` | string/list/string | 三选一：单个 Key、多个 Key，或环境变量名。多个 Key 时自动开启多 Key 轮询 |
| `group` | string | 渠道分组，默认 `default`，可逗号分隔多个分组 |
| `priority` | int | 渠道优先级，越大越优先 |
| `weight` | int | 同优先级下的权重 |
| `status` | 0/1 | 1 启用、0 禁用；省略时新建为 1、更新时保持原值 |
| `tag` | string | 渠道标签 |
| `remark` | string | 备注，最长 255 |
| `test_model` | string | 渠道测试使用的模型 |
| `auto_ban` | 0/1 | 是否允许自动禁用 |
| `multi_key_mode` | string | `random`（默认）或 `polling` |
| `model_mapping` | map | 模型名映射（客户端模型名 → 上游模型名） |
| `overrides` | map | 参数覆盖，支持简单 JSON 路径写法或 `operations` 条件写法 |
| `header_override` | map | 请求头上游覆盖 |
| `proxy` | string | 渠道出网代理 |
| `settings` | map | 额外 `ChannelSettings`（如 `force_format`、`http_protocol`） |
| `other_settings` | map | 额外 `ChannelOtherSettings`（如 `upstream_model_update_check_enabled`） |
| `other` | string | 原样写入渠道 `other` 字段（Vertex AI 地区等特殊渠道使用） |

### channels[].models

| 字段 | 类型 | 说明 |
|---|---|---|
| `fetch` | bool | 是否从上游 `/v1/models` 拉取，默认 `true` |
| `list` | list | 手工追加的模型，永远保留，不受 include/exclude 影响 |
| `include` | list | 仅保留匹配的拉取结果，glob 语法（`*`、`?`） |
| `exclude` | list | 排除匹配的拉取结果，glob 语法 |

`fetch: false` 时 `list` 必填。最终模型列表会去重并排序。

### overrides

`overrides` 原样写入渠道的 `param_override` 字段，支持两种写法。

简单写法（JSON 路径 → 值）：

```yaml
overrides:
  temperature: 0.7
```

条件写法（`operations`）：

```yaml
overrides:
  operations:
    - mode: set
      path: output_config.effort
      value: low
      logic: AND
      conditions:
        - path: model
          mode: full
          value: qwen38-27b
        - path: output_config.effort
          mode: full
          value: high
```

- `mode`：`set`、`delete`、`prepend`、`append`、`replace`、`return_error` 等，完整列表见 `relay/common/override.go`。
- `logic`：`AND` 或 `OR`，默认 `OR`。
- 条件 `mode`：`full`、`prefix`、`suffix`、`contains`、`gt`、`gte`、`lt`、`lte`。
- 条件先匹配请求 JSON，再匹配上下文；`model` 在上下文中是上游模型名。渠道配置了 `model_mapping` 且客户端使用别名时，请求体里的 `model` 是别名，`model == 上游名` 不成立，可改用 `upstream_model`。
- 只有条件命中才执行 `set`。上例中 `output_config.effort` 不存在时不会变成 `low`；如需缺省也降级，可在该条件上加 `pass_missing_key: true`，或去掉值条件。
- 导入时会校验 `operations` 的基本结构（列表、`path`/`mode` 必填、`logic` 只能是 AND/OR），但不会校验上游是否支持这些字段。

## 重复执行与生效时间

- 渠道按 `name` 幂等；重复执行只会更新，不会重复创建。
- 更新渠道时，配置文件**负责的字段**（类型、Key、地址、模型、分组、覆盖参数）会以文件为准；
  未出现的字段（如 `status`、`priority`、`remark`）保持数据库中的原值。
- 在运行中的实例上执行导入时，服务端通过常规数据库同步循环（默认 `SYNC_FREQUENCY` 秒）感知
  新渠道。若希望立即生效，可在启动前执行，或等待一个同步周期。
- 这是对数据库的直接写入，请确保执行环境能访问同一数据库（`SQL_DSN` / `SQLITE_PATH` 等环境变量一致）。
- 首次运行时命令会像启动服务一样执行数据库迁移；`--dry-run` 也会连接并迁移数据库，但不写入渠道。
- 任一步失败时命令以非 0 退出：预检（配置、上游模型）失败不会写库；写库阶段单渠道失败会被收集，其余渠道仍会继续处理，修复后重跑即可收敛。
