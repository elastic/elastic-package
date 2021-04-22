// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package archetype

const packageManifestTemplate = `format_version: 1.0.0
name: {{.Manifest.Name}}
title: "{{.Manifest.Title}}"
version: {{.Manifest.Version}}
license: {{.Manifest.License}}
description: "{{.Manifest.Description}}"
type: {{.Manifest.Type}}
categories:
{{- range $category := .Manifest.Categories}}
  - {{$category}}
{{end -}}
release: {{.Manifest.Release}}
conditions:
  kibana.version: "{{.Manifest.Conditions.Kibana.Version}}"
screenshots:
  - src: /img/sample-screenshot.png
    title: Sample screenshot
    size: 600x600
    type: image/png
icons:
  - src: /img/sample-logo.svg
    title: Sample logo
    size: 32x32
    type: image/svg+xml
policy_templates: []
owner:
  github: {{.Manifest.Owner.Github}}
`
