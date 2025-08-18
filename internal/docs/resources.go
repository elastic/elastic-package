// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package docs

import _ "embed"

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
