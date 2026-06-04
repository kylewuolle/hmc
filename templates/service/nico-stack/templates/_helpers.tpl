{{/*
Expand the name of the chart.
*/}}
{{- define "nico-stack.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{- define "nico-stack.fullname" -}}
{{- if .Values.fullnameOverride }}
{{- .Values.fullnameOverride | trunc 63 | trimSuffix "-" }}
{{- else }}
{{- printf "%s" (include "nico-stack.name" .) | trunc 63 | trimSuffix "-" }}
{{- end }}
{{- end }}

{{- define "nico-stack.labels" -}}
helm.sh/chart: {{ .Chart.Name }}-{{ .Chart.Version | replace "+" "_" }}
app.kubernetes.io/name: {{ include "nico-stack.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{/*
The Vault address reachable from within the cluster.
Uses HTTP in devMode (vault dev server has no TLS), HTTPS otherwise.
*/}}
{{- define "nico-stack.vaultAddr" -}}
{{- if .Values.devMode -}}
http://vault.vault.svc.cluster.local:8200
{{- else -}}
https://vault.vault.svc.cluster.local:8200
{{- end -}}
{{- end }}

{{/*
Resolved database host: use the explicit override or derive from the nico namespace.
*/}}
{{- define "nico-stack.dbHost" -}}
{{- if .Values.database.host }}
{{- .Values.database.host }}
{{- else }}
{{- printf "postgresql.%s.svc.cluster.local" .Values.nicoNamespace }}
{{- end }}
{{- end }}

{{/*
Strip trailing slash from a Vault mount path (for ConfigMap values).
*/}}
{{- define "nico-stack.stripSlash" -}}
{{- . | trimSuffix "/" }}
{{- end }}
