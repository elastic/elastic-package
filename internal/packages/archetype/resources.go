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

// Input definitions

//go:embed _static/inputs/aws-cloudwatch.yml
var inputAwsCloudwatch string

//go:embed _static/inputs/aws-s3.yml
var inputAwsS3 string

//go:embed _static/inputs/azure-blob-storage.yml
var inputAzureBlobStorage string

//go:embed _static/inputs/azure-eventhub.yml
var inputAzureEventhub string

//go:embed _static/inputs/cel.yml
var inputCel string

//go:embed _static/inputs/entity-analytics.yml
var inputEntityAnalytics string

//go:embed _static/inputs/etw.yml
var inputEtw string

//go:embed _static/inputs/filestream.yml
var inputFilestream string

//go:embed _static/inputs/gcp-pubsub.yml
var inputGcpPubSub string

//go:embed _static/inputs/gcs.yml
var inputGcs string

//go:embed _static/inputs/http_endpoint.yml
var inputHttpEndpoint string

//go:embed _static/inputs/httpjson.yml
var inputHttpJson string

//go:embed _static/inputs/journald.yml
var inputJournald string

//go:embed _static/inputs/netflow.yml
var inputNetflow string

//go:embed _static/inputs/redis.yml
var inputRedis string

//go:embed _static/inputs/tcp.yml
var inputTcp string

//go:embed _static/inputs/udp.yml
var inputUdp string

//go:embed _static/inputs/winlog.yml
var inputWinlog string

var inputResources = []string{
	inputAwsCloudwatch,
	inputAwsS3,
	inputAzureBlobStorage,
	inputAzureEventhub,
	inputCel,
	inputEntityAnalytics,
	inputEtw,
	inputFilestream,
	inputGcpPubSub,
	inputGcs,
	inputHttpEndpoint,
	inputHttpJson,
	inputJournald,
	inputNetflow,
	inputRedis,
	inputTcp,
	inputUdp,
	inputWinlog,
}
