// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package install

const kibanaConfigYml = `  
server.name: kibana
server.host: "0.0.0.0"

elasticsearch.hosts: [ "http://elasticsearch:9200" ]
elasticsearch.username: elastic
elasticsearch.password: changeme
xpack.monitoring.ui.container.elasticsearch.enabled: true

xpack.fleet.enabled: true
xpack.fleet.registryUrl: "http://package-registry:8080"
xpack.fleet.agents.enabled: true
xpack.fleet.agents.elasticsearch.host: "http://elasticsearch:9200"
xpack.fleet.agents.kibana.host: "http://kibana:5601"
xpack.fleet.agents.tlsCheckDisabled: true
xpack.encryptedSavedObjects.encryptionKey: "12345678901234567890123456789012"
`
