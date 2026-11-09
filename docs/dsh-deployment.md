# DSH Agent per-Organization Deployment (DSH 智能体按组织部署)

> Last Updated: 2026-11-09

本文档说明如何为**每个组织**部署一个独立的 DSH（DeepSeek Harness）运行时，并把网关、组织、容器三者绑定起来。适用对象是平台管理员（root）。

一句话概括：**一个组织 = 一个 `dsh` 容器 + 一条 type 62 的 DSH 渠道 + 一次管理端绑定**。附件按组织分目录落盘，任务按组织额度计费，个人令牌不能调用 DSH 任务接口。

## 1. 架构与隔离边界

```text
            浏览器
              │  /dsh 页面（任何已开通组织的成员）
              ▼
  ┌───────────────────────────────┐
  │ cuberouter 网关 (:3000)        │
  │  - DshAccess: 成员关系闸门     │
  │  - DshOrganization: 组织上下文 │
  │  - 组织渠道 pin（rank 5）      │
  └──────────────┬────────────────┘
                 │ http://dsh-org-<ORG_ID>:3080（共享网络 cuberouter-net）
     ┌───────────┼────────────┐
     ▼           ▼            ▼
 dsh-org-12   dsh-org-13   dsh-org-14   …每个组织一个容器
 /data 卷      /data 卷      /data 卷
     ▲
     └── 宿主机 <DSH_DATA_ROOT>/org-<ORG_ID>
         网关把附件写到 <DSH_DATA_ROOT>/org-<ORG_ID>/uploads/<userId>
         容器看到的是 /data/uploads/<userId>
```

| 边界 | 规则 |
|---|---|
| 运行时 | 每个组织一个容器、一份 `/data` 卷；组织之间不共享进程、不共享工作目录 |
| 选路 | 组织绑定的渠道把任务**钉死**在该组织的容器上（渠道 pin） |
| 附件 | `<DSH_DATA_ROOT>/org-<id>/uploads/<userId>`；跨组织路径在校验时不存在，直接拒绝 |
| 计费 | 强制组织上下文：扣组织额度，不扣个人钱包 |
| 凭据 | 组织的 DeepSeek Key 存在组织设置里（仅 root 可读写，列表脱敏），成员共享使用 |
| 个人令牌 | `POST /v1/tasks/dsh` 严格拒绝（管理员/root 也不例外）：没有组织就没有账本可记 |

「已开通」只有一个定义：**组织同时出现在白名单中、并绑定了一条可用的 DSH 渠道**。两者缺一，`/dsh` 入口不可见，任务与上传接口 403。

## 2. 前置条件与升级

1. **网关使用新的根 compose**（`dsh` 单实例服务已从中移除，改为共享卷 + 固定网络名）：

   ```bash
   docker compose -f docker-compose.yml up -d
   ```

   关键变化：

   - 网关卷：`/data/dsh:/data/dsh`（组织数据根目录，也是各组织容器 `/data` 卷的父目录）。
   - 网关环境：`DSH_UPLOAD_ROOT`（默认 `/data/dsh`）、`DSH_UPLOAD_MOUNT`（默认 `/data/uploads`）。
   - 网络：`cuberouter-network` 固定命名为 **`cuberouter-net`**，供 org 模板 `external: true` 接入。

2. **共享网络必须存在**（网关栈起来后自带；单独创建用）：

   ```bash
   docker network create cuberouter-net
   ```

3. **镜像**：org 模板从 `docker/dsh` 构建（`image: cuberouter-dsh:<tag>`），与旧单实例是同一镜像。首次 `up` 会自动构建。

   升级镜像时对每个组织项目重建：

   ```bash
   ORG_ID=12 DSH_API_KEY=... DEEPSEEK_API_KEY='...' \
     docker compose -p dsh-org-12 -f docker-compose.dsh-org.yml up -d --build
   ```

## 3. 为组织开通 DSH（三步）

以下以 `ORG_ID=12` 为例。

### 3.1 启动该组织的容器

```bash
ORG_ID=12 \
DSH_API_KEY=$(openssl rand -hex 24) \
DEEPSEEK_API_KEY='<组织的 DeepSeek 私钥>' \
docker compose -p dsh-org-12 -f docker-compose.dsh-org.yml up -d
```

必须注意：

- **`-p dsh-org-<ORG_ID>` 不能省**。compose 默认用目录名做项目名（`cuberouter`），会与网关栈撞车，把这个文件当成同一项目的 `dsh` 服务来重建；每个组织必须有自己的项目名。
- `DEEPSEEK_API_KEY` 用**单引号**包裹，避免 shell 对 `$`、`!` 等字符做展开。可以从面板「组织 DeepSeek Key」复制。
- `DSH_API_KEY` 是网关↔容器的共享密钥（渠道 Key），用 `openssl rand` 生成，结束后不要丢：下一步要填进渠道。
- `DSH_DATA_ROOT`（默认 `/data/dsh`）必须与网关的 `DSH_UPLOAD_ROOT` 指向**同一个宿主机目录**，否则附件在容器里看不到。
- 容器名 `dsh-org-12` 就是网络里的解析名；**Base URL 只能用 `http://dsh-org-12:3080`**。
- 端口 3080 不对宿主机发布：只有网关需要访问它。

