// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package policy

import (
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"github.com/elastic/elastic-package/internal/common"
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

func TestNormalizePolicyToCanonical(t *testing.T) {
	t.Run("rewrites OTel component IDs and references", func(t *testing.T) {
		policy := `
exporters:
  elasticsearch/default:
    endpoints:
      - https://elasticsearch:9200
receivers:
  zipkin/otelcol-zipkinreceiver-uuid-here:
    endpoint: 0.0.0.0:9411
service:
  pipelines:
    traces/custom-pipeline:
      receivers:
        - zipkin/otelcol-zipkinreceiver-uuid-here
      exporters:
        - elasticsearch/default
`
		out, err := normalizePolicyToCanonical([]byte(policy))
		assert.NoError(t, err)
		t.Log(string(out))
		assert.Contains(t, string(out), "elasticsearch/componentid-0")
		assert.Contains(t, string(out), "zipkin/componentid-0")
		assert.Contains(t, string(out), "traces/componentid-0")
		// References should be updated
		assert.Contains(t, string(out), "- zipkin/componentid-0")
		assert.Contains(t, string(out), "- elasticsearch/componentid-0")
	})

	t.Run("order-independent: same components different key order normalize to same result", func(t *testing.T) {
		policyA := `
exporters:
  elasticsearch/second:
    endpoints: ["b"]
  elasticsearch/first:
    endpoints: ["a"]
  elasticsearch/a5ae742d-5b47-4d5e-9511-969df92fcf3a:
    endpoints: ["d"]
`
		policyB := `
exporters:
  elasticsearch/sixth:
    endpoints: ["a"]
  elasticsearch/fourth:
    endpoints: ["b"]
  elasticsearch/2577857f-918e-405d-b657-a4dbdbf02a2f:
    endpoints: ["d"]
`
		outA, err := normalizePolicyToCanonical([]byte(policyA))
		assert.NoError(t, err)
		outB, err := normalizePolicyToCanonical([]byte(policyB))
		assert.NoError(t, err)
		assert.Equal(t, string(outA), string(outB), "equivalent policies with different key order should normalize to same YAML")
	})

	// Reproduces https://github.com/elastic/elastic-package/issues/3630:
	// Fleet (since https://github.com/elastic/kibana/pull/270771) suffixes extension keys
	// for cross-stream uniqueness, and references those extensions from service.extensions[]
	// and from auth.authenticator inside receiver bodies.
	t.Run("normalizes suffixed extension id referenced from service.extensions", func(t *testing.T) {
		policy := `
extensions:
  apikeyauth/otelcol-elasticapmintakereceiver-2ad3f316-95ec-4749-955d-bb680ccb3a6f-otelcol-elasticapm_input_otel-elasticapmintakereceiver-2ad3f316-95ec-4749-955d-bb680ccb3a6f:
    api_key: abc
service:
  extensions:
    - apikeyauth/otelcol-elasticapmintakereceiver-2ad3f316-95ec-4749-955d-bb680ccb3a6f-otelcol-elasticapm_input_otel-elasticapmintakereceiver-2ad3f316-95ec-4749-955d-bb680ccb3a6f
`
		out, err := normalizePolicyToCanonical([]byte(policy))
		assert.NoError(t, err)
		t.Log(string(out))
		assert.Contains(t, string(out), "apikeyauth/componentid-0")
		assert.Contains(t, string(out), "- apikeyauth/componentid-0")
	})

	t.Run("normalizes suffixed extension id referenced from auth.authenticator", func(t *testing.T) {
		policy := `
extensions:
  apikeyauth/otelcol-elasticapmintakereceiver-2ad3f316-95ec-4749-955d-bb680ccb3a6f-otelcol-elasticapm_input_otel-elasticapmintakereceiver-2ad3f316-95ec-4749-955d-bb680ccb3a6f:
    api_key: abc
receivers:
  elasticapmintakereceiver/2ad3f316-95ec-4749-955d-bb680ccb3a6f:
    endpoint: localhost:8200
    auth:
      authenticator: apikeyauth/otelcol-elasticapmintakereceiver-2ad3f316-95ec-4749-955d-bb680ccb3a6f-otelcol-elasticapm_input_otel-elasticapmintakereceiver-2ad3f316-95ec-4749-955d-bb680ccb3a6f
`
		out, err := normalizePolicyToCanonical([]byte(policy))
		assert.NoError(t, err)
		t.Log(string(out))
		assert.Contains(t, string(out), "apikeyauth/componentid-0")
		assert.Contains(t, string(out), "elasticapmintakereceiver/componentid-0")
		assert.Contains(t, string(out), "authenticator: apikeyauth/componentid-0")
	})

	t.Run("normalizes suffixed extension id referenced from both service.extensions and auth.authenticator", func(t *testing.T) {
		policy := `
extensions:
  apikeyauth/otelcol-elasticapmintakereceiver-2ad3f316-95ec-4749-955d-bb680ccb3a6f-otelcol-elasticapm_input_otel-elasticapmintakereceiver-2ad3f316-95ec-4749-955d-bb680ccb3a6f:
    api_key: abc
receivers:
  elasticapmintakereceiver/2ad3f316-95ec-4749-955d-bb680ccb3a6f:
    endpoint: localhost:8200
    auth:
      authenticator: apikeyauth/otelcol-elasticapmintakereceiver-2ad3f316-95ec-4749-955d-bb680ccb3a6f-otelcol-elasticapm_input_otel-elasticapmintakereceiver-2ad3f316-95ec-4749-955d-bb680ccb3a6f
service:
  extensions:
    - apikeyauth/otelcol-elasticapmintakereceiver-2ad3f316-95ec-4749-955d-bb680ccb3a6f-otelcol-elasticapm_input_otel-elasticapmintakereceiver-2ad3f316-95ec-4749-955d-bb680ccb3a6f
  pipelines:
    traces/custom:
      receivers:
        - elasticapmintakereceiver/2ad3f316-95ec-4749-955d-bb680ccb3a6f
`
		out, err := normalizePolicyToCanonical([]byte(policy))
		assert.NoError(t, err)
		t.Log(string(out))
		assert.Contains(t, string(out), "apikeyauth/componentid-0")
		assert.Contains(t, string(out), "elasticapmintakereceiver/componentid-0")
		assert.Contains(t, string(out), "- apikeyauth/componentid-0")
		assert.Contains(t, string(out), "authenticator: apikeyauth/componentid-0")
		assert.Contains(t, string(out), "traces/componentid-0")
	})

	t.Run("does not mix up references when there are two distinct apikeyauth extensions", func(t *testing.T) {
		policy := `
extensions:
  apikeyauth/otelcol-receiverA-aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa-otelcol-elasticapm_input_otel-receiverA-aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa:
    api_key: key-for-a
  apikeyauth/otelcol-receiverB-bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb-otelcol-elasticapm_input_otel-receiverB-bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb:
    api_key: key-for-b
receivers:
  elasticapmintakereceiver/aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa:
    endpoint: localhost:8200
    auth:
      authenticator: apikeyauth/otelcol-receiverA-aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa-otelcol-elasticapm_input_otel-receiverA-aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa
  elasticapmintakereceiver/bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb:
    endpoint: localhost:8201
    auth:
      authenticator: apikeyauth/otelcol-receiverB-bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb-otelcol-elasticapm_input_otel-receiverB-bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb
service:
  extensions:
    - apikeyauth/otelcol-receiverA-aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa-otelcol-elasticapm_input_otel-receiverA-aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa
    - apikeyauth/otelcol-receiverB-bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb-otelcol-elasticapm_input_otel-receiverB-bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb
`
		out, err := normalizePolicyToCanonical([]byte(policy))
		require.NoError(t, err)
		t.Log(string(out))

		var root map[string]any
		require.NoError(t, yaml.Unmarshal(out, &root))

		extensions, ok := root["extensions"].(map[string]any)
		require.True(t, ok, "extensions should be a map")

		// Identify each extension's canonical id by its distinguishing api_key,
		// since buildSectionMapping's sort order is value-based and not fixed here.
		var idForA, idForB string
		for key, val := range extensions {
			body, ok := val.(map[string]any)
			require.True(t, ok)
			switch body["api_key"] {
			case "key-for-a":
				idForA = key
			case "key-for-b":
				idForB = key
			}
		}
		require.NotEmpty(t, idForA, "extension for receiver A should have been found")
		require.NotEmpty(t, idForB, "extension for receiver B should have been found")
		assert.NotEqual(t, idForA, idForB, "the two extensions must normalize to distinct component ids")

		receivers, ok := root["receivers"].(map[string]any)
		require.True(t, ok, "receivers should be a map")

		var authForA, authForB string
		for _, val := range receivers {
			body, ok := val.(map[string]any)
			require.True(t, ok)
			auth, ok := body["auth"].(map[string]any)
			require.True(t, ok)
			authenticator, _ := auth["authenticator"].(string)
			switch body["endpoint"] {
			case "localhost:8200":
				authForA = authenticator
			case "localhost:8201":
				authForB = authenticator
			}
		}
		require.NotEmpty(t, authForA, "receiver A's authenticator should have been found")
		require.NotEmpty(t, authForB, "receiver B's authenticator should have been found")

		assert.Equal(t, idForA, authForA, "receiver A's authenticator must reference extension A's canonical id, not extension B's")
		assert.Equal(t, idForB, authForB, "receiver B's authenticator must reference extension B's canonical id, not extension A's")
	})
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

func TestReplaceMapStrValue(t *testing.T) {
	t.Run("replaces existing scalar value", func(t *testing.T) {
		m := common.MapStr{"key": "old"}
		err := replaceMapStrValue(m, "key", "new")
		require.NoError(t, err)
		assert.Equal(t, "new", m["key"])
	})

	t.Run("replaces existing slice value", func(t *testing.T) {
		m := common.MapStr{"key": []any{"a", "b"}}
		err := replaceMapStrValue(m, "key", []any{"x"})
		require.NoError(t, err)
		assert.Equal(t, []any{"x"}, m["key"])
	})

	t.Run("sets a new key", func(t *testing.T) {
		m := common.MapStr{}
		err := replaceMapStrValue(m, "new_key", 42)
		require.NoError(t, err)
		assert.Equal(t, 42, m["new_key"])
	})
}

func TestMapStringElemsInAnySlice(t *testing.T) {
	t.Run("transforms all string elements", func(t *testing.T) {
		in := []any{"hello", "world"}
		out, err := mapStringElemsInAnySlice(in, func(s string) string { return s + "!" })
		require.NoError(t, err)
		assert.Equal(t, []any{"hello!", "world!"}, out)
	})

	t.Run("returns error on non-string element", func(t *testing.T) {
		in := []any{"ok", 42}
		_, err := mapStringElemsInAnySlice(in, func(s string) string { return s })
		require.Error(t, err)
		assert.Contains(t, err.Error(), "expected string array element")
	})

	t.Run("empty slice returns empty slice", func(t *testing.T) {
		out, err := mapStringElemsInAnySlice([]any{}, func(s string) string { return s })
		require.NoError(t, err)
		assert.Empty(t, out)
	})
}

func TestFilterStringElemsInAnySlice(t *testing.T) {
	t.Run("keeps only elements matching predicate", func(t *testing.T) {
		in := []any{"keep", "drop", "keep2"}
		out, err := filterStringElemsInAnySlice(in, func(s string) bool { return s != "drop" })
		require.NoError(t, err)
		assert.Equal(t, []any{"keep", "keep2"}, out)
	})

	t.Run("returns error on non-string element", func(t *testing.T) {
		in := []any{"ok", 123}
		_, err := filterStringElemsInAnySlice(in, func(s string) bool { return true })
		require.Error(t, err)
		assert.Contains(t, err.Error(), "expected string array element")
	})

	t.Run("returns empty when all filtered out", func(t *testing.T) {
		in := []any{"a", "b"}
		out, err := filterStringElemsInAnySlice(in, func(s string) bool { return false })
		require.NoError(t, err)
		assert.Empty(t, out)
	})
}

func TestApplyElementsEntriesCleaning(t *testing.T) {
	t.Run("removes the named field from each list element", func(t *testing.T) {
		m := common.MapStr{
			"items": []any{
				common.MapStr{"id": "abc", "name": "foo"},
				common.MapStr{"id": "def", "name": "bar"},
			},
		}
		v, _ := m.GetValue("items")
		err := applyElementsEntriesCleaning(m, "items", v, []policyEntryFilter{{name: "id"}})
		require.NoError(t, err)

		items, err := common.ToMapStrSlice(m["items"])
		require.NoError(t, err)
		for _, item := range items {
			_, hasID := item["id"]
			assert.False(t, hasID, "id should have been removed")
			_, hasName := item["name"]
			assert.True(t, hasName, "name should be preserved")
		}
	})

	t.Run("returns error when value is not a slice of maps", func(t *testing.T) {
		m := common.MapStr{"items": "not-a-list"}
		err := applyElementsEntriesCleaning(m, "items", "not-a-list", []policyEntryFilter{{name: "id"}})
		require.Error(t, err)
	})
}

func TestApplyMapValuesCleaning(t *testing.T) {
	t.Run("removes filtered field from each nested map value", func(t *testing.T) {
		m := common.MapStr{
			"section": common.MapStr{
				"alpha": common.MapStr{"auth": "secret", "url": "http://example.com"},
				"beta":  common.MapStr{"auth": "secret2", "url": "http://other.com"},
			},
		}
		v, _ := m.GetValue("section")
		err := applyMapValuesCleaning(v, []policyEntryFilter{{name: "auth"}})
		require.NoError(t, err)

		section, err := common.ToMapStr(m["section"])
		require.NoError(t, err)
		for _, child := range section {
			childMap, err := common.ToMapStr(child)
			require.NoError(t, err)
			_, hasAuth := childMap["auth"]
			assert.False(t, hasAuth, "auth should have been removed")
			_, hasURL := childMap["url"]
			assert.True(t, hasURL, "url should be preserved")
		}
	})

	t.Run("returns error when value is not a map", func(t *testing.T) {
		err := applyMapValuesCleaning("not-a-map", []policyEntryFilter{{name: "id"}})
		require.Error(t, err)
	})
}

func TestApplyMemberReplace(t *testing.T) {
	re := &policyEntryReplace{
		regexp:  regexp.MustCompile(`^uuid-.*$`),
		replace: "normalized",
	}

	t.Run("replaces matching keys in a MapStr", func(t *testing.T) {
		m := common.MapStr{
			"perms": common.MapStr{
				"uuid-abc-123": "value1",
				"_keep":        "value2",
			},
		}
		v, _ := m.GetValue("perms")
		err := applyMemberReplace(m, "perms", v, re)
		require.NoError(t, err)

		perms, err := common.ToMapStr(m["perms"])
		require.NoError(t, err)
		_, hasOld := perms["uuid-abc-123"]
		assert.False(t, hasOld, "original key should be gone")
		_, hasNew := perms["normalized"]
		assert.True(t, hasNew, "replacement key should exist")
		_, hasKept := perms["_keep"]
		assert.True(t, hasKept, "non-matching key should be preserved")
	})

	t.Run("replaces matching string elements in a []any", func(t *testing.T) {
		m := common.MapStr{
			"refs": []any{"uuid-abc-123", "keep-me"},
		}
		v := m["refs"]
		err := applyMemberReplace(m, "refs", v, re)
		require.NoError(t, err)
		assert.Equal(t, []any{"normalized", "keep-me"}, m["refs"])
	})

	t.Run("returns error on non-string element in []any", func(t *testing.T) {
		m := common.MapStr{"refs": []any{42}}
		err := applyMemberReplace(m, "refs", m["refs"], re)
		require.Error(t, err)
	})

	t.Run("returns error on unexpected value type", func(t *testing.T) {
		m := common.MapStr{"refs": "a plain string"}
		err := applyMemberReplace(m, "refs", m["refs"], re)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "expected map or array for memberReplace")
	})
}

func TestApplyStringValueReplace(t *testing.T) {
	re := &policyEntryReplace{
		regexp:  regexp.MustCompile(`^(.+)-[0-9]+$`),
		replace: "$1",
	}

	t.Run("strips numeric suffix from matching string", func(t *testing.T) {
		m := common.MapStr{"name": "my-input-42"}
		err := applyStringValueReplace(m, "name", m["name"], re)
		require.NoError(t, err)
		assert.Equal(t, "my-input", m["name"])
	})

	t.Run("leaves non-matching string unchanged", func(t *testing.T) {
		m := common.MapStr{"name": "my-input"}
		err := applyStringValueReplace(m, "name", m["name"], re)
		require.NoError(t, err)
		assert.Equal(t, "my-input", m["name"])
	})

	t.Run("returns error when value is not a string", func(t *testing.T) {
		m := common.MapStr{"name": 99}
		err := applyStringValueReplace(m, "name", m["name"], re)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "expected string")
	})
}

