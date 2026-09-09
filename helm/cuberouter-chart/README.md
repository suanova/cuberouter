# cuberouter Helm Chart

Installs **CubeRouter** (the AI gateway app + its docs site) on Kubernetes together with its
dependency stores (PostgreSQL and Redis). The chart is **self-contained**: the two operator
subcharts are vendored under `charts/`, so it can be installed offline without `helm dependency update`.

| | |
|---|---|
| Chart | `cuberouter` **0.7.0** |
| App version | `v1.1.55-isuanova-agent-release` |
| Components | CubeRouter app, docs site, PostgreSQL (CrunchyData PGO), Redis (OpsTree redis-operator) |

## Contents

- [Deployment modes](#deployment-modes)
- [Prerequisites](#prerequisites)
- [Images and registry access](#images-and-registry-access)
- [Installation](#installation)
- [Accessing the app](#accessing-the-app)
- [Verifying the install](#verifying-the-install)
- [Secrets and credentials](#secrets-and-credentials)
- [Resource names and endpoints](#resource-names-and-endpoints)
- [Key values at a glance](#key-values-at-a-glance)
- [HA details](#ha-details)
- [Upgrading and rolling back](#upgrading-and-rolling-back)
- [Uninstalling](#uninstalling)
- [Validating the chart locally](#validating-the-chart-locally)

## Deployment modes

PostgreSQL and Redis are **required** and are always provisioned by the chart
through their operators:

| Component | What the chart installs |
|---|---|
| PostgreSQL | `PostgresCluster` CR (CrunchyData PGO / Patroni, PostgreSQL 16, 2 replicas) + PGO control plane |
| Redis | `RedisReplication` CR (1 master + 1 replica, embedded 3-sentinel set) + OpsTree control plane |

The operator subcharts (CRDs + control-plane Deployments) are always installed
together with the stores.

## Prerequisites

- **Helm 3.x** and **kubectl**
- A Kubernetes cluster with a **StorageClass** capable of `ReadWriteOnce` volumes (default sizes:
  app data 10Gi, app logs 10Gi, PostgreSQL 20Gi, backups 20Gi — each tunable)
- Multi-node clusters recommended so HA replicas spread across hosts (a soft pod anti-affinity is
  rendered for Crunchy instance pods)
- Network access from the cluster to the container registries hosting the images (see next section)

## Images and registry access

| Component | Default image | Values key |
|---|---|---|
| App | `harbor.isuanova.com/suanova/cuberouter:latest` | `cubeRouter.image.repository` / `.tag` |
| Docs | `harbor.isuanova.com.cn/suanova/cuberouter:latest` | `docs.image.repository` / `.tag` |
| PostgreSQL instances | operator `RELATED_IMAGE_POSTGRES_<postgresVersion>` (default `POSTGRES_16`) | `postgresql.image` (empty) or `postgresql-operator.relatedImages` |
| Redis instances | `quay.io/opstree/redis:v7.0.15` | `redis.image.repository` / `.tag` |
| Redis sentinel | `quay.io/opstree/redis-sentinel:v7.0.15` | `redis.sentinelImage.repository` / `.tag` |
| CrunchyData PGO operator | `registry.developers.crunchydata.com/crunchydata/postgres-operator:ubi9-6.0.2-0` | `postgresql-operator.image.*` |
| OpsTree redis-operator | `quay.io/opstree/redis-operator:v0.26.0` | `redis-operator.image.*` |

All images are plain registry images; the chart does not manage image pull secrets — create one in
the namespace (or configure the nodes/registry) if your registry requires authentication.

## Installation

The release name is the base for every resource name (see
[Resource names and endpoints](#resource-names-and-endpoints)), so the examples below use
`cuberouter` to reproduce the canonical `cuberouter-app`, `cuberouter-redis`, etc. naming.

All installs start from the chart directory:

```sh
# from the repository root
helm install cuberouter ./helm/cuberouter-chart -n cuberouter --create-namespace [flags]
```

### 1. Full HA (default values)

The defaults already install the app, the docs site, the HA PostgreSQL cluster and the HA Redis
failover — plus both operator control planes, HPA and PDB:

```sh
helm install cuberouter ./helm/cuberouter-chart -n cuberouter --create-namespace
```

For a smaller cluster, override first:

```yaml
# my-values.yaml
ingress:
  enabled: false              # you'll reach the app via port-forward instead
hpa:
  enabled: false              # defaults: app already runs 2 replicas
pdb:
  enabled: false
cubeRouter:
  replicaCount: 1
  persistence:
    data: { size: 5Gi }
    logs: { size: 2Gi }
postgresql:
  storage: 10Gi
  replicas: 1                 # single instance = no HA, still operator-managed
  backups:
    enabled: false
redis: {}                    # keep defaults: 2 nodes + 3 sentinels (embedded-sentinel
                              # failover needs a quorum of 2, so 1+1 is not a valid setup)
```

```sh
helm install cuberouter ./helm/cuberouter-chart -n cuberouter \
  --create-namespace -f my-values.yaml
```

### 2. Using an existing secret (0.6.0 pattern)

To manage all credentials yourself, set `secret.create=false` and point
`secret.existingSecret` at a pre-created secret carrying **all** six keys (see
[Secrets and credentials](#secrets-and-credentials)). The chart still creates the
operator-managed stores, so `SQL_DSN` / `REDIS_CONN_STRING` must target the services the
chart renders (`<fullname>-postgres-primary:5432`, `<fullname>-redis-master:6379`) with the
passwords you set.

```sh
kubectl create secret generic cuberouter-secret -n cuberouter \
  --from-literal=SQL_DSN='postgresql://cuberouter-user:pass@cuberouter-postgres-primary:5432/cuberouter_db' \
  --from-literal=REDIS_CONN_STRING='redis://:pass@cuberouter-redis-master:6379' \
  --from-literal=SESSION_SECRET='...' \
  --from-literal=CRYPTO_SECRET='...' \
  --from-literal=REDIS_PASSWORD='pass' \
  --from-literal=POSTGRES_PASSWORD='pass'

helm install cuberouter ./helm/cuberouter-chart -n cuberouter --create-namespace \
  --set secret.create=false \
  --set secret.existingSecret=cuberouter-secret
```

## Accessing the app

The app service is `ClusterIP` on **port 80 → container 3000**; the docs service is also port 80.
Two common ways in:

**Ingress** — the chart ships an `nginx` Ingress by default, but with the production host names
(`cuberouter.com`, …) and TLS enabled against secrets the cluster may not have. Configure it:

```yaml
ingress:
  className: nginx
  hosts: [api.example.com]          # replace the default production hosts
  tls:
    enabled: true
    secretName: cuberouter-tls     # must exist in the namespace
  docs:
    hosts: [docs.example.com]
    tls:
      enabled: true
      secretName: cuberouter-docs-tls
```

**Port-forward** (no Ingress):

```sh
kubectl -n cuberouter port-forward svc/cuberouter-app 3000:80   # app at http://localhost:3000
kubectl -n cuberouter port-forward svc/cuberouter-docs 8080:80  # docs at http://localhost:8080
```

## Verifying the install

```sh
helm status cuberouter -n cuberouter
kubectl -n cuberouter get pods
kubectl -n cuberouter get postgresclusters,redisreplications
```

Wait for the `PostgresCluster` to report `ClusterRunning` (its `status.phase`) and for the
`cuberouter-postgres-init` Job to complete — that Job syncs the app role password (the operator
auto-generates passwords; the app DSN uses the chart's). Then the app and docs pods pass their
`/api/status` probes:

```sh
curl -i http://localhost:3000/api/status
```

## Secrets and credentials

`secret.create=true` (default) makes the chart create the `<fullname>-secret` with these keys:

| Key | Meaning | Default |
|---|---|---|
| `SQL_DSN` | PostgreSQL connection string | always computed from the managed cluster |
| `REDIS_CONN_STRING` | Redis connection string | always computed from the managed failover |
| `SESSION_SECRET` | app session secret | random (32 chars) |
| `CRYPTO_SECRET` | app crypto secret | random (32 chars) |
| `REDIS_PASSWORD` | Redis password | random (24 chars) |
| `POSTGRES_PASSWORD` | PostgreSQL app role password | random (24 chars) |

Resolution order per key:

1. an explicit `secrets.<KEY>` value;
2. otherwise the value already stored in the existing secret in the cluster (read via `lookup`) —
   this is what keeps generated passwords **stable across `helm upgrade`**;
3. otherwise a freshly generated value.

> Passwords are random alphanumeric only by construction; if you set `REDIS_PASSWORD` or
> `POSTGRES_PASSWORD` explicitly they **must not contain single quotes or spaces** (the chart fails
> the render otherwise).

To fully manage the secret yourself (the 0.6.0 production pattern), set `secret.create=false` and
point `secret.existingSecret` at a pre-created secret that carries the same six keys — see the
[existing secret example](#2-using-an-existing-secret-060-pattern).

## Resource names and endpoints

`<fullname>` defaults to the **release name** (so release `cuberouter` → `cuberouter-app`,
matching the 0.6.0 production deployment); set `nameOverride` or `fullnameOverride` to change it.

| Resource | Name (`<f>` = `<fullname>`) |
|---|---|
| App Service / Deployment | `<f>-app` |
| App data / logs PVCs | `<f>-app-data`, `<f>-app-logs` |
| App HPA / PDB | `<f>-app-hpa`, `<f>-app-pdb` |
| Docs Service / Deployment | `<f>-docs` |
| ConfigMap / Secret | `<f>-config`, `<f>-secret` |
| App Ingress / Docs Ingress | `<f>-ingress`, `<f>-docs-ingress` |
| PostgresCluster CR | `<f>-postgres` |
| PG primary service (app connection target) | `<f>-postgres-primary:5432` |
| PG user / superuser secrets (operator-generated) | `<f>-postgres-pguser-<user>` / `<f>-postgres-pguser-postgres` |
| Post-install init Job | `<f>-postgres-init` |
| RedisReplication CR | `<f>-redis` (derived service names must stay ≤ 63 chars) |
| Redis master service (app connection target) | `<f>-redis-master:6379` |
| Sentinel headless service | `<f>-redis-s-hl:26379` |
| Redis operator auth secret | `<f>-redis-auth` |

Computed connection strings:

- PostgreSQL: `postgresql://<user>:<pw>@<f>-postgres-primary:5432/<db>` (defaults `cuberouter-user` / `cuberouter_db`)
- Redis: `redis://:<pw>@<f>-redis-master:6379` (operator-managed master service that follows the primary)

## Key values at a glance

| Values group | Highlights (defaults) |
|---|---|
| `config` | App env in the ConfigMap: `BATCH_UPDATE_ENABLED`, `ERROR_LOG_ENABLED`, `NODE_TYPE: master`, `PORT: 3000`, `TZ`; extend via `config.extra` |
| `secret` / `secrets` | see [Secrets and credentials](#secrets-and-credentials) |
| `cubeRouter` | `replicaCount: 2`, image, `service.port: 80`, persistence `/data` + `/app/logs`, probes on `/api/status`, `resources`, `envVars`, `nodeSelector` / `tolerations` |
| `docs` | `enabled: true`, own image, own Deployment + Service |
| `hpa` | `enabled: true`, 2→5 replicas at 70% CPU |
| `pdb` | `enabled: true`, `minAvailable: 1` for the app |
| `ingress` | `enabled: true`, `className: nginx`, production hosts + TLS secrets — **override for your cluster** |
| `postgresql` | `auth.database/username`, `postgresVersion: 16`, `replicas: 2`, `storage: 20Gi`, `image`, `resources`, `backups.*`, `pgBouncer.*`, `initJob.*` |
| `redis` | `image` (redis instances), `replicas: 2` (1 master + 1 replica), `sentinelReplicas: 3`, `sentinelImage`, `redisCustomConfig`, `persistence.*` |
| `postgresql-operator` | PGO control plane (always installed), image, `relatedImages` (defaults ship `POSTGRES_15`–`POSTGRES_18` from the v6.0.2 installer) |
| `redis-operator` | OpsTree control plane (always installed), image |

## HA details

### PostgreSQL (CrunchyData PGO)

- The operator auto-generates passwords and a `PostgresCluster` cannot pin them, so the chart's
  post-install/post-upgrade hook Job (`<f>-postgres-init`) connects as superuser, sets the app role
  password to the chart-generated one, ensures the database exists, and hands ownership of the
  `public` schema to the app role (required on PG15+). It is idempotent and safe to re-run.
- Set `postgresql.postgresVersion` to match the Postgres **major** version you run; when
  `postgresql.image` is empty the operator uses `postgresql-operator.relatedImages`
  (`POSTGRES_<version>`, default `POSTGRES_16`).
- pgBackRest full backups run on `postgresql.backups.schedule` (default weekly Sunday 02:00) into a
  dedicated repo volume. `postgresql.pgBouncer.enabled` optionally adds a pgBouncer proxy pool.
- Instance pods default to a soft pod anti-affinity spreading replicas across nodes.

### Redis (OpsTree redis-operator)

- The chart renders one `RedisReplication` CR with an **embedded sentinel set**: the operator runs
  the redis StatefulSet (`<f>-redis-0..N`) plus a sentinel StatefulSet (`<f>-redis-s-0..M`) that
  monitors master group `mymaster` and performs automatic failover.
- Native auth: the chart renders the `<f>-redis-auth` secret (`password` key) and the CR references
  it via `spec.kubernetesConfig.redisSecret`; the same password protects the redis nodes and the
  sentinels.
- The app connects to `<f>-redis-master:6379` — a ClusterIP service whose selector
  (`redis-role=master`) follows the currently promoted node.
- Keep the release name short enough that derived service names (e.g. `<f>-redis-additional`) stay
  ≤ 63 characters (the chart fails the render with a clear message if exceeded).
- Runtime tuning via `redis.redisCustomConfig` (list of `"<key> <value>"` pairs, applied with
  `CONFIG SET`).
- `redis.persistence.enabled: false` by default — enable it to survive pod restarts.

## Upgrading and rolling back

- `helm upgrade` is safe: generated passwords are re-read from the existing secret (`lookup`) so they
  stay stable, and the Crunchy init Job re-runs (idempotently) on every upgrade.
- Migrating from the 0.6.0 deployment: keep `secret.create=false` + the existing
  `existingSecret=cuberouter-secret` — its `SQL_DSN` / `REDIS_CONN_STRING` keep working unchanged,
  or switch to the managed clusters by importing your data first. This chart does **not** migrate
  data automatically.
- Roll back with `helm rollback cuberouter <revision> -n cuberouter`.

## Uninstalling

```sh
helm uninstall cuberouter -n cuberouter
```

`helm uninstall` removes the rendered workloads, Services, PVCs and the PostgresCluster /
RedisReplication CRs (and, via the operators, their underlying PVCs). The operator **CRDs** are
cluster-scoped and installed by the operator subcharts, so they remain after uninstall; remove them
separately if desired. **Back up your databases before uninstalling.**

## Validating the chart locally

```sh
helm lint helm/cuberouter-chart
helm template cuberouter helm/cuberouter-chart -n cuberouter          # full HA (default)
helm template cuberouter helm/cuberouter-chart -n cuberouter \
  --set secret.create=false --set secret.existingSecret=my-secret      # 0.6.0 pattern
```

Or run the bundled script that lints and renders every mode (it also asserts that a password
containing quotes or spaces is rejected):

```sh
helm/validate-chart.sh
```
