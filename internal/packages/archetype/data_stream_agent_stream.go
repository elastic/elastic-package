// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package archetype

const dataStreamAgentStreamTemplate = `{{if eq .Manifest.Type "logs"}}paths:
` + "{{`{{#each paths as |path i|}}`}}" + `
  - ` + "{{`{{path}}`}}" + `
` + "{{`{{/each}}`}}" + `
exclude_files: [".gz$"]
processors:
  - add_locale: ~
{{else}}metricsets: ["sample_metricset"]
hosts:
` + "{{`{{#each hosts}}`}}" + `
  - ` + "{{`{{this}}`}}" + `
` + "{{`{{/each}}`}}" + `
period: ` + "{{`{{period}}`}}" + `
{{end}}`
