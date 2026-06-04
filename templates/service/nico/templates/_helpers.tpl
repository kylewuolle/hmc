{{- define "nico.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{- define "nico.labels" -}}
helm.sh/chart: {{ .Chart.Name }}-{{ .Chart.Version | replace "+" "_" }}
app.kubernetes.io/name: {{ include "nico.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{/*
Vault address — uses HTTP in devMode (no TLS), HTTPS otherwise.
Always resolves within the release namespace.
*/}}
{{- define "nico.vaultAddr" -}}
{{- if .Values.devMode -}}
http://vault.{{ .Release.Namespace }}.svc.cluster.local:8200
{{- else -}}
https://vault.{{ .Release.Namespace }}.svc.cluster.local:8200
{{- end -}}
{{- end }}

{{/*
PostgreSQL host within the release namespace.
*/}}
{{- define "nico.postgresHost" -}}
nico-postgresql.{{ .Release.Namespace }}.svc.cluster.local
{{- end }}
