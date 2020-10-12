// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package install

const kibanaConfigYml = `  
server.name: kibana
server.host: "0"

elasticsearch.hosts: [ "http://elasticsearch:9200" ]
elasticsearch.username: elastic
elasticsearch.password: changeme
xpack.monitoring.ui.container.elasticsearch.enabled: true

xpack.fleet.registryUrl: "http://package-registry:8080"
xpack.fleet.enabled: true
xpack.fleet.elasticsearch.host: "http://elasticsearch:9200"
xpack.fleet.kibana.host: "http://kibana:5601"
xpack.fleet.tlsCheckDisabled: true

# TODO: Remove once https://github.com/elastic/kibana/issues/77613 is resolved.
xpack.fleet.pollingRequestTimeout: 60000

xpack.encryptedSavedObjects.encryptionKey: "12345678901234567890123456789012"
`