可用变量：

| 变量 | 默认 | 说明 |
|---|---|---|
| `ORG_ID` | 必填 | 组织 ID，同时决定容器名与数据目录 |
| `DSH_API_KEY` | 必填 | 渠道 Key（网关→容器） |
| `DEEPSEEK_API_KEY` | 必填 | 容器的模型凭据（容器→模型） |
| `DSH_DATA_ROOT` | `/data/dsh` | 宿主机组织数据根目录 |
| `DSH_LLM_BASE_URL` | 空 | 把 DeepSeek 适配器指向任意 OpenAI 兼容端点（例如本网关 `/v1`） |
| `DSH_IMAGE_INPUT` | `on` | `off` 关闭图片输入（省去渲染后看图的时间） |
| `DSH_IMAGE_REVIEW_LIMIT` | `0` | 图片复核次数上限 |
| `DSH_TRUSTED_HOST` | 空 | 仅在浏览器直连容器 Web UI 时需要；网关走任务 API，不必设置 |
| `DSH_IMAGE_TAG` | `latest` | 镜像 tag |

### 3.2 创建 / 更新渠道

在「渠道管理」新建一条渠道：

- **类型**：`62`（任务插件 / Task Plugin）。
- **设置 → 插件标识（`task_plugin_key`）**：`dsh`。填错等于这条渠道不会被 DSH 使用。
- **Base URL**：`http://dsh-org-12:3080`。**禁止**使用 `http://dsh:3080`——共享网络上 `dsh` 是所有组织项目共用的服务别名，指向谁不确定。
- **Key**：上一步的 `DSH_API_KEY`。
- **分组**：必须包含该组织的分组（默认 `default`，即组织的 `group` 字段）。渠道能力按 `(渠道, 分组, 模型)` 生成，分组不匹配时模型下拉会是空的。
- **模型**：至少包含 `deepseek-flash`（与 `docker/dsh/overlay.yml` 的模型目录一致）。

### 3.3 管理端绑定 + 组织密钥

「系统设置 → Operations → DSH Agent Access」：

1. 在组织多选里勾选该组织（白名单）。
2. 该组织行内选择上一步的渠道，点「Save binding」。绑定成功后该组织的成员立即可以打开 `/dsh`；解绑或清空白名单则立即失去访问（轮询中的旧任务仍可能在上游继续执行）。
3. 在「Organization DeepSeek key」里粘贴组织的 DeepSeek 私钥并保存。它是**面板里的一份副本**：成员共享使用、可在需要时复制回 compose 命令；修改它**不会**自动下发到容器，容器用的是 `DEEPSEEK_API_KEY` 环境变量（见 §5）。
4. 行内会显示建议 Base URL（`http://dsh-org-12:3080`），可一键复制。

参考接口（root）：

| 方法 | 路径 | 用途 |
|---|---|---|
| GET | `/api/admin/dsh/channels` | 列出 type 62 + `dsh` 的渠道及已绑定组织 |
| GET | `/api/admin/dsh/organizations/:id` | 组织配置、渠道状态、密钥脱敏预览 |
| PUT | `/api/admin/dsh/organizations/:id` | 绑定渠道（`channel_id`）、写入密钥（`deepseek_key`）、清除密钥（`clear_deepseek_key`） |
| POST | `/api/admin/dsh/organizations/:id/secret` | 读取密钥明文（用于复制进 compose），响应 `Cache-Control: no-store` |

绑定时的服务端校验：渠道必须是启用的 type 62 + `dsh`、Base URL 非空、渠道分组包含组织分组、同一渠道不能绑定到另一个组织。

## 4. 旧单实例迁移

旧部署是根 compose 里的单个 `dsh` 服务，数据在 `/data/dsh/zhongshan`。迁移到一个组织（假定 `ORG_ID=12`）：

```bash
# 1) 停掉旧容器（根 compose 已删除该服务，直接按容器名停）
docker stop dsh && docker rm dsh

# 2) 目录改名：新布局是 <DSH_DATA_ROOT>/org-<ORG_ID>
mv /data/dsh/zhongshan /data/dsh/org-12

# 3) 用 §3.1 的命令启动（可沿用旧 DSH_API_KEY，也可轮换后同步改渠道 Key）
# 4) 在管理端按 §3.2 / §3.3 创建渠道并绑定组织 12

# 5) 校验：容器健康 + 目录属主
docker inspect --format '{{.State.Health.Status}}' dsh-org-12
ls -la /data/dsh/org-12
```

注意：

