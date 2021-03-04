// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package install

const (
	elasticAgentImageName  = "docker.elastic.co/beats/elastic-agent"
	elasticsearchImageName = "docker.elastic.co/elasticsearch/elasticsearch"
	kibanaImageName        = "docker.elastic.co/kibana/kibana"
)

const applicationConfigYml = `stack:
  imageRefOverrides:
    7.13.0-SNAPSHOT:
      elastic-agent: ` + elasticAgentImageName + `@sha256:6182d3ebb975965c4501b551dfed2ddc6b7f47c05187884c62fe6192f7df4625
      elasticsearch: ` + elasticsearchImageName + `:7.13.0-FOO
      kibana: ` + kibanaImageName + `:7.13.0-BAR
`
