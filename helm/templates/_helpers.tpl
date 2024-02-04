{{/*namespace*/}}
{{- define "nm.namespaceOverride" -}}
  {{- if .Values.namespaceOverride -}}
    {{- .Values.namespaceOverride -}}
  {{- else -}}
    {{- .Release.Namespace -}}
  {{- end -}}
{{- end -}}


{{- define "global.imageRegistry" -}}
  {{- if .Values.global.imageRegistry -}}
    {{- printf "%s/" .Values.global.imageRegistry -}}
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