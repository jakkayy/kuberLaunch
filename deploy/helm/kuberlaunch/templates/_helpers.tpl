{{- define "kuberlaunch.labels" -}}
app.kubernetes.io/name: kuberlaunch
app.kubernetes.io/instance: {{ .Release.Name }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{- define "kuberlaunch.image" -}}
{{ .image.repository }}:{{ .image.tag | default .root.Chart.AppVersion }}
{{- end }}

{{- define "kuberlaunch.secretsDir" -}}/etc/kuberlaunch/secrets{{- end }}
