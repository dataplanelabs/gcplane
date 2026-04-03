{{/*
Expand the name of the chart.
*/}}
{{- define "gcplane.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create a default fully qualified app name.
*/}}
{{- define "gcplane.fullname" -}}
{{- if .Values.fullnameOverride }}
{{- .Values.fullnameOverride | trunc 63 | trimSuffix "-" }}
{{- else }}
{{- $name := default .Chart.Name .Values.nameOverride }}
{{- if contains $name .Release.Name }}
{{- .Release.Name | trunc 63 | trimSuffix "-" }}
{{- else }}
{{- printf "%s-%s" .Release.Name $name | trunc 63 | trimSuffix "-" }}
{{- end }}
{{- end }}
{{- end }}

{{/*
Create chart label value.
*/}}
{{- define "gcplane.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Common labels.
*/}}
{{- define "gcplane.labels" -}}
helm.sh/chart: {{ include "gcplane.chart" . }}
{{ include "gcplane.selectorLabels" . }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{/*
Selector labels.
*/}}
{{- define "gcplane.selectorLabels" -}}
app.kubernetes.io/name: {{ include "gcplane.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{/*
ServiceAccount name.
*/}}
{{- define "gcplane.serviceAccountName" -}}
{{- if .Values.serviceAccount.create }}
{{- default (include "gcplane.fullname" .) .Values.serviceAccount.name }}
{{- else }}
{{- default "default" .Values.serviceAccount.name }}
{{- end }}
{{- end }}

{{/*
ConfigMap name for the manifest (existing or chart-managed).
*/}}
{{- define "gcplane.configMapName" -}}
{{- if .Values.manifestConfigMap }}
{{- .Values.manifestConfigMap }}
{{- else }}
{{- printf "%s-manifest" (include "gcplane.fullname" .) }}
{{- end }}
{{- end }}

{{/*
Container image with tag fallback to appVersion.
*/}}
{{- define "gcplane.image" -}}
{{- $tag := .Values.image.tag | default .Chart.AppVersion }}
{{- printf "%s:%s" .Values.image.repository $tag }}
{{- end }}
