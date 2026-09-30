# cuberouter Helm Chart

Installs **CubeRouter** (the AI gateway app) on Kubernetes together with its
dependency stores (PostgreSQL and Redis). The chart is **self-contained**: the two operator
subcharts are vendored under `charts/`, so it can be installed offline without `helm dependency update`.

| | |
|---|---|
| Chart | `cuberouter-chart` **1.1.0** |
| App version | `v1.0.0` |
| Components | CubeRouter app (user + admin docs bundled), PostgreSQL (CloudNativePG), Redis (OpsTree redis-operator) |

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
**base** runs a single replica everywhere (app 1, one PostgreSQL instance,
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

The defaults already install the app, the HA PostgreSQL cluster and the HA Redis
failover — plus both operator control planes (the PDB keeps at least one
app replica available):

```sh
helm install cuberouter ./helm/cuberouter-chart -n cuberouter --create-namespace
```

For a smaller (dev/test) cluster, use `deployMode: base` (single replica everywhere) and
override first:

```yaml
# my-values.yaml
deployMode: base              # app 1, 1 PG instance, 1 redis + 1 sentinel
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
`secret.existingSecret` at a pre-created secret carrying **all six keys** (see
[Secrets and credentials](#secrets-and-credentials)), plus the two Media Studio keys if you enable
reference uploads. The chart still creates the
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
  # add these two only when mediaStudio.enabled is true:
  #   --from-literal=MEDIA_STUDIO_S3_ACCESS_KEY='...' \
  #   --from-literal=MEDIA_STUDIO_S3_SECRET_KEY='...'

helm install cuberouter ./helm/cuberouter-chart -n cuberouter --create-namespace \
  --set secret.create=false \
  --set secret.existingSecret=cuberouter-secret
```

### 3. Media Studio reference uploads

Media Studio's **Image to image** mode lets a user attach reference images. The browser asks the app
for a presigned URL, uploads the bytes straight to your bucket, and the image provider fetches them
back through a second signed URL — the object store is never proxied through CubeRouter. Without a
bucket configured, text-to-image keeps working and the reference picker reports that uploads are not
configured.

```yaml
# media-studio.yaml
mediaStudio:
  enabled: true
  s3:
    endpoint: https://objects.example.com   # origin only, HTTPS
    bucket: cuberouter-media
    region: us-east-1
    access_key: "..."
    secret_key: "..."

# ...or keep the credentials out of the values file entirely:
#   secret.create=false + secret.existingSecret, carrying
#   MEDIA_STUDIO_S3_ACCESS_KEY / MEDIA_STUDIO_S3_SECRET_KEY
```

```sh
helm upgrade --install cuberouter ./helm/cuberouter-chart -n cuberouter \
  --create-namespace -f media-studio.yaml
```

The `s3` block is not secret except for `access_key` / `secret_key`: the first three land in the
ConfigMap, the credentials in the app Secret and reach the container through a `secretKeyRef`, so
they never appear in the Deployment spec.

> Pass the credentials with `-f` or `--set-file`, not `--set`: Helm splits `--set` values on commas,
> and a real S3 secret key is base64-ish and may contain one. Use `--set-string` if it has no comma.
> Do not commit a values file that carries these two keys.

On the bucket side:

- Grant the server key only `PutObject` and `GetObject` under the upload prefix
  (`media-studio/uploads/`). No bucket listing, no `ListBucket`, no public ACL.
- Configure CORS for the exact CubeRouter frontend origin, methods `PUT` and `GET`, and the
  `Content-Type` request header.
- Configure lifecycle deletion for `media-studio/uploads/` — a signed URL expiring does **not**
  delete the object.
- The bucket must be reachable by **both** the browser and the image provider. An internal-only
  endpoint will sign URLs that neither can fetch.

Two details worth knowing:

- **Rotating a credential needs a restart.** The first enable changes the Deployment and rolls the
  pods, but a later rotation only changes the Secret, and `secretKeyRef` env vars are read once at
  container start. Run `kubectl rollout restart deployment/<fullname>` after rotating.
- **A misconfiguration fails at install time.** The chart checks the settings before rendering, so
  `helm install` stops instead of silently disabling uploads. Two deliberate divergences from the
  server's own validator: the chart rejects plain `http` even for the loopback addresses the server
  accepts (use env vars directly for local development), and it tolerates a trailing `/` on the
  endpoint, which the server also accepts.

Full background, including the model-metadata labels that place a model in the Text-to-image or
Image-to-image list: [`docs/media-studio-integration.md`](../../docs/media-studio-integration.md).

## Accessing the app

The app service is `ClusterIP` on **port 80 → container 3000**, and it serves the bundled
documentation itself: user docs at `/docs/user/`, admin docs at `/docs/admin/`. Two common ways in:

**Ingress** — the chart ships an `nginx` Ingress by default, but with the production host names
(`cuberouter.com`, …) and TLS enabled against secrets the cluster may not have. Configure it:

```yaml
ingress:
  className: nginx
  hosts: [api.example.com]          # replace the default production hosts
  tls:
    enabled: true
    secretName: cuberouter-tls     # must exist in the namespace
```

Add an extra host to `ingress.hosts` (e.g. `docs.example.com`) if you want to reach the docs on
their own hostname — it is the same service, and the docs stay under `/docs/user/`.

**Port-forward** (no Ingress):

```sh
kubectl -n cuberouter port-forward svc/cuberouter 3000:80  # app + docs at http://localhost:3000
```

## Publishing under a URL prefix

The app can be published under a path such as `/cuberouter` instead of a dedicated hostname.
The Ingress strips the prefix and the app keeps serving from the root, so set `config.BASE_PATH`
to the same path the Ingress strips:

```yaml
config:
  BASE_PATH: /cuberouter
```

The value is a **path only** — `cuberouter`, `/cuberouter` and `/cuberouter/` are all accepted
and normalised. A full URL (`https://host/cuberouter`) is rejected and the app falls back to the
site root with a warning in the log. The default is empty, which is exactly the previous
behaviour.

The Ingress needs to rewrite the path for every request it forwards, including API and relay
traffic:

```yaml
ingress:
  annotations:
    nginx.ingress.kubernetes.io/use-regex: "true"
    nginx.ingress.kubernetes.io/rewrite-target: /$2
  hosts: [gateway.example.com]
  paths:
    - path: /cuberouter(/|$)(.*)
      pathType: ImplementationSpecific
```

Keep `config.BASE_PATH` and the Ingress `path` in sync — they are two halves of one setting, and
the chart cannot derive one from the other.

Two things deliberately stay **unprefixed**:

- the readiness/liveness probes (`/api/status`) — they hit the container port directly, never
  the Ingress;
- `ServerAddress` — it is an origin (scheme + host + port) because WebAuthn builds its list of
  allowed origins from it. The prefix is applied to paths by the app, so leave any path out of
  it.

Changing the prefix changes the URLs users see, so update the OAuth callback URLs registered
with each identity provider and `SESSION_COOKIE_TRUSTED_URL` in the same release.

## Verifying the install

```sh
helm status cuberouter -n cuberouter
kubectl -n cuberouter get pods
kubectl -n cuberouter get clusters,redisreplications
```

Wait for the `Cluster` to report `ClusterOnline` (its `status.phase`) and for its instances to
be ready. The app role and its chart-generated password exist from first start (the operator
applies `cuberouter-postgres-app-auth` during bootstrap). Until the stores are online the app
pods show `Init:x/2` (the `wait-for-postgres` / `wait-for-redis` init containers are still
waiting); once both stores are ready they start and pass their `/api/status` probes:

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
| `MEDIA_STUDIO_S3_ACCESS_KEY` | Media Studio reference-upload object-store key | `mediaStudio.s3.access_key`; only when `mediaStudio.enabled` |
| `MEDIA_STUDIO_S3_SECRET_KEY` | its secret | `mediaStudio.s3.secret_key`; only when `mediaStudio.enabled` |

The last two keys appear only when `mediaStudio.enabled` is true, and they are **excepted from the
resolution order below**: they are operator-supplied, never generated, and never read back from the
cluster. Generating one would mint a credential the object store rejects, and reading one back would
quietly resurrect a credential you just removed — both leave `GET /api/media-studio/config` reporting
`upload_enabled=true` while every upload 403s. Fail the render instead.

Resolution order per key:

1. an explicit `secrets.<KEY>` value;
2. otherwise the value already stored in the existing secret in the cluster (read via `lookup`) —
   this is what keeps generated passwords **stable across `helm upgrade`**;
3. otherwise a freshly generated value.

> Passwords are random alphanumeric only by construction; if you set `REDIS_PASSWORD` or
> `POSTGRES_PASSWORD` explicitly they **must not contain single quotes, spaces or URI special
> characters (`@ : / ? # [ ] %`)**, because they are embedded verbatim in the computed connection
> strings (the chart fails the render otherwise). `postgresql.auth.username` must match
> `[a-z0-9-]+` for the same reason.

To fully manage the secret yourself (the 0.6.0 production pattern), set `secret.create=false` and
point `secret.existingSecret` at a pre-created secret that carries the same six keys — plus
`MEDIA_STUDIO_S3_ACCESS_KEY` / `MEDIA_STUDIO_S3_SECRET_KEY` if you enable reference uploads — see the
[existing secret example](#2-using-an-existing-secret-060-pattern).

## Resource names and endpoints

`<fullname>` defaults to the **release name** (so release `cuberouter` → Deployment
+ Service `cuberouter`, CRs `cuberouter-postgres` / `cuberouter-redis`); set
`nameOverride` or `fullnameOverride` to change it.

| Resource | Name (`<f>` = `<fullname>`) |
|---|---|
| App Service / Deployment | `<f>` (same name, different kinds) |
| App data / logs PVCs | `<f>-app-data`, `<f>-app-logs` |
| App PDB | `<f>-app-pdb` (only in high mode) |
| ConfigMap / Secret | `<f>-config`, `<f>-secret` |
| App Ingress | `<f>-ingress` |
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
| `config` | App env in the ConfigMap: `BATCH_UPDATE_ENABLED`, `ERROR_LOG_ENABLED`, `NODE_TYPE: master`, `PORT: 3000`, `TZ`, `BASE_PATH` (empty = serve from the site root); extend via `config.extra` |
| `secret` / `secrets` | see [Secrets and credentials](#secrets-and-credentials) |
| `mediaStudio` | `enabled: false`; `s3.endpoint` / `s3.bucket` / `s3.region` (ConfigMap) and `s3.access_key` / `s3.secret_key` (Secret) for Image-to-image reference uploads |
| `cubeRouter` | `replicaCount: 2`, image, `service.port: 80`, persistence `/data` + `/app/logs`, probes on `/api/status`, `resources`, `envVars`, `waitForPostgres` / `waitForRedis` (init containers), `nodeSelector` / `tolerations` |
| `pdb` | `enabled: true`, `minAvailable: 1` for the app (not rendered in base mode) |
| `ingress` | `enabled: true`, `className: nginx`, production hosts + TLS secrets — **override for your cluster** |
| `postgresql` | `auth.database/username`, `image` (PostgreSQL 16.14), `replicas: 2`, `storage: 20Gi`, `resources`, `backups.*`, `pgBouncer.*` |
| `redis` | `image` (redis instances), `replicas: 2` (1 master + 1 replica), `sentinelReplicas: 3`, `sentinelImage`, `redisCustomConfig` / `redisCustomConfigHA`, `podSecurityContext` (`fsGroup: 1000`, [details below](#redis-opstree-redis-operator)), `persistence.*` |
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
  `postgresql.backups.schedule` (default weekly Sunday 02:00) taking a **cold volume snapshot**
  (CNPG's recommended mode: crash-consistent on its own, no WAL archiving required; the primary
  is fenced read-only only while the snapshot is taken) — the cluster must provide a default
  `VolumeSnapshotClass`; set `backups.enabled: false` if it does not.
- `postgresql.pgBouncer.enabled` (default off) adds an operator-managed pgbouncer `Pooler` in
  front of the cluster (service `<f>-postgres-pgbouncer:5432`, transaction pooling; the operator
  manages pooler authentication itself through its automated integration — the chart sets no
  `authQuery`, which would disable that setup and require an `authQuerySecret`).
- The operator spreads instance pods across nodes automatically (preferred pod anti-affinity).
  Superuser TCP access is disabled (the `postgres` role has no password).
- The operator admission webhooks run with `failurePolicy: Ignore` (subchart values): the
  Cluster CR is created in the same release as the operator, before the operator is serving;
  "Ignore" admits it during that window and validates normally afterwards (the operator
  injects its self-signed CA into the webhook configurations at startup).
- **Startup ordering:** the app pods run init containers (`cubeRouter.waitForPostgres` /
  `cubeRouter.waitForRedis`, both on by default) that retry every 5 seconds — `psql "SELECT 1"`
  with the app's own DSN for PostgreSQL and `redis-cli` AUTH + PING against the master for Redis —
  so the pods stay in the `Init` state until the primary accepts TLS connections, the app role
  authenticates, the app database exists and the master answers PING. The app container never
  starts against an unready store.

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
  `CONFIG SET` in every deployment mode). `redis.redisCustomConfigHA` takes the same shape but is
  rendered **only in `deployMode: high`** — it carries `min-replicas-to-write` /
  `min-replicas-max-lag`, which make the master refuse writes while no replica is connected.
  `deployMode: base` is a single node, so no replica is ever connected and applying them there
  would reject every write with `NOREPLICAS`.
- `redis.persistence.enabled: true` by default — Redis persists to a PVC (AOF/RDB) and survives
  pod restarts.
- `redis.podSecurityContext` is rendered into the CRD's `spec.podSecurityContext`, defaulting to
  `fsGroup: 1000` + `fsGroupChangePolicy: OnRootMismatch`. The opstree redis image runs as uid/gid
  1000, while a CSI-provisioned volume (ceph-rbd, EBS, …) is handed to the pod as `root:root`; the
  kubelet only rewrites that ownership when the pod declares an `fsGroup`. Without it the
  entrypoint cannot create `/data/appendonlydir` and the redis pod crash-loops on
  `Permission denied` — the sentinel pods are unaffected because they use `emptyDir`. Opt out with
  `--set redis.podSecurityContext=null` if your storage class handles ownership itself, or adjust
  `fsGroup` if you run a redis image with a different GID. A values-file `{}` will not clear the
  default: Helm deep-merges maps, so the chart's `fsGroup: 1000` survives it.

## Upgrading and rolling back

- **CRDs are not updated by Helm.** `helm upgrade` only creates the subcharts' CRDs when they do
  not exist yet; existing ones are skipped (and never deleted). When upgrading to a chart version
  that ships a newer operator (and therefore possibly changed CRDs), apply the new CRDs first:

  ```sh
  kubectl apply --server-side \
    -f helm/cuberouter-chart/charts/cloudnative-pg/crds/crds.yaml \
    -f helm/cuberouter-chart/charts/redis-operator/crds/crds.yaml
  ```

  (`--server-side` because two CNPG CRDs exceed the 256 KiB client-side-apply limit.)
- `helm upgrade` is safe: generated passwords are re-read from the existing secret (`lookup`) so they
  stay stable, and the CloudNativePG operator re-applies the app role password whenever the
  `cuberouter-postgres-app-auth` secret changes.
- Migrating from the 0.6.0 deployment: keep `secret.create=false` + the existing
  `existingSecret=cuberouter-secret` — its `SQL_DSN` / `REDIS_CONN_STRING` keep working unchanged,
  or switch to the managed clusters by importing your data first. This chart does **not** migrate
  data automatically.
- Upgrading a release that deployed the standalone docs workload: its Deployment, Service and
  Ingress are gone — the app image now bundles both docs sites and serves them at `/docs/user/`
  and `/docs/admin/`. An upgrade removes `<f>-docs` and `<f>-docs-ingress`; drop the `docs.*` and
  `ingress.docs.*` values (they are ignored) and point the documentation hostname at the app
  service, e.g. by adding it to `ingress.hosts`.
- Roll back with `helm rollback cuberouter <revision> -n cuberouter`.

## Uninstalling

```sh
helm uninstall cuberouter -n cuberouter
```

`helm uninstall` removes the rendered workloads, Services, PVCs, the Cluster /
RedisReplication CRs (and, via the operators, their underlying PVCs) and both operator
control planes with their cluster-scoped RBAC / webhook configurations. The operator
**CRDs** are packaged in the subcharts' `crds/` directory (and are also marked
`helm.sh/resource-policy: keep` upstream), so Helm installs them ahead of the rest of
the release and never deletes them on uninstall; remove them manually if desired, e.g.
`kubectl get crd -o name | grep postgresql.cnpg.io | xargs kubectl delete` (same for the
`redis.redis.opstreelabs.in` CRDs). Two caveats:

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

It additionally covers the `mediaStudio` block: that a default render emits no `MEDIA_STUDIO_S3_*`,
that an enabled render puts the settings in the ConfigMap and the credentials in the Secret (and
nowhere else in the manifest), that a pre-created secret still renders with `secret.create=false`,
and that a malformed endpoint, bucket, region, missing credential or conflicting `config.extra` entry
fails the render.
