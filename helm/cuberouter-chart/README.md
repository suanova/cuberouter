# cuberouter Helm Chart

Installs **CubeRouter** (the AI gateway app + its docs site) on Kubernetes together with its
dependency stores (PostgreSQL and Redis). The chart is **self-contained**: the two operator
subcharts are vendored under `charts/`, so it can be installed offline without `helm dependency update`.

| | |
|---|---|
| Chart | `cuberouter` **0.7.0** |
| App version | `v1.1.55-isuanova-agent-release` |
| Components | CubeRouter app, docs site, PostgreSQL (CloudNativePG), Redis (OpsTree redis-operator) |

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
| PostgreSQL | `Cluster` CR (CloudNativePG, built-in streaming replication + automatic failover, PostgreSQL 16, 2 instances) + CNPG control plane |
| Redis | `RedisReplication` CR (1 master + 1 replica, embedded 3-sentinel set) + OpsTree control plane |

The operator subcharts (CRDs + control-plane Deployments) are always installed
together with the stores.

`deployMode` is the replica-count preset: **high** (default) is the HA setup above;
**base** runs a single replica everywhere (app 1, docs 1, one PostgreSQL instance,
one Redis node + one sentinel), for dev/test clusters. In base mode the
per-component replica values are ignored (everything is 1) and the app PDB is
not rendered (a single replica has nothing to protect, and a PDB would block
rolling updates).

## Prerequisites

- **Helm 3.x** and **kubectl**
- A Kubernetes cluster with a **StorageClass** capable of `ReadWriteOnce` volumes (default sizes:
  app data 10Gi, app logs 10Gi, PostgreSQL 20Gi — each tunable) and, with the default
  `postgresql.backups.enabled: true`, a **default `VolumeSnapshotClass`** for volume-snapshot
  backups (disable backups if the cluster has none)
- Multi-node clusters recommended so HA replicas spread across hosts (the CloudNativePG operator
  applies a soft pod anti-affinity to the instance pods)
- Network access from the cluster to the container registries hosting the images (see next section)

## Images and registry access

| Component | Default image | Values key |
|---|---|---|
| App | `harbor.isuanova.com/suanova/cuberouter:latest` | `cubeRouter.image.repository` / `.tag` |
| Docs | `harbor.isuanova.com.cn/suanova/cuberouter:latest` | `docs.image.repository` / `.tag` |
| PostgreSQL instances | `ghcr.io/cloudnative-pg/postgresql:16.14-system-trixie` (tag encodes the PG version) | `postgresql.image` |
| Redis instances | `quay.io/opstree/redis:v7.0.15` | `redis.image.repository` / `.tag` |
| Redis sentinel | `quay.io/opstree/redis-sentinel:v7.0.15` | `redis.sentinelImage.repository` / `.tag` |
| CloudNativePG operator | `ghcr.io/cloudnative-pg/cloudnative-pg:1.30.0` | `cloudnative-pg.image.*` |
| OpsTree redis-operator | `quay.io/opstree/redis-operator:v0.26.0` | `redis-operator.image.*` |

All images are plain registry images; the chart does not manage image pull secrets — create one in
the namespace (or configure the nodes/registry) if your registry requires authentication.

## Installation

