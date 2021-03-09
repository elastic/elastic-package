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

/*

Uncomment and use the commented definition of "stack" in case of emergency to define Docker image overrides
(stack.image_ref_overrides). The following sample defines overrides for the Elastic stack ver. 7.13.0-SNAPSHOT.
It's advised to use latest stable snapshots for the stack snapshot.

const applicationConfigurationYml = `stack:
  image_ref_overrides:
    7.13.0-SNAPSHOT:
      # Use stable image versions for Agent and Kibana
      elastic-agent: ` + elasticAgentImageName + `@sha256:76c294cf55654bc28dde72ce936032f34ad5f40c345f3df964924778b249e581
      kibana: ` + kibanaImageName + `@sha256:78ae3b1ca09efee242d2c77597dfab18670e984adb96c2407ec03fe07ceca4f6`
*/

const applicationConfigurationYml = `stack:
  image_ref_overrides:
`
