package archetype

const packageManifestTemplate = `format_version: 1.0.0
name: {{.Name}}
title: "{{.Title}}"
version: {{.Version}}
license: {{.License}}
description: "{{.Description}}"
type: {{.Type}}
categories:
{{range $category := .Categories -}}
  - {{$category}}
{{end}}
release: {{.Release}}
conditions:
  kibana.version: "{{.Conditions.Kibana.Version}}"
screenshots:
  - src: /img/sample-screenshot.png
    title: Sample dashboard
    size: 1024x768
    type: image/png
icons:
  - src: /img/logo-elastic.svg
    title: Elastic logo
    size: 32x32
    type: image/svg+xml
policy_templates: ~
owner:
  github: {{.Owner.Github}}
`