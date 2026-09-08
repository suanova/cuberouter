# cube-router Helm Chart

将 CubeRouter（应用 + 文档站）及其依赖组件（PostgreSQL、Redis）安装到 Kubernetes，支持 HA。

结构（自包含，离线可安装，无需 `helm dependency update`）：

```
helm/cuberouter-chart/
├── Chart.yaml          # cube-router 0.7.0，含两个条件依赖
├── values.yaml
├── templates/          # 应用、配置、secret、HPA/PDB、Ingress、HA CR
└── charts/
    ├── postgresql-operator/   # CrunchyData PGO v6.0.2（vendored，含 CRD）
    └── redis-operator/        # Spotahome redis-operator 3.3.0（vendored，含 CRD）
```

## 部署模式

| 组件 | 单机模式（0.6.0 行为） | HA 模式 |
|---|---|---|
| PostgreSQL | `postgresql.enabled=true`，StatefulSet | `postgresql.crunchy.enabled=true` + `postgresql-operator.enabled=true`，PostgresCluster（Patroni 自动故障切换，默认 2 实例） |
| Redis | `redis.enabled=true`，StatefulSet | `redis.operator.enabled=true` + `redis-operator.enabled=true`，RedisFailover（默认 2 redis + 3 sentinel） |

两种模式互斥；两者都不开时需通过 `secrets.SQL_DSN` / `secrets.REDIS_CONN_STRING` 指向外部实例。

## 安装

### 全 HA（推荐）

```sh
helm install cube-router ./helm/cuberouter-chart -n cube-router --create-namespace \
  --set postgresql.enabled=false \
  --set postgresql.crunchy.enabled=true \
  --set postgresql-operator.enabled=true \
  --set redis.enabled=false \
  --set redis.operator.enabled=true \
  --set redis-operator.enabled=true
```

### 单机（默认值即可，零参数）

```sh
helm install cube-router ./helm/cuberouter-chart -n cube-router --create-namespace
```

### 外部依赖（自建 PostgreSQL / Redis）

```sh
kubectl create secret generic cube-router-secret -n cube-router \
  --from-literal=SQL_DSN='postgresql://user:pass@db.example:5432/db' \
  --from-literal=REDIS_CONN_STRING='redis://:pass@redis.example:6379' \
  --from-literal=SESSION_SECRET='...' --from-literal=CRYPTO_SECRET='...' \
  --from-literal=REDIS_PASSWORD='pass' --from-literal=POSTGRES_PASSWORD='pass'

helm install cube-router ./helm/cuberouter-chart -n cube-router --create-namespace \
  --set postgresql.enabled=false \
  --set redis.enabled=false \
  --set secret.create=false --set secret.existingSecret=cube-router-secret
```

## 凭据模型

`secret.create=true`（默认）时，chart 创建 `<fullname>-secret`，键为
`SQL_DSN / REDIS_CONN_STRING / SESSION_SECRET / CRYPTO_SECRET / REDIS_PASSWORD / POSTGRES_PASSWORD`：

- `secrets.*` 显式值优先；
- 留空且集群中已存在该 secret（upgrade 场景，通过 `lookup` 读取）→ 沿用原值，**升级不会改密码**；
- 全新安装且留空 → `randAlphaNum` 自动生成。

`SQL_DSN` / `REDIS_CONN_STRING` 留空时按当前模式自动计算：

| 模式 | SQL_DSN | REDIS_CONN_STRING |
|---|---|---|
| Crunchy HA | `postgresql://<user>:<pw>@<f>-postgres-primary:5432/<db>` | — |
| Redis HA | — | `redis://:<pw>@rf-rm-<f>-redis:6379`（operator 维护的 master 服务） |
| 单机 | `postgresql://<user>:<pw>@<f>-postgresql:5432/<db>` | `redis://:<pw>@<f>-redis:6379` |

### HA 下的密码衔接

- **PostgreSQL（Crunchy）**：operator 自动生成密码、CR 无法指定，因此 chart 渲染一个
  post-install/post-upgrade hook Job（`<f>-postgres-init`）：等待集群就绪后，以 superuser
  （`<f>-postgres-pguser-postgres` secret）把 chart 生成的密码同步到应用角色，并确保
  数据库存在、`public` schema 归应用角色所有（PG15+ 需要）。脚本幂等。
- **Redis（Spotahome）**：使用 operator 原生认证 —— chart 渲染
  `<f>-redis-auth` secret（`password` 键），CR 引用 `spec.auth.secretPath`，
  operator 自动把 `requirepass`/`masterauth` 注入 redis/sentinel 配置。

## HA 资源命名

| 资源 | 名称 |
|---|---|
| PostgresCluster CR | `<f>-postgres` |
| PG primary 服务 | `<f>-postgres-primary:5432` |
| PG user/superuser secret（operator 生成） | `<f>-postgres-pguser-<user>` / `<f>-postgres-pguser-postgres` |
| RedisFailover CR | `<f>-redis`（operator 限制 ≤48 字符） |
| Redis master 服务（应用连接目标） | `rf-rm-<f>-redis:6379` |
| Sentinel 服务 | `rfs-<f>-redis:26379` |

`<f>` = release 名（无 override 时），与 0.6.0 的 `cube-router-app`、`cube-router-redis` 等命名一致。

## 升级与回滚

- `helm upgrade` 安全：已生成的密码通过 `lookup` 保持稳定；Crunchy 的 init Job 每次
  upgrade 都会重跑（幂等，重新同步密码/授权）。
- 从 0.6.0 迁移：保持 `secret.create=false` + `existingSecret=cube-router-secret`，
  并把 `postgresql.enabled` / `redis.enabled` 置 false、启用对应 HA 模式；
  如需数据迁移，先导入 PG/Redis 再切换（本 chart 不内置数据迁移）。
- 卸载：`helm uninstall` 会删除 PostgresCluster/RedisFailover CR 及其 PVC
  （`--keep-json` 场景除外）；卸载前请确认备份。

## 常用调参（values 摘要）

- `postgresql.crunchy.replicas`（默认 2，Patroni HA）、`storage`（20Gi）、`image`（留空=operator RELATED_IMAGE）、`backups.*`（pgBackRest 全量备份，默认每周日 02:00）、`pgBouncer.*`
- `redis.operator.redisReplicas`（2）、`sentinelReplicas`（3）、`redisCustomConfig`（运行时 CONFIG SET）、`persistence.*`
- `cubeRouter.*`、`docs.*`、`hpa.*`、`pdb.*`、`ingress.*` 与 0.6.0 相同
- `global.imagePullSecrets` 默认 `[{name: huawei-swr}]`

## 验证

```sh
helm lint helm/cuberouter-chart
helm template cube-router helm/cuberouter-chart -n cube-router
# 全 HA：
helm template cube-router helm/cuberouter-chart -n cube-router \
  --set postgresql.enabled=false --set postgresql.crunchy.enabled=true \
  --set postgresql-operator.enabled=true --set redis.enabled=false \
  --set redis.operator.enabled=true --set redis-operator.enabled=true
```