func TestApplyDeletePattern(t *testing.T) {
	pat := regexp.MustCompile(`^beatsauth/`)

	t.Run("removes matching keys from a MapStr", func(t *testing.T) {
		m := common.MapStr{
			"extensions": common.MapStr{
				"beatsauth/default": common.MapStr{},
				"health_check/abc":  common.MapStr{},
			},
		}
		v, _ := m.GetValue("extensions")
		err := applyDeletePattern(m, "extensions", v, pat)
		require.NoError(t, err)

		ext, err := common.ToMapStr(m["extensions"])
		require.NoError(t, err)
		_, hasBeats := ext["beatsauth/default"]
		assert.False(t, hasBeats, "beatsauth key should be removed")
		_, hasHealth := ext["health_check/abc"]
		assert.True(t, hasHealth, "non-matching key should remain")
	})

	t.Run("filters matching string elements from []any", func(t *testing.T) {
		m := common.MapStr{
			"service": common.MapStr{
				"extensions": []any{"beatsauth/default", "health_check/abc"},
			},
		}
		v, _ := m.GetValue("service.extensions")
		err := applyDeletePattern(m, "service.extensions", v, pat)
		require.NoError(t, err)
		got, err := m.GetValue("service.extensions")
		require.NoError(t, err)
		assert.Equal(t, []any{"health_check/abc"}, got)
	})

	t.Run("returns error on non-string element in []any", func(t *testing.T) {
		m := common.MapStr{"list": []any{123}}
		err := applyDeletePattern(m, "list", m["list"], pat)
		require.Error(t, err)
	})

	t.Run("returns error on unexpected value type", func(t *testing.T) {
		m := common.MapStr{"field": "scalar"}
		err := applyDeletePattern(m, "field", m["field"], pat)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "expected map or array for deletePattern")
	})
}
