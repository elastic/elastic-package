// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package archetype

const packageChangelogTemplate = `# newer versions go on top
- version: "{{.Manifest.Version}}"
  changes:
    - description: Initial draft of the package
      type: enhancement
      link: https://github.com/elastic/integrations/pull/0 # FIXME Replace with the real PR link
`