The release name is the base for every resource name (see
[Resource names and endpoints](#resource-names-and-endpoints)), so the examples below use
`cuberouter` to reproduce the canonical `cuberouter`, `cuberouter-redis`, etc. naming.

All installs start from the chart directory:

```sh
# from the repository root
helm install cuberouter ./helm/cuberouter-chart -n cuberouter --create-namespace [flags]
```

### 1. Full HA (default values)

The defaults already install the app, the docs site, the HA PostgreSQL cluster and the HA Redis
failover — plus both operator control planes (HPA is off by default; the PDB keeps at least one
app replica available):

```sh
helm install cuberouter ./helm/cuberouter-chart -n cuberouter --create-namespace
```

For a smaller (dev/test) cluster, use `deployMode: base` (single replica everywhere) and
override first:

```yaml
# my-values.yaml
deployMode: base              # app 1, docs 1, 1 PG instance, 1 redis + 1 sentinel
ingress:
  enabled: false              # you'll reach the app via port-forward instead
cubeRouter:
  persistence:
    data: { size: 5Gi }
    logs: { size: 2Gi }
postgresql:
  storage: 10Gi
  backups:
    enabled: false            # no VolumeSnapshotClass needed then
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
chart renders (`<fullname>-postgres-rw:5432`, `<fullname>-redis-master:6379`) with the
passwords you set.

```sh
kubectl create secret generic cuberouter-secret -n cuberouter \
  --from-literal=SQL_DSN='postgresql://cuberouter:pass@cuberouter-postgres-rw:5432/cuberouter?sslmode=require' \
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
kubectl -n cuberouter get clusters,redisreplications
```

Wait for the `Cluster` to report `ClusterOnline` (its `status.phase`) and for its instances to
be ready. The app role and its chart-generated password exist from first start (the operator
applies `cuberouter-postgres-app-auth` during bootstrap), so the app and docs pods pass their
`/api/status` probes as soon as the cluster is online:

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

`<fullname>` defaults to the **release name** (so release `cuberouter` → Deployment
`cuberouter`, service `cuberouter-app`, CRs `cuberouter-postgres` / `cuberouter-redis`);
set `nameOverride` or `fullnameOverride` to change it.

| Resource | Name (`<f>` = `<fullname>`) |
|---|---|
| App Service | `<f>-app` |
| App Deployment | `<f>` |
| App data / logs PVCs | `<f>-app-data`, `<f>-app-logs` |
| App HPA / PDB | `<f>-app-hpa`, `<f>-app-pdb` (PDB only in high mode) |
| Docs Service / Deployment | `<f>-docs` |
| ConfigMap / Secret | `<f>-config`, `<f>-secret` |
| App Ingress / Docs Ingress | `<f>-ingress`, `<f>-docs-ingress` |
| Cluster CR (CloudNativePG) | `<f>-postgres` |
| PG primary service (app connection target) | `<f>-postgres-rw:5432` |
| PG read services | `<f>-postgres-ro:5432` (replicas), `<f>-postgres-r:5432` (all ready), `<f>-postgres-any:5432` |
| App role credentials secret (chart-managed) | `<f>-postgres-app-auth` |
| Pooler CR / service (only when `pgBouncer.enabled`) | `<f>-postgres-pgbouncer` / `<f>-postgres-pgbouncer:5432` |
| ScheduledBackup CR (only when `backups.enabled`) | `<f>-postgres-backup` |
| RedisReplication CR | `<f>-redis` (derived service names must stay ≤ 63 chars) |
| Redis master service (app connection target) | `<f>-redis-master:6379` |
| Sentinel headless service | `<f>-redis-s-hl:26379` |
| Redis operator auth secret | `<f>-redis-auth` |

Computed connection strings:

- PostgreSQL: `postgresql://<user>:<pw>@<f>-postgres-rw:5432/<db>?sslmode=require` (defaults `cuberouter` / `cuberouter`; `sslmode=require` because the server certificate is self-signed — plain `host` connections would also be accepted)
- Redis: `redis://:<pw>@<f>-redis-master:6379` (operator-managed master service that follows the primary)

## Key values at a glance

| Values group | Highlights (defaults) |
|---|---|
| `deployMode` | `high` (HA replica counts) \| `base` (single replica everywhere; app PDB not rendered) |
| `config` | App env in the ConfigMap: `BATCH_UPDATE_ENABLED`, `ERROR_LOG_ENABLED`, `NODE_TYPE: master`, `PORT: 3000`, `TZ`; extend via `config.extra` |
| `secret` / `secrets` | see [Secrets and credentials](#secrets-and-credentials) |
| `cubeRouter` | `replicaCount: 2`, image, `service.port: 80`, persistence `/data` + `/app/logs`, probes on `/api/status`, `resources`, `envVars`, `nodeSelector` / `tolerations` |
| `docs` | `enabled: true`, own image, own Deployment + Service |
| `hpa` | `enabled: false`, 2→5 replicas at 70% CPU (minReplicas is 1 in base mode) |
| `pdb` | `enabled: true`, `minAvailable: 1` for the app (not rendered in base mode) |
| `ingress` | `enabled: true`, `className: nginx`, production hosts + TLS secrets — **override for your cluster** |
| `postgresql` | `auth.database/username`, `image` (PostgreSQL 16.14), `replicas: 2`, `storage: 20Gi`, `resources`, `backups.*`, `pgBouncer.*` |
| `redis` | `image` (redis instances), `replicas: 2` (1 master + 1 replica), `sentinelReplicas: 3`, `sentinelImage`, `redisCustomConfig`, `persistence.*` |
| `cloudnative-pg` | CNPG control plane (always installed), image, `resources` |
| `redis-operator` | OpsTree control plane (always installed), image |

## HA details

### PostgreSQL (CloudNativePG)

- The `Cluster` CR declares the app role through `spec.bootstrap.initdb` (`database` + `owner` +
  `secret`): the operator creates the role (`CREATE ROLE <user> LOGIN`), creates the database with
  `OWNER <user>`, and applies the password from the chart-rendered `<f>-postgres-app-auth`
  secret (basic-auth format). The chart password is therefore the live database password from
  first start — no init Job — and the operator re-applies it whenever that secret changes.
- Because the app user **owns** the database, it has full rights on its `public` schema
  (PostgreSQL 15+), so the app's DDL/migrations work without extra grants.
- Client connections use TLS with a self-signed server certificate (generated by the operator);
  the computed `SQL_DSN` carries `?sslmode=require` (TLS, no certificate verification). The
  default `pg_hba` also accepts plain TCP, so either works.
- `postgresql.image` is explicit — the tag encodes the PostgreSQL major version (default
  `16.14-system-trixie` = PostgreSQL 16.14). Pick a different tag to change the major version.
- Backups (`postgresql.backups.enabled`, default on): a `ScheduledBackup` CR runs on
  `postgresql.backups.schedule` (default weekly Sunday 02:00) using **volume snapshots** — the
  cluster must provide a default `VolumeSnapshotClass`; set `backups.enabled: false` if it does
  not.
- `postgresql.pgBouncer.enabled` (default off) adds an operator-managed pgbouncer `Pooler` in
  front of the cluster (service `<f>-postgres-pgbouncer:5432`, transaction pooling, clients
  authenticated against the role catalog).
- The operator spreads instance pods across nodes automatically (preferred pod anti-affinity).
  Superuser TCP access is disabled (the `postgres` role has no password).
- The operator admission webhooks run with `failurePolicy: Ignore` (subchart values): the
  Cluster CR is created in the same release as the operator, before the operator is serving;
  "Ignore" admits it during that window and validates normally afterwards (the operator
  injects its self-signed CA into the webhook configurations at startup).

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
  stay stable, and the CloudNativePG operator re-applies the app role password whenever the
  `cuberouter-postgres-app-auth` secret changes.
- Migrating from the 0.6.0 deployment: keep `secret.create=false` + the existing
  `existingSecret=cuberouter-secret` — its `SQL_DSN` / `REDIS_CONN_STRING` keep working unchanged,
  or switch to the managed clusters by importing your data first. This chart does **not** migrate
  data automatically.
- Roll back with `helm rollback cuberouter <revision> -n cuberouter`.

## Uninstalling

```sh
helm uninstall cuberouter -n cuberouter
```

`helm uninstall` removes the rendered workloads, Services, PVCs, the Cluster /
RedisReplication CRs (and, via the operators, their underlying PVCs) and both operator
control planes with their cluster-scoped RBAC / webhook configurations. The operator
**CRDs** are marked `helm.sh/resource-policy: keep` upstream (so that uninstalling one
release cannot wipe the API of other clusters sharing it) and therefore remain; remove
them manually if desired, e.g. `kubectl get crd -o name | grep postgresql.cnpg.io | xargs
kubectl delete` (same for the `redis.redis.opstreelabs.in` CRDs). Two caveats:

- deleting the CloudNativePG CRDs deletes **every** `Cluster` CR in the cluster, not only
  the one owned by this release — uninstall other CNPG releases first if they exist;
- operator-generated leftovers that are not part of the release (e.g. the `cnpg-webhook-cert`
  secret) stay in the namespace — remove them with `kubectl delete ns <namespace>`.

**Back up your databases before uninstalling.**

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
