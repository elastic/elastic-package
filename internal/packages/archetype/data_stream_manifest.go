// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package archetype

const dataStreamManifestTemplate = `title: "{{.Manifest.Title}}"
type: {{.Manifest.Type}}
streams:{{if eq .Manifest.Type "logs" }}
  - input: logfile
    title: Sample logs
    description: Collect sample logs
    vars:
      - name: paths
        type: text
        title: Paths
        multi: true
        default:
          - /var/log/*.log
{{else}}
  - input: sample/metrics
    title: Sample metrics
    description: Collect sample metrics
    vars:
      - name: period
        type: text
        title: Period
        default: 10s
{{ if .Manifest.Elasticsearch }}
elasticsearch:
{{ if .Manifest.Elasticsearch.SourceMode }}
  source_mode: {{ .Manifest.Elasticsearch.SourceMode }}
{{- end}}
{{ if .Manifest.Elasticsearch.IndexMode }}
  index_mode: {{ .Manifest.Elasticsearch.IndexMode }}
{{- end}}
{{- end}}
{{- end}}
`
