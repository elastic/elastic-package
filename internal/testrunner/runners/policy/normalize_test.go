// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package policy

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

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

	t.Run("bare forward connector normalizes to forward/componentid-N", func(t *testing.T) {
		policy := `
connectors:
  forward: {}
service:
  pipelines:
    logs/my-stream:
      receivers:
        - otlp/my-stream
      exporters:
        - forward
    logs:
      receivers:
        - forward
      exporters:
        - elasticsearch/default
`
		out, err := normalizePolicyToCanonical([]byte(policy))
		assert.NoError(t, err)
		assert.Contains(t, string(out), "forward/componentid-0")
		assert.NotContains(t, string(out), "forward: {}")
		assert.NotContains(t, string(out), "_bare")
	})

	t.Run("bare and suffixed forward normalize to same result", func(t *testing.T) {
		bare := `
connectors:
  forward: {}
service:
  pipelines:
    logs/my-stream:
      exporters:
        - forward
    logs:
      receivers:
        - forward
      exporters:
        - elasticsearch/default
`
		suffixed := `
connectors:
  forward/default: {}
service:
  pipelines:
    logs/my-stream:
      exporters:
        - forward/default
    logs/default:
      receivers:
        - forward/default
      exporters:
        - elasticsearch/default
`
		outBare, err := normalizePolicyToCanonical([]byte(bare))
		assert.NoError(t, err)
		t.Log(string(outBare))
		outSuffixed, err := normalizePolicyToCanonical([]byte(suffixed))
		assert.NoError(t, err)
		t.Log(string(outSuffixed))
		assert.Equal(t, string(outBare), string(outSuffixed))
	})

	// Regression test: the string "forward" must only be renamed to "forward/_bare" when
	// it appears as a connector reference inside pipeline receiver/exporter lists. A
	// component config list that happens to contain the string "forward" as an arbitrary
	// value (e.g. a filter processor body-match list) must not be rewritten.
	t.Run("forward string in non-pipeline list is not renamed to forward/_bare", func(t *testing.T) {
		policy := `
connectors:
  forward: {}
processors:
  filter/abc:
    logs:
      include:
        match_type: strict
        bodies:
          - forward
service:
  pipelines:
    logs:
      receivers:
        - otlp/abc
      processors:
        - filter/abc
      exporters:
        - forward
`
		out, err := normalizePolicyToCanonical([]byte(policy))
		assert.NoError(t, err)
		t.Log(string(out))
		// The connector reference in the pipeline exporter list must be normalized.
		assert.Contains(t, string(out), "- forward/componentid-0")
		// The "forward" string inside the filter processor body list is not a connector
		// reference — it must remain unchanged.
		assert.Contains(t, string(out), "- forward\n")
	})

	// Regression test for kibana#270487: Fleet started suffixing bare pipeline keys
	// (e.g. "logs", "metrics") with the output ID (e.g. "logs/default"). A bare key
	// and its suffixed equivalent must normalize to the same canonical form.
	t.Run("bare and suffixed pipeline keys normalize to same result", func(t *testing.T) {
		bare := `
receivers:
  otlp/abc: {}
exporters:
  elasticsearch/default: {}
service:
  pipelines:
    logs:
      receivers:
        - otlp/abc
      exporters:
        - elasticsearch/default
    metrics:
      receivers:
        - otlp/abc
      exporters:
        - elasticsearch/default
`
		suffixed := `
receivers:
  otlp/abc: {}
exporters:
  elasticsearch/default: {}
service:
  pipelines:
    logs/default:
      receivers:
        - otlp/abc
      exporters:
        - elasticsearch/default
    metrics/default:
      receivers:
        - otlp/abc
      exporters:
        - elasticsearch/default
`
		outBare, err := normalizePolicyToCanonical([]byte(bare))
		assert.NoError(t, err)
		t.Log(string(outBare))
		outSuffixed, err := normalizePolicyToCanonical([]byte(suffixed))
		assert.NoError(t, err)
		t.Log(string(outSuffixed))
		assert.Equal(t, string(outBare), string(outSuffixed))
	})

	t.Run("strips OTTL where clause from data_stream set statements", func(t *testing.T) {
		policy := `
processors:
  transform/abc:
    log_statements:
      - context: log
        statements:
          - set(attributes["data_stream.type"], "logs")
          - set(attributes["data_stream.dataset"], "my_pkg.events") where attributes["data_stream.dataset"] == nil
          - set(attributes["data_stream.namespace"], "ep") where attributes["data_stream.namespace"] == nil
`
		out, err := normalizePolicyToCanonical([]byte(policy))
		assert.NoError(t, err)
		s := string(out)
		assert.Contains(t, s, `set(attributes["data_stream.dataset"], "my_pkg.events")`)
		assert.Contains(t, s, `set(attributes["data_stream.namespace"], "ep")`)
		assert.NotContains(t, s, "where")
		assert.NotContains(t, s, "== nil")
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

	t.Run("bare extension key referenced from service.extensions normalizes to componentid-0", func(t *testing.T) {
		policy := `
extensions:
  basicauth:
    htpasswd:
      - username: user
        password: pass
service:
  extensions:
    - basicauth
  pipelines:
    traces/abc:
      receivers:
        - otlp/abc
`
		out, err := normalizePolicyToCanonical([]byte(policy))
		assert.NoError(t, err)
		t.Log(string(out))
		assert.Contains(t, string(out), "basicauth/componentid-0")
		assert.Contains(t, string(out), "- basicauth/componentid-0")
		assert.NotContains(t, string(out), "_bare")
	})

	t.Run("bare extension key referenced from auth.authenticator normalizes to componentid-0", func(t *testing.T) {
		policy := `
extensions:
  basicauth:
    htpasswd:
      - username: user
        password: pass
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
`
		out, err := normalizePolicyToCanonical([]byte(policy))
		assert.NoError(t, err)
		t.Log(string(out))
		assert.Contains(t, string(out), "basicauth/componentid-0")
		assert.Contains(t, string(out), "authenticator: basicauth/componentid-0")
		assert.NotContains(t, string(out), "_bare")
	})

	t.Run("bare and suffixed extension normalize to same result", func(t *testing.T) {
		bare := `
extensions:
  basicauth:
    htpasswd:
      - username: user
        password: pass
receivers:
  otlp/abc:
    protocols:
      grpc:
        auth:
          authenticator: basicauth
service:
  extensions:
    - basicauth
`
		suffixed := `
extensions:
  basicauth/componentid-0:
    htpasswd:
      - username: user
        password: pass
receivers:
  otlp/abc:
    protocols:
      grpc:
        auth:
          authenticator: basicauth/componentid-0
service:
  extensions:
    - basicauth/componentid-0
`
		outBare, err := normalizePolicyToCanonical([]byte(bare))
		assert.NoError(t, err)
		t.Log(string(outBare))
		outSuffixed, err := normalizePolicyToCanonical([]byte(suffixed))
		assert.NoError(t, err)
		t.Log(string(outSuffixed))
		assert.Equal(t, string(outBare), string(outSuffixed))
	})

	t.Run("suffixed extension key with bare reference normalizes correctly", func(t *testing.T) {
		// Expected files may be in a mixed state: extension map key already has the
		// componentid suffix but references (service.extensions, auth.authenticator) are
		// still bare. This happens when Fleet starts suffixing extension IDs but the
		// expected file was only partially updated.
		policy := `
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
`
		out, err := normalizePolicyToCanonical([]byte(policy))
		assert.NoError(t, err)
		t.Log(string(out))
		assert.Contains(t, string(out), "basicauth/componentid-0")
		assert.Contains(t, string(out), "authenticator: basicauth/componentid-0")
		assert.Contains(t, string(out), "- basicauth/componentid-0")
		assert.NotContains(t, string(out), "_bare")
	})

	t.Run("two bare extensions both normalize with correct cross-references", func(t *testing.T) {
		policy := `
extensions:
  basicauth:
    htpasswd:
      - username: user
        password: pass
  bearertokenauth:
    token: mytoken
receivers:
  otlp/grpc:
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
`
		out, err := normalizePolicyToCanonical([]byte(policy))
		assert.NoError(t, err)
		t.Log(string(out))
		assert.NotContains(t, string(out), "_bare")
		// Both extensions must appear with a componentid suffix
		assert.Contains(t, string(out), "basicauth/componentid-")
		assert.Contains(t, string(out), "bearertokenauth/componentid-")
	})

	t.Run("bare extension key referenced from middlewares[].id normalizes to componentid-0", func(t *testing.T) {
		policy := `
extensions:
  myauth:
    token: secret
receivers:
  otlp/abc:
    protocols:
      grpc:
        middlewares:
          - id: myauth
service:
  extensions:
    - myauth
`
		out, err := normalizePolicyToCanonical([]byte(policy))
		assert.NoError(t, err)
		t.Log(string(out))
		assert.Contains(t, string(out), "myauth/componentid-0")
		assert.Contains(t, string(out), "id: myauth/componentid-0")
		assert.NotContains(t, string(out), "_bare")
	})

	t.Run("suffixed extension key with bare middlewares[].id reference normalizes correctly", func(t *testing.T) {
		// Mixed state: extension map key already has the componentid suffix but
		// middlewares[].id still uses the bare type name.
		policy := `
extensions:
  myauth/componentid-0:
    token: secret
receivers:
  otlp/abc:
    protocols:
      grpc:
        middlewares:
          - id: myauth
service:
  extensions:
    - myauth
`
		out, err := normalizePolicyToCanonical([]byte(policy))
		assert.NoError(t, err)
		t.Log(string(out))
		assert.Contains(t, string(out), "myauth/componentid-0")
		assert.Contains(t, string(out), "id: myauth/componentid-0")
		assert.Contains(t, string(out), "- myauth/componentid-0")
		assert.NotContains(t, string(out), "_bare")
	})

	t.Run("non-extension id field inside middlewares is not affected by extension ref resolver", func(t *testing.T) {
		// A middlewares list element may carry fields other than "id" — and an "id" field
		// elsewhere in the tree (outside of middlewares) must not be touched.
		policy := `
extensions:
  myauth/componentid-0:
    token: secret
processors:
  batch/abc:
    send_batch_size: 100
receivers:
  otlp/abc:
    protocols:
      grpc:
        middlewares:
          - id: myauth
            extra_field: myauth
service:
  extensions:
    - myauth
`
		out, err := normalizePolicyToCanonical([]byte(policy))
		assert.NoError(t, err)
		t.Log(string(out))
		assert.Contains(t, string(out), "id: myauth/componentid-0")
		// extra_field is not an extension-reference position — must stay unchanged.
		assert.Contains(t, string(out), "extra_field: myauth")
		assert.NotContains(t, string(out), "extra_field: myauth/componentid-0")
	})

	// Regression test: extension type names that appear as arbitrary config values (e.g. a
	// processor action's "value" field) must not be renamed. Only values at known OTel
	// extension-reference positions — auth.authenticator and service.extensions[] — are
	// extension references per the OTel collector source (config/configauth).
	t.Run("extension type name used as config value is not renamed", func(t *testing.T) {
		policy := `
extensions:
  basicauth:
    htpasswd:
      file: /etc/otel/.htpasswd
receivers:
  otlp/abc:
    protocols:
      grpc:
        auth:
          authenticator: basicauth
        endpoint: localhost:4317
      http:
        auth:
          authenticator: basicauth
        endpoint: localhost:4318
processors:
  attributes/abc:
    actions:
      - key: auth.scheme
        value: basicauth
        action: insert
service:
  extensions:
    - basicauth
`
		out, err := normalizePolicyToCanonical([]byte(policy))
		require.NoError(t, err)
		t.Log(string(out))
		// Receiver auth.authenticator references (nested inside receivers.*.protocols.*.auth)
		// must be normalized — this is the real OTel extension-reference position.
		assert.Contains(t, string(out), "authenticator: basicauth/componentid-0")
		// service.extensions reference must also be normalized.
		assert.Contains(t, string(out), "- basicauth/componentid-0")
		// The processor "value" field is NOT an extension reference — must stay unchanged.
		assert.Contains(t, string(out), "value: basicauth")
		assert.NotContains(t, string(out), "value: basicauth/componentid-0")
	})

	// Regression test: if the policy already contains a genuine "name/_bare" key, the
	// bare-key rename in preNormalizePolicy must not overwrite it. Without a guard the
	// bare rename silently replaces the existing "_bare" entry, losing its config content.
	// Covers both connectors (forward/_bare) and extensions (basicauth/_bare).
	t.Run("bare rename is skipped when _bare variant already exists", func(t *testing.T) {
		policy := `
connectors:
  forward/_bare:
    timeout: 5s
  forward:
    timeout: 10s
extensions:
  basicauth/_bare:
    htpasswd:
      file: /etc/otel/.htpasswd-real
  basicauth:
    htpasswd:
      file: /etc/otel/.htpasswd-bare
service:
  extensions:
    - basicauth/_bare
  pipelines:
    logs:
      receivers:
        - forward
      exporters:
        - forward/_bare
`
		out, err := normalizePolicyToCanonical([]byte(policy))
		require.NoError(t, err)
		t.Log(string(out))
		// The real basicauth/_bare entry must survive — its content must not be lost.
		assert.Contains(t, string(out), "/etc/otel/.htpasswd-real")
		// The bare basicauth entry must survive as its own key (rename skipped).
		// "basicauth:\n" matches the bare key but not "basicauth/componentid-0:".
		assert.Contains(t, string(out), "/etc/otel/.htpasswd-bare")
		assert.Contains(t, string(out), "basicauth:\n")
		// The real forward/_bare entry must survive — timeout: 5s must not be lost.
		assert.Contains(t, string(out), "timeout: 5s")
		// The bare forward entry must survive as its own key (rename skipped).
		// "forward:\n" matches the bare key but not "forward/componentid-0:".
		assert.Contains(t, string(out), "timeout: 10s")
		assert.Contains(t, string(out), "forward:\n")
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
