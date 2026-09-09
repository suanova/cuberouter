{{/*
Chart name.
*/}}
{{- define "cuberouter.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" -}}
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
