{{/*
Chart name.
*/}}
{{- define "cube-router.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{/*
Fully qualified app name.
Follows the 0.6.0 convention: with no override, the release name is the base,
so release "cube-router" yields "cube-router-app", etc.
*/}}
{{- define "cube-router.fullname" -}}
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
{{- define "cube-router.labels" -}}
app.kubernetes.io/name: {{ include "cube-router.name" . }}
helm.sh/chart: {{ printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
app.kubernetes.io/instance: {{ .Release.Name }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
{{- end -}}

{{/*
Selector labels for a component.
Usage: {{ include "cube-router.selectorLabels" (dict "root" . "component" "app") }}
*/}}
{{- define "cube-router.selectorLabels" -}}
app.kubernetes.io/name: {{ include "cube-router.name" .root }}
app.kubernetes.io/instance: {{ .root.Release.Name }}
app.kubernetes.io/component: {{ .component }}
{{- end -}}

{{/*
App secret name (created or pre-existing).
*/}}
{{- define "cube-router.secretName" -}}
{{- if .Values.secret.create -}}
{{- printf "%s-secret" (include "cube-router.fullname" .) -}}
{{- else -}}
{{- required "secret.existingSecret is required when secret.create=false" .Values.secret.existingSecret -}}
{{- end -}}
{{- end -}}

{{/*
PostgresCluster CR name (HA mode).
*/}}
{{- define "cube-router.postgresClusterName" -}}
{{- printf "%s-postgres" (include "cube-router.fullname" .) -}}
{{- end -}}

{{/*
RedisFailover CR name (HA mode). Must stay <= 48 chars (operator limit).
*/}}
{{- define "cube-router.redisFailoverName" -}}
{{- printf "%s-redis" (include "cube-router.fullname" .) -}}
{{- end -}}

{{/*
PostgreSQL connection host:port (always the writable primary).
*/}}
{{- define "cube-router.postgresAddr" -}}
{{- if .Values.postgresql.crunchy.enabled -}}
{{- printf "%s-primary:5432" (include "cube-router.postgresClusterName" .) -}}
{{- else if .Values.postgresql.enabled -}}
{{- printf "%s-postgresql:%v" (include "cube-router.fullname" .) (.Values.postgresql.service.port | int) -}}
{{- else -}}
{{- "" -}}
{{- end -}}
{{- end -}}

{{/*
Redis connection host:port (always the writable node).
Standalone: the StatefulSet service. HA: the operator-managed master service
("rf-rm-<name>" follows the current master pod).
*/}}
{{- define "cube-router.redisAddr" -}}
{{- if .Values.redis.operator.enabled -}}
{{- printf "rf-rm-%s:6379" (include "cube-router.redisFailoverName" .) -}}
{{- else if .Values.redis.enabled -}}
{{- printf "%s-redis:%v" (include "cube-router.fullname" .) (.Values.redis.service.port | int) -}}
{{- else -}}
{{- "" -}}
{{- end -}}
{{- end -}}

{{/*
Postgres image used for the Crunchy cluster and psql client images.
Empty crunchy.image = operator RELATED_IMAGE_POSTGRES_<ver>.
*/}}
{{- define "cube-router.postgresImage" -}}
{{- if .Values.postgresql.crunchy.image -}}
{{- .Values.postgresql.crunchy.image -}}
{{- else -}}
{{- $rel := index (index .Values "postgresql-operator").relatedImages (printf "POSTGRES_%v" .Values.postgresql.crunchy.postgresVersion) -}}
{{- if $rel -}}
{{- $rel -}}
{{- else -}}
{{- required (printf "postgresql.crunchy.image is required when postgresql-operator.relatedImages has no POSTGRES_%v entry" .Values.postgresql.crunchy.postgresVersion) "" -}}
{{- end -}}
{{- end -}}
{{- end -}}

{{/*
Image pull secrets from global.
*/}}
{{- define "cube-router.imagePullSecrets" -}}
{{- toYaml .Values.global.imagePullSecrets -}}
{{- end -}}