- 旧容器里所有用户的会话/工作区都在同一份数据里，迁移后归 org 12 的成员共享（历史任务不区分组织）。如果需要把历史数据分给多个组织，按 `workspace/<userId>` 手工拆分到各 `org-<id>`。
- 网关侧 `DSH_UPLOAD_ROOT` 之前是 `DSH_UPLOAD_DIR`；升级根 compose 后旧环境变量不再生效，确认新变量指向 `/data/dsh`。

## 5. Key 轮换

这里有两把**互不相同**的 Key：

| Key | 链路 | 存在哪里 | 轮换方式 |
|---|---|---|---|
| `DSH_API_KEY` | 网关 → 容器 | 容器环境变量 + 渠道 Key | 先改容器环境重启，再改渠道 Key（顺序颠倒会短暂 401） |
| `DEEPSEEK_API_KEY` | 容器 → 模型 | 仅容器环境变量 | 用新值重新 `up -d`（compose 会重建容器）；面板里的组织密钥同步更新，否则复制的副本会过期 |

`DSH_API_KEY` 轮换：

```bash
ORG_ID=12 DSH_API_KEY=$(openssl rand -hex 24) DEEPSEEK_API_KEY='...' \
  docker compose -p dsh-org-12 -f docker-compose.dsh-org.yml up -d
# 然后把新 key 填进渠道（渠道管理 → 该渠道 → 密钥）
```

`DEEPSEEK_API_KEY` 轮换：面板更新组织密钥 → 用相同命令（新 `DEEPSEEK_API_KEY`、原 `DSH_API_KEY`）重新 `up -d`。

## 6. 扩容、缩容与解绑

- **新增组织**：重复 §3 的三步。每个组织独立项目名、独立容器、独立数据目录。
- **删除组织实例**：

  ```bash
  # 先在管理端点「Save binding」解绑（或取消勾选该组织），让流量停止
  docker compose -p dsh-org-12 -f docker-compose.dsh-org.yml down
  ```

  `down` 不删除宿主机的 `/data/dsh/org-12`；确认不再需要后再手工清理。解绑后该组织的成员访问 `/dsh` 立即 403，但已在执行的任务仍可能在上游跑完。

- **同一组织多副本**：不支持。一个组织只应绑定一条渠道、一个容器；渠道被第二个组织绑定时服务端会拒绝。

## 7. 排障

| 现象 | 排查 |
|---|---|
| `/dsh` 入口不可见 / 页面 403 `organization_unavailable` | 组织是否既在白名单、又绑定了渠道？用户是否为该组织的 **active** 成员？ |
| 任务 403 `organization_channel_unavailable` | 渠道被停用 / 类型不是 62 / 插件标识不是 `dsh` / Base URL 为空；或令牌 key 里带了 `-<channelId>` 后缀（令牌渠道 pin 会与组织 pin 冲突，被显式拒绝） |
| 提交 403 `organization_required` | 用了**个人令牌**调用 `POST /v1/tasks/dsh`。DSH 只接受组织作用域的令牌 |
| 模型下拉为空 | 渠道分组是否包含组织分组（默认 `default`）？渠道上是否声明了模型？成员白名单是否把模型排除了？ |
| 网关连不上容器（`dial tcp: lookup dsh-org-12 ... no such host`） | 网关与容器是否在同一个 `cuberouter-net`？容器是否在运行？Base URL 是否误写成 `http://dsh:3080`？ |
| 容器 `unhealthy` | `docker logs dsh-org-12`；健康检查用到 `DSH_API_KEY` 与 `/tmp/dsh-bind-host`（entrypoint 写入），容器内 `wget` 是否存在 |
| 附件“找不到” | 网关 `DSH_UPLOAD_ROOT` 与容器 `DSH_DATA_ROOT` 是否指向同一宿主机目录？写盘目录属主是否允许容器读取？ |
| 组织 A 的成员看到组织 B 的附件 | 不应发生：校验只认 `<root>/org-<id>/uploads/<userId>` 下的文件，跨组织路径直接判缺失 |

日志：

```bash
docker logs -f dsh-org-12
docker compose -p dsh-org-12 -f docker-compose.dsh-org.yml ps
```

## 8. 安全说明

- `DEEPSEEK_API_KEY` 会出现在容器环境变量里，`docker inspect` / `docker compose config` 都能看到；宿主机权限即边界。不要在文档、工单、聊天里贴明文。
- 面板里的组织密钥只对 **root** 可写；读取明文需要显式调用 reveal 接口，响应不缓存。列表接口只返回脱敏预览（如 `sk-...abcd`）。
- `DSH_API_KEY` 是网关到容器的唯一鉴权；泄露等于可以绕过网关直接驱动该组织的智能体。
- 任务接口只接受组织令牌：个人令牌、以及不属于已开通组织的令牌一律 403。管理员/root 若不在任何已开通组织中也同样被拒绝（可以配置，但不能使用）。
- 不要对宿主机发布 3080 端口；容器只应通过共享网络被网关访问。
- 共享网络上**永远不要**用 `http://dsh:3080`：它是所有组织项目共用的服务别名，指向哪个容器不确定。
