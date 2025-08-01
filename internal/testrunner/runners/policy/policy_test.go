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
        uuid-for-permissions-on-related-indices:
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
        uuid-for-permissions-on-related-indices:
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
        uuid-for-permissions-on-related-indices:
            indices:
                - names:
                    - logs-*-*
                  privileges:
                    - auto_configure
                    - create_doc
receivers:
    httpcheck/componentid:
        collection_interval: 1m
        targets:
            - endpoints:
                - https://epr.elastic.co
              method: GET
secret_references: []
service:
    pipelines:
        logs:
            receivers:
                - httpcheck/componentid

`,
			found: `
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
                    - logs-*-*
                  privileges:
                    - auto_configure
                    - create_doc
receivers:
    httpcheck/b0f518d6-4e2d-4c5d-bda7-f9808df537b7:
        collection_interval: 1m
        targets:
            - endpoints:
                - https://epr.elastic.co
              method: GET
secret_references: []
service:
    pipelines:
        logs:
            receivers:
                - httpcheck/b0f518d6-4e2d-4c5d-bda7-f9808df537b7

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
