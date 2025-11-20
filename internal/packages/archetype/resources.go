// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package archetype

import _ "embed"

// Common Package Templates

//go:embed _static/package-manifest.yml.tmpl
var packageManifestTemplate string

//go:embed _static/package-changelog.yml.tmpl
var packageChangelogTemplate string

//go:embed _static/package-docs-readme.md.tmpl
var packageDocsReadme string

//go:embed _static/fields-base.yml.tmpl
var fieldsBaseTemplate string

//go:embed _static/package-validation.yml.tmpl
var validationBaseTemplate string

// Images (logo and screenshot)

//go:embed _static/sampleIcon.svg
var packageImgSampleIcon []byte

// Screenshot: big Elastic logo (600x600 PNG)

//go:embed _static/sampleScreenshot.png.b64
var packageImgSampleScreenshot string

//go:embed _static/package-sample-event.json.tmpl
var packageSampleEvent string

// Input Package templates

//go:embed _static/input-package-agent-config.yml.tmpl
var inputAgentConfigTemplate string

// Data Stream templates

//go:embed _static/dataStream-agent-stream.yml.tmpl
var dataStreamAgentStreamTemplate string

//go:embed _static/dataStream-elasticsearch-ingest-pipeline.yml.tmpl
var dataStreamElasticsearchIngestPipelineTemplate string

//go:embed _static/dataStream-manifest.yml.tmpl
var dataStreamManifestTemplate string

// GetPackageDocsReadmeTemplate returns the embedded README template content
func GetPackageDocsReadmeTemplate() string {
	return packageDocsReadme
}
