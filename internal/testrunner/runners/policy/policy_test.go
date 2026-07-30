// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package policy

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCleanPolicy(t *testing.T) {
	cases := []struct {
		title    string
		policy   string
		expected string
	}{
		{
			title: "clean single exporter endpoint",
			policy: `
exporters:
    elasticsearch/default:
        endpoints:
            - https://abc123def.elastic.cloud:443
`,
			expected: `exporters:
    elasticsearch/componentid-0:
        endpoints:
            - https://elasticsearch:9200
`,
		},
		{
			title: "clean multiple exporter endpoints",
			policy: `
exporters:
    elasticsearch/default:
        endpoints:
            - https://my-deployment.elastic-cloud.com:443
    elasticsearch/secondary:
        endpoints:
            - http://localhost:9200
`,
			expected: `exporters:
    elasticsearch/componentid-0:
        endpoints:
            - https://elasticsearch:9200
    elasticsearch/componentid-1:
        endpoints:
            - https://elasticsearch:9200
`,
		},
		{
			title: "clean exporter with multiple endpoints in list",
			policy: `
exporters:
    elasticsearch/default:
        endpoints:
            - https://node1.elastic.cloud:443
            - https://node2.elastic.cloud:443
            - http://node3.example.com:9200
`,
			expected: `exporters:
    elasticsearch/componentid-0:
        endpoints:
            - https://elasticsearch:9200
            - https://elasticsearch:9200
            - https://elasticsearch:9200
`,
		},
		// beatsauth fields injected by Fleet in OTel policies since 9.4.0.
		{
			title: "strip auth from exporter, keep endpoints",
			policy: `
exporters:
    elasticsearch/default:
        endpoints:
            - https://abc123def.elastic.cloud:443
        auth:
            authenticator: beatsauth/default
`,
			expected: `exporters:
    elasticsearch/componentid-0:
        endpoints:
            - https://elasticsearch:9200
`,
		},
		{
			title: "strip beatsauth entries from extensions, keep non-beatsauth",
			policy: `
extensions:
    beatsauth/default:
        ssl:
            ca_trusted_fingerprint: abc123
    health_check/default:
        endpoint: 0.0.0.0:13133
`,
			expected: `extensions:
    health_check/componentid-0:
        endpoint: 0.0.0.0:13133
`,
		},
		{
			title: "remove extensions entirely when only beatsauth entries remain",
			policy: `
extensions:
    beatsauth/default:
        ssl:
            ca_trusted_fingerprint: abc123
`,
			expected: `{}
`,
		},
		{
			title: "strip beatsauth entries from service.extensions, keep others",
			policy: `
service:
    extensions:
        - beatsauth/default
        - health_check/default
    pipelines:
        logs/default:
            receivers:
                - otlp/default
`,
			expected: `service:
    extensions:
        - health_check/default
    pipelines:
        logs/componentid-0:
            receivers:
                - otlp/default
`,
		},
		{
			title: "remove service.extensions entirely when only beatsauth entries remain",
			policy: `
service:
    extensions:
        - beatsauth/default
    pipelines:
        logs/default:
            receivers:
                - otlp/default
`,
			expected: `service:
    pipelines:
        logs/componentid-0:
            receivers:
                - otlp/default
`,
		},
		{
			title: "strip all beatsauth fields injected by Fleet on 9.4.0+",
			policy: `
exporters:
    elasticsearch/default:
        endpoints:
            - https://abc123def.elastic.cloud:443
        auth:
            authenticator: beatsauth/default
extensions:
    beatsauth/default:
        ssl:
            ca_trusted_fingerprint: abc123
    health_check/default:
        endpoint: 0.0.0.0:13133
service:
    extensions:
        - beatsauth/default
        - health_check/default
`,
			expected: `exporters:
    elasticsearch/componentid-0:
        endpoints:
            - https://elasticsearch:9200
extensions:
    health_check/componentid-0:
        endpoint: 0.0.0.0:13133
service:
    extensions:
        - health_check/componentid-0
`,
		},
	}

	for _, c := range cases {
		t.Run(c.title, func(t *testing.T) {
			cleaned, err := cleanPolicy([]byte(c.policy), policyEntryFilters)
			assert.NoError(t, err)
			assert.Equal(t, c.expected, string(cleaned))
		})
	}
}

