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
xpack.fleet.agents.fleet_server.hosts: ["http://fleet-server:8220"]

xpack.encryptedSavedObjects.encryptionKey: "12345678901234567890123456789012"

# CLOUD CONFIG
xpack.fleet.packages:
  - name: fleet_server
    version: latest
  - name: system
    version: latest

xpack.fleet.agentPolicies:
  # Cloud Agent policy
  - name: Elastic Cloud agent policy
    description: Default agent policy for agents hosted on Elastic Cloud
    is_managed: true
    id: elastic-agent-on-cloud
    is_default_fleet_server: false
    monitoring_enabled: []
    package_policies:
      - package:
          name: fleet_server
        name: Fleet Server
        inputs:
          - type: fleet-server
            vars:
              - name: host
                value: 0.0.0.0
                frozen: true

  # Default policy
  - name: Default policy
    namespace: default
    description: Default agent policy created by Kibana
    is_default: true
    is_managed: false
    monitoring_enabled:
      - logs
      - metrics
    package_policies:
      - name: system-1
        package:
          name: system

  # Default Fleet Server policy
  - name: Default Fleet Server policy
    description: Default Fleet Server agent policy created by Kibana
    is_managed: false
    is_default_fleet_server: true
    is_default: false
    monitoring_enabled:
      - logs
      - metrics
    package_policies:
      - package:
          name: fleet_server
        name: fleet_server-1
`
