package archetype

const packageChangelogTemplate = `# newer versions go on top
- version: "{{.Version}}"
  changes:
    - description: Initial draft of the package
      type: enhancement
      link: https://github.com/elastic/integrations/pull/0 # FIXME Replace with the real PR link
`