func TestComparePolicies(t *testing.T) {
	cases := []struct {
		title    string
		expected string
		found    string
		equal    bool
		fail     bool
	}{
		{
			title: "same content",
			expected: `
foo: "2e19c1c4-185b-11ef-a7fc-43855f39047f"
`,
			found: `
foo: "2e19c1c4-185b-11ef-a7fc-43855f39047f"
`,
			equal: true,
		},
		{
			title: "ignored ids",
			expected: `
id: "2e19c1c4-185b-11ef-a7fc-43855f39047f"
`,
			found: `
id: "8ddb2260-185b-11ef-9bb0-6753eb8e2b83"
`,
			equal: true,
		},
		{
			title: "invalid JSON",
			expected: `
id: "2e19c1c4-185b-11ef-a7fc-43855f39047f"
`,
			found: `
404 Not Found
`,
			fail: true,
		},
		{
			title: "invalid JSON",
			expected: `
id: "2e19c1c4-185b-11ef-a7fc-43855f39047f"
`,
			found: `
404 Not Found
`,
			fail: true,
		},
		{
			title: "clean namespaces if empty",
			expected: `
`,
			found: `
namespaces: []
`,
			equal: true,
		},
		{
			title: "clean namespaces if default",
			expected: `
`,
			found: `
namespaces: [default]
`,
			equal: true,
		},
		{
			title: "clean namespaces only if empty",
			expected: `
namespaces: []
`,
			found: `
namespaces: [foo]
`,
			equal: false,
		},
		{
			title: "clean suffix in package policy name",
			expected: `
inputs:
    - data_stream:
        namespace: ep
      meta:
        package:
            name: test_package
      name: test-name
      streams: []
      type: test_package/logs
      use_output: default
`,
			found: `
inputs:
    - data_stream:
        namespace: ep
      meta:
        package:
            name: test_package
      name: test-name-12345
`,
			equal: false,
		},
		{
			title: "clean expected",
			expected: `
inputs:
    - data_stream:
        namespace: ep
      meta:
        package:
            name: sql_input
      name: test-mysql-sql_input-12345
      streams:
        - data_stream:
            dataset: sql_input.sql_query
            elasticsearch:
                dynamic_dataset: true
                dynamic_namespace: true
            type: metrics
          driver: mysql
          hosts:
            - root:test@tcp(localhost:3306)/
          metricsets:
            - query
          period: 10s
          sql_query: SHOW GLOBAL STATUS LIKE 'Innodb_%';
          sql_response_format: variables
          password: ${SECRET_0}
      type: sql/metrics
      use_output: default
output_permissions:
    default:
        _elastic_agent_checks:
            cluster:
                - monitor
        _elastic_agent_monitoring:
            indices: []
        8d024b11-4e82-4192-8e7f-be71d1b13aac:
            indices:
                - names:
                    - metrics-*-*
                  privileges:
                    - auto_configure
                    - create_doc
secret_references:
    - {}
`,
			found: `
id: 8fb82eb0-185c-11ef-b65b-9b66b5f5b53c
revision: 2
agent: {}
fleet: {}
outputs: {}
inputs:
    - id: package/9d111234-185c-11ef-9f2d-ebbd90f9ac83
      revision: 2
      data_stream:
        namespace: ep
      meta:
        package:
            name: sql_input
            version: 1.0.0
            release: ga
            policy_template: sql_input
      name: test-mysql-sql_input
      package_policy_id: b2775cd2-185c-11ef-bf70-b7bd5adaa788
      streams:
        - data_stream:
            dataset: sql_input.sql_query
            elasticsearch:
                dynamic_dataset: true
                dynamic_namespace: true
            type: metrics
          driver: mysql
          hosts:
            - root:test@tcp(localhost:3306)/
          metricsets:
            - query
          period: 10s
          sql_query: SHOW GLOBAL STATUS LIKE 'Innodb_%';
          sql_response_format: variables
          password: ${SECRET_0}
      type: sql/metrics
      use_output: default
namespaces: []
output_permissions:
    default:
        _elastic_agent_checks:
            cluster:
                - monitor
        _elastic_agent_monitoring:
            indices: []
        c02bd2c2-185c-11ef-8e9b-b7fa6a98a253:
            indices:
                - names:
                    - metrics-*-*
                  privileges:
                    - auto_configure
                    - create_doc
secret_references:
    - id: asdaddsaads
`,
			equal: true,
		},
		{
			title: "clean but different",
			expected: `
inputs:
    - data_stream:
        namespace: ep
      meta:
        package:
            name: sql_input
      name: test-mysql-sql_input
      streams:
        - data_stream:
            dataset: sql_input.sql_query
            elasticsearch:
                dynamic_dataset: true
                dynamic_namespace: true
            type: metrics
          driver: mysql
          hosts:
            - root:test@tcp(localhost:3306)/
          metricsets:
            - query
          period: 10s
          sql_query: SHOW GLOBAL STATUS LIKE 'Innodb_%';
          sql_response_format: variables
          password: ${SECRET_0}
      type: sql/metrics
      use_output: default
output_permissions:
    default:
        _elastic_agent_checks:
            cluster:
                - monitor
        _elastic_agent_monitoring:
            indices: []
        bfe4f402-df02-4673-8a71-fd5b29f1e2f3:
            indices:
                - names:
                    - metrics-*-*
                  privileges:
                    - auto_configure
                    - create_doc
secret_references:
    - {}
`,
			found: `
id: 8fb82eb0-185c-11ef-b65b-9b66b5f5b53c
revision: 2
agent: {}
fleet: {}
outputs: {}
inputs:
    - id: package/9d111234-185c-11ef-9f2d-ebbd90f9ac83
      revision: 2
      data_stream:
        namespace: ep
      meta:
        package:
            name: sql_input
            version: 1.0.0
      name: test-mysql-sql_input
      package_policy_id: b2775cd2-185c-11ef-bf70-b7bd5adaa788
      streams:
        - data_stream:
            dataset: sql_input.sql_query
            elasticsearch:
                dynamic_dataset: true
                dynamic_namespace: true
            type: metrics
          driver: mysql
          hosts:
            - root:test@tcp(localhost:3306)/
          metricsets:
            - query
          period: 10s
          sql_query: SHOW GLOBAL STATUS LIKE 'Innodb_%';
          sql_response_format: table
          password: ${SECRET_0}
      type: sql/metrics
      use_output: default
output_permissions:
    default:
        _elastic_agent_checks:
            cluster:
                - monitor
        _elastic_agent_monitoring:
            indices: []
        c02bd2c2-185c-11ef-8e9b-b7fa6a98a253:
            indices:
                - names:
                    - metrics-*-*
                  privileges:
                    - auto_configure
                    - create_doc
secret_references:
    - id: asdaddsaads
`,
			equal: false,
		},
		{
			title: "otel ids",
			expected: `
inputs: []
output_permissions:
    default:
        _elastic_agent_checks:
            cluster:
                - monitor
        _elastic_agent_monitoring:
            indices: []
        05c98f91-203c-44a9-bee7-dd621c9bd37e:
            indices:
                - names:
                    - logs-*-*
                  privileges:
                    - auto_configure
                    - create_doc
extensions:
    health_check/31c94f44-214a-4778-8a36-acc2634096f7: {}
exporters:
    elasticsearch/default:
        endpoints:
          - https://something.elastic.cloud:443
processors:
    batch/11c35ad0-4351-49d4-9c78-fa679ce9d950:
        send_batch_size: 10
        timeout: 1s
    batch/e6e379c5-6446-4090-af10-a9e5f8fc4640:
        send_batch_size: 10000
        timeout: 10s
    transform/otelcol-check-9987a1b9-3a12-43e8-a0a2-e83fa9deebfd-otelcol-httpcheck-check-9987a1b9-3a12-43e8-a0a2-e83fa9deebfd-routing:
        metric_statements:
            - context: datapoint
              statements:
                - set(attributes["data_stream.type"], "metrics")
                - set(attributes["data_stream.dataset"], "httpcheck.check")
                - set(attributes["data_stream.namespace"], "ep")
connectors:
  forward: {}
receivers:
    httpcheck/4bae34b3-8f66-49c1-b04f-d58af1b5f743:
        collection_interval: 1m
        targets:
            - endpoints:
                - https://epr.elastic.co
              method: GET
    httpcheck/otelcol-check-9987a1b9-3a12-43e8-a0a2-e83fa9deebfd-otelcol-httpcheck-check-9987a1b9-3a12-43e8-a0a2-e83fa9deebfd:
        collection_interval: 2m
        targets:
            - endpoints:
                - https://epr.elastic.co
              method: GET
secret_references: []
service:
    extensions:
        - health_check/31c94f44-214a-4778-8a36-acc2634096f7
    pipelines:
        metrics/otelcol-check-9987a1b9-3a12-43e8-a0a2-e83fa9deebfd-otelcol-httpcheck-check-9987a1b9-3a12-43e8-a0a2-e83fa9deebfd:
            receivers:
                - >-
                  httpcheck/otelcol-check-9987a1b9-3a12-43e8-a0a2-e83fa9deebfd-otelcol-httpcheck-check-9987a1b9-3a12-43e8-a0a2-e83fa9deebfd
            processors:
                - >-
                  transform/otelcol-check-9987a1b9-3a12-43e8-a0a2-e83fa9deebfd-otelcol-httpcheck-check-9987a1b9-3a12-43e8-a0a2-e83fa9deebfd-routing
        logs:
            receivers:
                - httpcheck/4bae34b3-8f66-49c1-b04f-d58af1b5f743
            processors:
                - batch/11c35ad0-4351-49d4-9c78-fa679ce9d950
                - batch/e6e379c5-6446-4090-af10-a9e5f8fc4640

`,
			found: `
inputs: []
output_permissions:
    default:
        _elastic_agent_checks:
            cluster:
                - monitor
        aeb4d606-2d90-4b41-b231-27bfad6dea09:
            indices:
                - names:
                    - logs-*-*
                  privileges:
                    - auto_configure
                    - create_doc
        _elastic_agent_monitoring:
            indices: []
extensions:
    health_check/4391d954-1ffe-4014-a256-5eda78a71829: {}
exporters:
    elasticsearch/fleet-default-output:
        endpoints:
          - https://sfca8c1a9178b40b28c73f0f1d8a08267.elastic.cloud:443
processors:
    batch/567fce7a-ff2e-4a6c-a32a-0abb4671b39b:
        send_batch_size: 10
        timeout: 1s
    batch/8ec6ee99-2176-4231-9668-908069c77784:
        send_batch_size: 10000
        timeout: 10s
    transform/otelcol-check-12bd7179-ea83-494b-9f2c-5bf818cd6a77-otelcol-httpcheck-check-12bd7179-ea83-494b-9f2c-5bf818cd6a77-routing:
        metric_statements:
            - context: datapoint
              statements:
                - set(attributes["data_stream.type"], "metrics")
                - set(attributes["data_stream.dataset"], "httpcheck.check")
                - set(attributes["data_stream.namespace"], "ep")
connectors:
  forward: {}
receivers:
    httpcheck/b0f518d6-4e2d-4c5d-bda7-f9808df537b7:
        collection_interval: 1m
        targets:
            - endpoints:
                - https://epr.elastic.co
              method: GET
    httpcheck/otelcol-check-12bd7179-ea83-494b-9f2c-5bf818cd6a77-otelcol-httpcheck-check-12bd7179-ea83-494b-9f2c-5bf818cd6a77:
        collection_interval: 2m
        targets:
            - endpoints:
                - https://epr.elastic.co
              method: GET
secret_references: []
service:
    extensions:
        - health_check/4391d954-1ffe-4014-a256-5eda78a71829
    pipelines:
        metrics/otelcol-check-12bd7179-ea83-494b-9f2c-5bf818cd6a77-otelcol-httpcheck-check-12bd7179-ea83-494b-9f2c-5bf818cd6a77:
            receivers:
                - >-
                  httpcheck/otelcol-check-12bd7179-ea83-494b-9f2c-5bf818cd6a77-otelcol-httpcheck-check-12bd7179-ea83-494b-9f2c-5bf818cd6a77
            processors:
                - >-
                  transform/otelcol-check-12bd7179-ea83-494b-9f2c-5bf818cd6a77-otelcol-httpcheck-check-12bd7179-ea83-494b-9f2c-5bf818cd6a77-routing
        logs:
            receivers:
                - httpcheck/b0f518d6-4e2d-4c5d-bda7-f9808df537b7
            processors:
                - batch/567fce7a-ff2e-4a6c-a32a-0abb4671b39b
                - batch/8ec6ee99-2176-4231-9668-908069c77784

`,
			equal: true,
		},
		{
			title: "otel hardcode expected ids",
			expected: `
inputs: []
output_permissions:
    default:
        _elastic_agent_checks:
            cluster:
                - monitor
        _elastic_agent_monitoring:
            indices: []
        05c98f91-203c-44a9-bee7-dd621c9bd37e:
            indices:
                - names:
                    - logs-*-*
                  privileges:
                    - auto_configure
                    - create_doc
extensions:
    health_check/componentid-0: {}
processors:
    batch/componentid-0:
        send_batch_size: 10
        timeout: 1s
    batch/componentid-1:
        send_batch_size: 10000
        timeout: 10s
    transform/componentid-2:
        metric_statements:
            - context: datapoint
              statements:
                - set(attributes["data_stream.type"], "metrics")
                - set(attributes["data_stream.dataset"], "httpcheck.check")
                - set(attributes["data_stream.namespace"], "ep")
connectors:
  forward: {}
receivers:
    httpcheck/componentid-0:
        collection_interval: 1m
        targets:
            - endpoints:
                - https://epr.elastic.co
              method: GET
    httpcheck/componentid-1:
        collection_interval: 2m
        targets:
            - endpoints:
                - https://epr.elastic.co
              method: GET
secret_references: []
service:
    extensions:
        - health_check/componentid-0
    pipelines:
        metrics/componentid-0:
            receivers:
                - >-
                  httpcheck/componentid-1
            processors:
                - >-
                  transform/componentid-2
        logs:
            receivers:
                - httpcheck/componentid-0
            processors:
                - batch/componentid-0
                - batch/componentid-1

`,
			found: `
inputs: []
output_permissions:
    default:
        _elastic_agent_checks:
            cluster:
                - monitor
        aeb4d606-2d90-4b41-b231-27bfad6dea09:
            indices:
                - names:
                    - logs-*-*
                  privileges:
                    - auto_configure
                    - create_doc
        _elastic_agent_monitoring:
            indices: []
extensions:
    health_check/4391d954-1ffe-4014-a256-5eda78a71828: {}
processors:
    batch/567fce7a-ff2e-4a6c-a32a-0abb4671b39b:
        send_batch_size: 10
        timeout: 1s
    batch/8ec6ee99-2176-4231-9668-908069c77784:
        send_batch_size: 10000
        timeout: 10s
    transform/otelcol-check-12bd7179-ea83-494b-9f2c-5bf818cd6a77-otelcol-httpcheck-check-12bd7179-ea83-494b-9f2c-5bf818cd6a77-routing:
        metric_statements:
            - context: datapoint
              statements:
                - set(attributes["data_stream.type"], "metrics")
                - set(attributes["data_stream.dataset"], "httpcheck.check")
                - set(attributes["data_stream.namespace"], "ep")
connectors:
  forward: {}
receivers:
    httpcheck/b0f518d6-4e2d-4c5d-bda7-f9808df537b7:
        collection_interval: 1m
        targets:
            - endpoints:
                - https://epr.elastic.co
              method: GET
    httpcheck/otelcol-check-12bd7179-ea83-494b-9f2c-5bf818cd6a77-otelcol-httpcheck-check-12bd7179-ea83-494b-9f2c-5bf818cd6a77:
        collection_interval: 2m
        targets:
            - endpoints:
                - https://epr.elastic.co
              method: GET
secret_references: []
service:
    extensions:
        - health_check/4391d954-1ffe-4014-a256-5eda78a71828
    pipelines:
        metrics/otelcol-check-12bd7179-ea83-494b-9f2c-5bf818cd6a77-otelcol-httpcheck-check-12bd7179-ea83-494b-9f2c-5bf818cd6a77:
            receivers:
                - >-
                  httpcheck/otelcol-check-12bd7179-ea83-494b-9f2c-5bf818cd6a77-otelcol-httpcheck-check-12bd7179-ea83-494b-9f2c-5bf818cd6a77
            processors:
                - >-
                  transform/otelcol-check-12bd7179-ea83-494b-9f2c-5bf818cd6a77-otelcol-httpcheck-check-12bd7179-ea83-494b-9f2c-5bf818cd6a77-routing
        logs:
            receivers:
                - httpcheck/b0f518d6-4e2d-4c5d-bda7-f9808df537b7
            processors:
                - batch/567fce7a-ff2e-4a6c-a32a-0abb4671b39b
                - batch/8ec6ee99-2176-4231-9668-908069c77784

`,
			equal: true,
		},
		{
			title: "clean exporter endpoints",
			expected: `
exporters:
    elasticsearch/default:
        endpoints:
            - https://elasticsearch:9200
`,
			found: `
exporters:
    elasticsearch/default:
        endpoints:
            - https://abc123def.elastic.cloud:443
`,
			equal: true,
		},
		{
			title: "clean multiple exporter endpoints",
			expected: `
exporters:
    elasticsearch/default:
        endpoints:
            - https://elasticsearch:9200
    elasticsearch/secondary:
        endpoints:
            - https://elasticsearch:9200
`,
			found: `
exporters:
    elasticsearch/default:
        endpoints:
            - https://my-deployment-12345.elastic-cloud.com:443
    elasticsearch/secondary:
        endpoints:
            - http://localhost:9200
`,
			equal: true,
		},
		{
			title: "clean http exporter endpoint",
			expected: `
exporters:
    elasticsearch/default:
        endpoints:
            - https://elasticsearch:9200
`,
			found: `
exporters:
    elasticsearch/default:
        endpoints:
            - http://insecure-es.example.com:9200
`,
			equal: true,
		},
		{
			title: "clean policy ensuring ordering",
			found: `
id: f3032029-fa01-4072-98f1-ce7d2b51cbf2
revision: 2
outputs:
  default:
    type: elasticsearch
    hosts: &ref_0
      - https://elasticsearch:9200
    ssl.ca_trusted_fingerprint: ccccc
    preset: latency
fleet:
  hosts:
    - https://fleet-server:8220
output_permissions:
  default:
    _elastic_agent_monitoring:
      indices: []
    _elastic_agent_checks:
      cluster:
        - monitor
    5e216c73-dcbf-444a-953b-50672c9df682:
      indices:
        - names:
            - metrics-*-*
          privileges: &ref_1
            - auto_configure
            - create_doc
        - names:
            - logs-*-*
          privileges: *ref_1
agent:
  download:
    sourceURI: https://artifacts.elastic.co/downloads/
  monitoring:
    enabled: false
    logs: false
    metrics: false
    traces: false
  features: {}
  protection:
    enabled: false
    uninstall_token_hash: bbbb
    signing_key: >-
      aaaaaaa
inputs: []
signed:
  data: >-
    dddd
  signature: >-
    1234567890
receivers:
  sqlserver/otelcol-sqlserverreceiver-5e216c73-dcbf-444a-953b-50672c9df682-otelcol-sql_server_input_otel-sqlserverreceiver:
    collection_interval: 10s
    initial_delay: 1s
    events:
      db.server.query_sample:
        enabled: false
      db.server.top_query:
        enabled: false
processors:
  resourcedetection/system/otelcol-sqlserverreceiver-5e216c73-dcbf-444a-953b-50672c9df682-otelcol-sql_server_input_otel-sqlserverreceiver:
    detectors:
      - system
  transform/otelcol-sqlserverreceiver-5e216c73-dcbf-444a-953b-50672c9df682-otelcol-sql_server_input_otel-sqlserverreceiver-routing:
    metric_statements:
      - context: datapoint
        statements:
          - set(attributes["data_stream.type"], "metrics")
          - set(attributes["data_stream.dataset"], "sqlserverreceiver")
          - set(attributes["data_stream.namespace"], "ep")
    log_statements:
      - context: log
        statements:
          - set(attributes["data_stream.type"], "logs")
          - set(attributes["data_stream.dataset"], "sqlserverreceiver")
          - set(attributes["data_stream.namespace"], "ep")
service:
  pipelines:
    metrics/otelcol-sqlserverreceiver-5e216c73-dcbf-444a-953b-50672c9df682-otelcol-sql_server_input_otel-sqlserverreceiver:
      receivers:
        - >-
          sqlserver/otelcol-sqlserverreceiver-5e216c73-dcbf-444a-953b-50672c9df682-otelcol-sql_server_input_otel-sqlserverreceiver
      processors:
        - >-
          resourcedetection/system/otelcol-sqlserverreceiver-5e216c73-dcbf-444a-953b-50672c9df682-otelcol-sql_server_input_otel-sqlserverreceiver
        - >-
          transform/otelcol-sqlserverreceiver-5e216c73-dcbf-444a-953b-50672c9df682-otelcol-sql_server_input_otel-sqlserverreceiver-routing
      exporters:
        - forward
    logs/otelcol-sqlserverreceiver-5e216c73-dcbf-444a-953b-50672c9df682-otelcol-sql_server_input_otel-sqlserverreceiver:
      receivers:
        - >-
          sqlserver/otelcol-sqlserverreceiver-5e216c73-dcbf-444a-953b-50672c9df682-otelcol-sql_server_input_otel-sqlserverreceiver
      processors:
        - >-
          resourcedetection/system/otelcol-sqlserverreceiver-5e216c73-dcbf-444a-953b-50672c9df682-otelcol-sql_server_input_otel-sqlserverreceiver
        - >-
          transform/otelcol-sqlserverreceiver-5e216c73-dcbf-444a-953b-50672c9df682-otelcol-sql_server_input_otel-sqlserverreceiver-routing
      exporters:
        - forward
    metrics:
      receivers:
        - forward
      exporters:
        - elasticsearch/default
    logs:
      receivers:
        - forward
      exporters:
        - elasticsearch/default
connectors:
  forward: {}
exporters:
  elasticsearch/default:
    endpoints: *ref_0
secret_references: []
namespaces:
  - default
`,
			expected: `
connectors:
    forward: {}
exporters:
    elasticsearch/componentid-0:
        endpoints:
            - https://elasticsearch:9200
inputs: []
output_permissions:
    default:
        _elastic_agent_checks:
            cluster:
                - monitor
        _elastic_agent_monitoring:
            indices: []
        uuid-for-permissions-on-related-indices:
            indices:
                - names:
                    - metrics-*-*
                  privileges:
                    - auto_configure
                    - create_doc
                - names:
                    - logs-*-*
                  privileges:
                    - auto_configure
                    - create_doc
processors:
    resourcedetection/componentid-0:
        detectors:
            - system
    transform/componentid-1:
        log_statements:
            - context: log
              statements:
                - set(attributes["data_stream.type"], "logs")
                - set(attributes["data_stream.dataset"], "sqlserverreceiver")
                - set(attributes["data_stream.namespace"], "ep")
        metric_statements:
            - context: datapoint
              statements:
                - set(attributes["data_stream.type"], "metrics")
                - set(attributes["data_stream.dataset"], "sqlserverreceiver")
                - set(attributes["data_stream.namespace"], "ep")
receivers:
    sqlserver/componentid-0:
        collection_interval: 10s
        events:
            db.server.query_sample:
                enabled: false
            db.server.top_query:
                enabled: false
        initial_delay: 1s
secret_references: []
service:
    pipelines:
        logs:
            exporters:
                - elasticsearch/componentid-0
            receivers:
                - forward
        logs/componentid-0:
            exporters:
                - forward
            processors:
                - resourcedetection/componentid-0
                - transform/componentid-1
            receivers:
                - sqlserver/componentid-0
        metrics:
            exporters:
                - elasticsearch/componentid-0
            receivers:
                - forward
        metrics/componentid-1:
            exporters:
                - forward
            processors:
                - resourcedetection/componentid-0
                - transform/componentid-1
            receivers:
                - sqlserver/componentid-0
`,
			equal: true,
		},
		{
			// Verifies the normalization does not hide genuine differences between policies
			// that have matching extension IDs but different extension bodies. The expected
			// file uses a bare extension ref (mixed state); the found policy uses the full
			// component-ID form. Only the htpasswd file path differs — that must still be
			// detected as a difference.
			title: "different extension body is still detected as different after bare-ref normalization",
			expected: `
extensions:
    basicauth/componentid-0:
        htpasswd:
            file: /etc/otel/.htpasswd
service:
    extensions:
        - basicauth
`,
			found: `
extensions:
    basicauth/componentid-0:
        htpasswd:
            file: /etc/otel/.other-htpasswd
service:
    extensions:
        - basicauth/componentid-0
`,
			equal: false,
		},
		// --- end-to-end mixed-state positive: bare refs in expected, suffixed refs in found ---
		{
			// Reproduces the exact state of the otlp_input_otel expected files: extension map
			// key already has the componentid suffix, but service.extensions and
			// auth.authenticator still use the bare type name. Found policy (from 9.5.0+) has
			// the suffix everywhere. Same body → should compare as equal.
			title: "mixed-state expected (bare refs) equals fully-suffixed found with same body",
			expected: `
extensions:
    basicauth/componentid-0:
        htpasswd:
            file: /etc/otel/.htpasswd
receivers:
    otlp/abc:
        protocols:
            grpc:
                auth:
                    authenticator: basicauth
            http:
                auth:
                    authenticator: basicauth
service:
    extensions:
        - basicauth
    pipelines:
        traces/abc:
            receivers:
                - otlp/abc
`,
			found: `
extensions:
    basicauth/componentid-0:
        htpasswd:
            file: /etc/otel/.htpasswd
receivers:
    otlp/abc:
        protocols:
            grpc:
                auth:
                    authenticator: basicauth/componentid-0
            http:
                auth:
                    authenticator: basicauth/componentid-0
service:
    extensions:
        - basicauth/componentid-0
    pipelines:
        traces/abc:
            receivers:
                - otlp/abc
`,
			equal: true,
		},
		{
			// Multi-extension variant of the mixed-state scenario, matching the
			// test-auth-multi.yml case: two extensions with different componentid suffixes
			// in the map keys; expected file references both with bare type names.
			title: "multi-extension mixed-state expected equals fully-suffixed found",
			expected: `
extensions:
    basicauth/componentid-1:
        htpasswd:
            file: /etc/otel/.htpasswd
    bearertokenauth/componentid-0:
        token: mytoken
receivers:
    otlp/abc:
        protocols:
            grpc:
                auth:
                    authenticator: basicauth
            http:
                auth:
                    authenticator: basicauth
service:
    extensions:
        - basicauth
        - bearertokenauth
    pipelines:
        traces/abc:
            receivers:
                - otlp/abc
`,
			found: `
extensions:
    basicauth/componentid-1:
        htpasswd:
            file: /etc/otel/.htpasswd
    bearertokenauth/componentid-0:
        token: mytoken
receivers:
    otlp/abc:
        protocols:
            grpc:
                auth:
                    authenticator: basicauth/componentid-1
            http:
                auth:
                    authenticator: basicauth/componentid-1
service:
    extensions:
        - basicauth/componentid-1
        - bearertokenauth/componentid-0
    pipelines:
        traces/abc:
            receivers:
                - otlp/abc
`,
			equal: true,
		},
		// --- negatives: genuine differences must still be detected ---
		{
			// Different extension types must not be treated as equivalent even when both
			// sides use bare names or componentid suffixes.
			title: "different extension type is detected as different",
			expected: `
extensions:
    basicauth/componentid-0:
        htpasswd:
            file: /etc/otel/.htpasswd
service:
    extensions:
        - basicauth
`,
			found: `
extensions:
    bearertokenauth/componentid-0:
        token: mytoken
service:
    extensions:
        - bearertokenauth/componentid-0
`,
			equal: false,
		},
		{
			// Found policy has an extra extension not present in the expected file.
			title: "extra extension in found is detected as different",
			expected: `
extensions:
    basicauth/componentid-0:
        htpasswd:
            file: /etc/otel/.htpasswd
service:
    extensions:
        - basicauth
`,
			found: `
extensions:
    basicauth/componentid-0:
        htpasswd:
            file: /etc/otel/.htpasswd
    bearertokenauth/componentid-1:
        token: mytoken
service:
    extensions:
        - basicauth/componentid-0
        - bearertokenauth/componentid-1
`,
			equal: false,
		},
		{
			// In a two-extension setup, if the authenticator assignments are swapped between
			// receivers the difference must still be caught after normalization.
			title: "swapped authenticator assignments between receivers are detected as different",
			expected: `
extensions:
    basicauth/componentid-0:
        htpasswd:
            file: /etc/otel/.htpasswd
    bearertokenauth/componentid-1:
        token: mytoken
receivers:
    otlp/grpc:
        protocols:
            grpc:
                auth:
                    authenticator: basicauth
    otlp/http:
        protocols:
            http:
                auth:
                    authenticator: bearertokenauth
service:
    extensions:
        - basicauth
        - bearertokenauth
`,
			found: `
extensions:
    basicauth/componentid-0:
        htpasswd:
            file: /etc/otel/.htpasswd
    bearertokenauth/componentid-1:
        token: mytoken
receivers:
    otlp/grpc:
        protocols:
            grpc:
                auth:
                    authenticator: bearertokenauth/componentid-1
    otlp/http:
        protocols:
            http:
                auth:
                    authenticator: basicauth/componentid-0
service:
    extensions:
        - basicauth/componentid-0
        - bearertokenauth/componentid-1
`,
			equal: false,
		},
	}

	for _, c := range cases {
		t.Run(c.title, func(t *testing.T) {
			diff, err := comparePolicies([]byte(c.expected), []byte(c.found))
			if c.fail {
				assert.Error(t, err)
				return
			}

			assert.NoError(t, err)
			if c.equal {
				assert.Empty(t, diff)
			} else {
				assert.NotEmpty(t, diff)
			}
		})
	}
}
