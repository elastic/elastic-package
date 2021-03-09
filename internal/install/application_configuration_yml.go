// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package install

const (
	elasticAgentImageName  = "docker.elastic.co/beats/elastic-agent"
	elasticsearchImageName = "docker.elastic.co/elasticsearch/elasticsearch"
	kibanaImageName        = "docker.elastic.co/kibana/kibana"
)

const applicationConfigurationYmlFile = "config.yml"

const applicationConfigurationYml = `stack:
  image_ref_overrides:
`
