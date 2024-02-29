{{/*namespace*/}}
{{- define "nm.namespaceOverride" -}}
  {{- if .Values.namespaceOverride -}}
    {{- .Values.namespaceOverride -}}
  {{- else -}}
    {{- .Release.Namespace -}}
  {{- end -}}
{{- end -}}


{{- define "global.imageRegistry" -}}
{{- $registry := default .Values.global.imageRegistry .Values.imageRegistryOverride }}
  {{- if $registry -}}
    {{- printf "%s/" $registry -}}
  {{- end -}}
{{- end -}}


{{- define "common.notificationmanager.nodeSelectors" -}}
{{- $selector := default .Values.global.nodeSelector .Values.notificationmanager.nodeSelector }}
{{- toYaml $selector | nindent 4 }}
{{- end -}}

{{- define "common.operator.nodeSelectors" -}}
{{- $selector := default .Values.global.nodeSelector .Values.operator.nodeSelector }}
{{- toYaml $selector | nindent 8 }}
{{- end -}}