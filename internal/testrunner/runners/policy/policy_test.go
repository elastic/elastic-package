// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package policy

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

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
			title: "clean expected",
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
