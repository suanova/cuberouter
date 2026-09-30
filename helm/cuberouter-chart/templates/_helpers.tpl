{{/*
Chart name.

Deliberately NOT .Chart.Name: the chart is packaged and published as
"cuberouter-chart" (see Chart.yaml and .github/workflows/ci-chart.yml), but
this value feeds app.kubernetes.io/name in the Deployment selector, which is
immutable — so it must keep the historical "cuberouter" for existing releases
to remain upgradeable.
*/}}
{{- define "cuberouter.name" -}}
{{- default "cuberouter" .Values.nameOverride | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{/*
Fully qualified app name.
Follows the 0.6.0 convention: with no override, the release name is the base,
so release "cuberouter" yields "cuberouter-app", etc.
*/}}
{{- define "cuberouter.fullname" -}}
{{- if .Values.fullnameOverride -}}
{{- .Values.fullnameOverride | trunc 63 | trimSuffix "-" -}}
{{- else if .Values.nameOverride -}}
{{- .Values.nameOverride | trunc 63 | trimSuffix "-" -}}
{{- else -}}
{{- .Release.Name | trunc 63 | trimSuffix "-" -}}
{{- end -}}
{{- end -}}

{{/*
Common labels.
*/}}
{{- define "cuberouter.labels" -}}
app.kubernetes.io/name: {{ include "cuberouter.name" . }}
helm.sh/chart: {{ printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
app.kubernetes.io/instance: {{ .Release.Name }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
{{- end -}}

{{/*
Selector labels for a component.
Usage: {{ include "cuberouter.selectorLabels" (dict "root" . "component" "app") }}
*/}}
{{- define "cuberouter.selectorLabels" -}}
app.kubernetes.io/name: {{ include "cuberouter.name" .root }}
app.kubernetes.io/instance: {{ .root.Release.Name }}
app.kubernetes.io/component: {{ .component }}
{{- end -}}

{{/*
Replica count for a component. deployMode=base forces 1 everywhere; in
deployMode=high the per-component value (HA defaults) applies.
Usage: {{ include "cuberouter.replicas" (dict "root" . "ha" .Values.cubeRouter.replicaCount) }}
*/}}
{{- define "cuberouter.replicas" -}}
{{- if ne .root.Values.deployMode "base" -}}
{{- if ne .root.Values.deployMode "high" -}}
{{- fail (printf "deployMode must be 'base' or 'high', got %q" .root.Values.deployMode) -}}
{{- end -}}
{{- end -}}
{{- if eq .root.Values.deployMode "base" -}}
1
{{- else -}}
{{ .ha }}
{{- end -}}
{{- end -}}

{{/*
Fail the render when a value that is embedded verbatim in a URI connection
string (SQL_DSN / REDIS_CONN_STRING) would break the userinfo segment.
Generated passwords are alphanumeric and always pass; explicit values with
URI special characters are rejected (instead of being percent-encoded) so
the rendered DSNs stay predictable for psql, libpq and go-redis alike.
Usage: {{ include "cuberouter.assertDSNSafe" (dict "value" $pw "label" "secrets.X") }}
*/}}
{{- define "cuberouter.assertDSNSafe" -}}
{{- $v := .value -}}
{{- if or (contains "'" $v) (contains " " $v) (contains "@" $v) (contains ":" $v) (contains "/" $v) (contains "?" $v) (contains "#" $v) (contains "[" $v) (contains "]" $v) (contains "%" $v) -}}
{{- fail (printf "%s must not contain single quotes, spaces or URI special characters (@ : / ? # [ ] %%)" .label) -}}
{{- end -}}
{{- end -}}

{{/*
Fail the render when Media Studio reference uploads are enabled but their S3
settings are incomplete or malformed.

Mirrors StudioUploadConfig.Validate() in service/media_studio_upload.go: the
app treats an invalid configuration exactly like an absent one (it reports
upload_enabled=false and only reference uploads stop working), so a typo would
otherwise degrade silently at runtime instead of stopping the release.

The two credentials are only required when the chart owns the secret. A
secret.create=false release carries them in the pre-created secret named by
secret.existingSecret, so empty mediaStudio.s3 entries are legitimate there.

Usage: {{- include "cuberouter.assertMediaStudioS3" . -}}
*/}}
{{- define "cuberouter.assertMediaStudioS3" -}}
{{- if .Values.mediaStudio.enabled -}}
{{- $s3 := default (dict) .Values.mediaStudio.s3 -}}
{{- $endpoint := get $s3 "endpoint" | default "" | toString -}}
{{- $bucket := get $s3 "bucket" | default "" | toString -}}
{{- $region := get $s3 "region" | default "" | toString -}}

{{- if not $endpoint -}}
{{- fail "mediaStudio.s3.endpoint is required when mediaStudio.enabled=true" -}}
{{- else if not (regexMatch "^https://[^/?#@\\s]+/?$" $endpoint) -}}
{{- fail (printf "mediaStudio.s3.endpoint must be an https origin without a bucket, path, query or credentials; got %q" $endpoint) -}}
{{- end -}}

{{- if not $bucket -}}
{{- fail "mediaStudio.s3.bucket is required when mediaStudio.enabled=true" -}}
{{- else if not (regexMatch "^[a-z0-9][a-z0-9.-]{1,61}[a-z0-9]$" $bucket) -}}
{{- fail (printf "mediaStudio.s3.bucket must be a lowercase bucket name of 3-63 characters; got %q" $bucket) -}}
{{- else if or (contains ".." $bucket) (regexMatch "^[0-9]+\\.[0-9]+\\.[0-9]+\\.[0-9]+$" $bucket) -}}
{{- fail (printf "mediaStudio.s3.bucket must not contain a dot segment or look like an IP address; got %q" $bucket) -}}
{{- end -}}

{{- if not $region -}}
{{- fail "mediaStudio.s3.region is required when mediaStudio.enabled=true" -}}
{{- else if not (regexMatch "^[A-Za-z0-9][A-Za-z0-9._-]{0,127}$" $region) -}}
{{- fail (printf "mediaStudio.s3.region is not a valid signing region; got %q" $region) -}}
{{- end -}}

{{- if .Values.secret.create -}}
{{- if not (get $s3 "access_key" | default "" | toString) -}}
{{- fail "mediaStudio.s3.access_key is required when mediaStudio.enabled=true and secret.create=true (set secret.create=false to keep the credentials in secret.existingSecret instead)" -}}
{{- end -}}
{{- if not (get $s3 "secret_key" | default "" | toString) -}}
{{- fail "mediaStudio.s3.secret_key is required when mediaStudio.enabled=true and secret.create=true (set secret.create=false to keep the credentials in secret.existingSecret instead)" -}}
{{- end -}}
{{- end -}}

{{/* config.extra is injected through the same ConfigMap envFrom, so a
     hand-rolled MEDIA_STUDIO_S3_* entry would either duplicate a key the
     mediaStudio block already renders or bypass its validation. */}}
{{- range $key, $value := (default (dict) .Values.config.extra) -}}
{{- if hasPrefix "MEDIA_STUDIO_S3_" $key -}}
{{- fail (printf "config.extra.%s conflicts with the mediaStudio block; configure it under mediaStudio.s3 instead" $key) -}}
{{- end -}}
{{- end -}}
{{- end -}}
{{- end -}}

{{/*
App secret name (created or pre-existing).
*/}}
{{- define "cuberouter.secretName" -}}
{{- if .Values.secret.create -}}
{{- printf "%s-secret" (include "cuberouter.fullname" .) -}}
{{- else -}}
{{- required "secret.existingSecret is required when secret.create=false" .Values.secret.existingSecret -}}
{{- end -}}
{{- end -}}

{{/*
CloudNativePG Cluster CR name (HA mode).
*/}}
{{- define "cuberouter.postgresClusterName" -}}
{{- printf "%s-postgres" (include "cuberouter.fullname" .) -}}
{{- end -}}

{{/*
RedisReplication CR name (HA mode). Must stay <= 50 chars so the derived
service names (<name>-master, <name>-additional, ...) fit in 63.
*/}}
{{- define "cuberouter.redisName" -}}
{{- printf "%s-redis" (include "cuberouter.fullname" .) -}}
{{- end -}}

{{/*
PostgreSQL connection host:port (always the writable primary): the
CloudNativePG read-write service ("<name>-rw" selects the current primary
pod).
*/}}
{{- define "cuberouter.postgresAddr" -}}
{{- printf "%s-rw:5432" (include "cuberouter.postgresClusterName" .) -}}
{{- end -}}

{{/*
Redis connection host:port (always the writable node): the operator-managed
master service ("<name>-master" follows the current master pod via the
redis-role label).
*/}}
{{- define "cuberouter.redisAddr" -}}
{{- printf "%s-master:6379" (include "cuberouter.redisName" .) -}}
{{- end -}}
