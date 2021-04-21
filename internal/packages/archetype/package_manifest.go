// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

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
screenshots: ~
icons:
  - src: /img/sample-logo.svg
    title: Sample logo
    size: 32x32
    type: image/svg+xml
policy_templates: ~
owner:
  github: {{.Owner.Github}}
`
