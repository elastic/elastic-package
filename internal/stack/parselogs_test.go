// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package stack

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseLogsFromReader(t *testing.T) {

	for _, tc := range []struct {
		name                   string
		logs                   string
		expectedParsedLogCount int
	}{
		{
			name:                   "alternative logs",
			logs:                   "elasticsearch-1  | {\"type\": \"server\", \"timestamp\": \"2024-04-10T12:54:26,235Z\", \"level\": \"WARN\", \"component\": \"o.e.i.c.GrokProcessor\", \"cluster.name\": \"elasticsearch\", \"node.name\": \"9e39afe7080b\", \"message\": \"character class has '-' without escape\", \"cluster.uuid\": \"WbdhEzxGRo6c9Td8pW646g\", \"node.id\": \"AATj6t7cTCqOrA8G2NT40Q\"  }\nelasticsearch-1  | {\"type\": \"server\", \"timestamp\": \"2024-04-10T12:54:26,289Z\", \"level\": \"WARN\", \"component\": \"o.e.i.c.GrokProcessor\", \"cluster.name\": \"elasticsearch\", \"node.name\": \"9e39afe7080b\", \"message\": \"character class has '-' without escape\", \"cluster.uuid\": \"WbdhEzxGRo6c9Td8pW646g\", \"node.id\": \"AATj6t7cTCqOrA8G2NT40Q\"  }\nelasticsearch-1  | {\"type\": \"server\", \"timestamp\": \"2024-04-10T12:54:26,292Z\", \"level\": \"WARN\", \"component\": \"o.e.i.c.GrokProcessor\", \"cluster.name\": \"elasticsearch\", \"node.name\": \"9e39afe7080b\", \"message\": \"character class has '-' without escape\", \"cluster.uuid\": \"WbdhEzxGRo6c9Td8pW646g\", \"node.id\": \"AATj6t7cTCqOrA8G2NT40Q\"  }",
			expectedParsedLogCount: 3,
		},
		{
			name:                   "standard docker compose logs",
			logs:                   "elasticsearch-1  | {\"@timestamp\":\"2024-04-10T12:54:24.215Z\", \"log.level\": \"WARN\", \"message\":\"regular expression has redundant nested repeat operator * /(?:^(?:(?:.*)(?:\\\\s*)?(?i)interface)(?:\\\\s*)(?<DATA:cisco_nexus.log.interface.name>.*?) is up in mode (?<DATA:cisco_nexus.log.interface.mode>.*?)$)|(?:^(?:(?:.*)(?:\\\\s*)?(?i)interface)(?:\\\\s*)(?<DATA:cisco_nexus.log.interface.name>.*?) is (?:.*)$)|(?:^(?:(?:.*)(?:\\\\s*)?(?i)interface)(?:\\\\s*)(?<DATA:cisco_nexus.log.interface.name>.*?), operational speed changed to (?<DATA:cisco_nexus.log.operational.speed>.*?)$)|(?:^(?:(?:.*)(?:\\\\s*)?(?i)interface)(?:\\\\s*)(?<DATA:cisco_nexus.log.interface.name>.*?), operational duplex mode changed to (?<DATA:cisco_nexus.log.operational.duplex_mode>.*?)$)|(?:^(?:(?:.*)(?:\\\\s*)?(?i)interface)(?:\\\\s*)(?<DATA:cisco_nexus.log.interface.name>.*?), operational Receive Flow Control state changed to (?<DATA:cisco_nexus.log.operational.receive_flow_control_state>.*?)$)|(?:^(?:(?:.*)(?:\\\\s*)?(?i)interface)(?:\\\\s*)(?<DATA:cisco_nexus.log.interface.name>.*?), operational Transmit Flow Control state changed to (?<DATA:cisco_nexus.log.operational.transmit_flow_control_state>.*?)$)|(?:^(?:(?:.*)(?:\\\\s*)?(?i)interface)(?:\\\\s*)(?<DATA:cisco_nexus.log.interface.name>.*?), (?:.*)$)/\", \"ecs.version\": \"1.2.0\",\"service.name\":\"ES_ECS\",\"event.dataset\":\"elasticsearch.server\",\"process.thread.name\":\"elasticsearch[f2ffbe394425][generic][T#8]\",\"log.logger\":\"org.elasticsearch.ingest.common.GrokProcessor\",\"trace.id\":\"89d7761d700b89b4f0de49c96497da89\",\"elasticsearch.cluster.uuid\":\"PTFv9bIHQsu41s4fb0mNmQ\",\"elasticsearch.node.id\":\"z2txLVDzSzqbK_TFymgr1Q\",\"elasticsearch.node.name\":\"f2ffbe394425\",\"elasticsearch.cluster.name\":\"elasticsearch\"}\nelasticsearch-1  | {\"@timestamp\":\"2024-04-10T12:54:24.216Z\", \"log.level\": \"WARN\", \"message\":\"regular expression has redundant nested repeat operator * /(?:^(?:(?:.*)(?:\\\\s*)?(?i)interface)(?:\\\\s*)(?<DATA:cisco_nexus.log.interface.name>.*?) is up in mode (?<DATA:cisco_nexus.log.interface.mode>.*?)$)|(?:^(?:(?:.*)(?:\\\\s*)?(?i)interface)(?:\\\\s*)(?<DATA:cisco_nexus.log.interface.name>.*?) is (?:.*)$)|(?:^(?:(?:.*)(?:\\\\s*)?(?i)interface)(?:\\\\s*)(?<DATA:cisco_nexus.log.interface.name>.*?), operational speed changed to (?<DATA:cisco_nexus.log.operational.speed>.*?)$)|(?:^(?:(?:.*)(?:\\\\s*)?(?i)interface)(?:\\\\s*)(?<DATA:cisco_nexus.log.interface.name>.*?), operational duplex mode changed to (?<DATA:cisco_nexus.log.operational.duplex_mode>.*?)$)|(?:^(?:(?:.*)(?:\\\\s*)?(?i)interface)(?:\\\\s*)(?<DATA:cisco_nexus.log.interface.name>.*?), operational Receive Flow Control state changed to (?<DATA:cisco_nexus.log.operational.receive_flow_control_state>.*?)$)|(?:^(?:(?:.*)(?:\\\\s*)?(?i)interface)(?:\\\\s*)(?<DATA:cisco_nexus.log.interface.name>.*?), operational Transmit Flow Control state changed to (?<DATA:cisco_nexus.log.operational.transmit_flow_control_state>.*?)$)|(?:^(?:(?:.*)(?:\\\\s*)?(?i)interface)(?:\\\\s*)(?<DATA:cisco_nexus.log.interface.name>.*?), (?:.*)$)/\", \"ecs.version\": \"1.2.0\",\"service.name\":\"ES_ECS\",\"event.dataset\":\"elasticsearch.server\",\"process.thread.name\":\"elasticsearch[f2ffbe394425][generic][T#8]\",\"log.logger\":\"org.elasticsearch.ingest.common.GrokProcessor\",\"trace.id\":\"89d7761d700b89b4f0de49c96497da89\",\"elasticsearch.cluster.uuid\":\"PTFv9bIHQsu41s4fb0mNmQ\",\"elasticsearch.node.id\":\"z2txLVDzSzqbK_TFymgr1Q\",\"elasticsearch.node.name\":\"f2ffbe394425\",\"elasticsearch.cluster.name\":\"elasticsearch\"}\nelasticsearch-1  | {\"@timestamp\":\"2024-04-10T12:54:24.216Z\", \"log.level\": \"WARN\", \"message\":\"regular expression has redundant nested repeat operator * /(?:^(?:(?:.*)(?:\\\\s*)?(?i)interface)(?:\\\\s*)(?<DATA:cisco_nexus.log.interface.name>.*?) is up in mode (?<DATA:cisco_nexus.log.interface.mode>.*?)$)|(?:^(?:(?:.*)(?:\\\\s*)?(?i)interface)(?:\\\\s*)(?<DATA:cisco_nexus.log.interface.name>.*?) is (?:.*)$)|(?:^(?:(?:.*)(?:\\\\s*)?(?i)interface)(?:\\\\s*)(?<DATA:cisco_nexus.log.interface.name>.*?), operational speed changed to (?<DATA:cisco_nexus.log.operational.speed>.*?)$)|(?:^(?:(?:.*)(?:\\\\s*)?(?i)interface)(?:\\\\s*)(?<DATA:cisco_nexus.log.interface.name>.*?), operational duplex mode changed to (?<DATA:cisco_nexus.log.operational.duplex_mode>.*?)$)|(?:^(?:(?:.*)(?:\\\\s*)?(?i)interface)(?:\\\\s*)(?<DATA:cisco_nexus.log.interface.name>.*?), operational Receive Flow Control state changed to (?<DATA:cisco_nexus.log.operational.receive_flow_control_state>.*?)$)|(?:^(?:(?:.*)(?:\\\\s*)?(?i)interface)(?:\\\\s*)(?<DATA:cisco_nexus.log.interface.name>.*?), operational Transmit Flow Control state changed to (?<DATA:cisco_nexus.log.operational.transmit_flow_control_state>.*?)$)|(?:^(?:(?:.*)(?:\\\\s*)?(?i)interface)(?:\\\\s*)(?<DATA:cisco_nexus.log.interface.name>.*?), (?:.*)$)/\", \"ecs.version\": \"1.2.0\",\"service.name\":\"ES_ECS\",\"event.dataset\":\"elasticsearch.server\",\"process.thread.name\":\"elasticsearch[f2ffbe394425][generic][T#8]\",\"log.logger\":\"org.elasticsearch.ingest.common.GrokProcessor\",\"trace.id\":\"89d7761d700b89b4f0de49c96497da89\",\"elasticsearch.cluster.uuid\":\"PTFv9bIHQsu41s4fb0mNmQ\",\"elasticsearch.node.id\":\"z2txLVDzSzqbK_TFymgr1Q\",\"elasticsearch.node.name\":\"f2ffbe394425\",\"elasticsearch.cluster.name\":\"elasticsearch\"}\n",
			expectedParsedLogCount: 3,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			redLogLines := 0
			err := ParseLogsFromReader(strings.NewReader(tc.logs), ParseLogsOptions{}, func(log LogLine) error {
				if log.LogLevel == "" {
					return nil
				}
				redLogLines++
				return nil
			})
			require.NoError(t, err)
			require.Equal(t, tc.expectedParsedLogCount, redLogLines)
		})
	}
}

const sampleFailedToIndexDocument = `{"log.level":"error","@timestamp":"2026-07-01T17:34:35.220Z","message":"failed to index document","http.response.status_code":400,"error.type":"illegal_argument_exception","error.reason":"field [event.original] cannot reconstruct _source from doc values; every field must be reconstructable from doc values in index using [logsdb_columnar] index mode","ecs.version":"1.6.0"}`

func TestParseNDJSONLogsFromReader_IndexingFailure(t *testing.T) {
	start, err := time.Parse(time.RFC3339Nano, "2026-07-01T17:00:00.000Z")
	require.NoError(t, err, "parse start time")

	var logs []LogLine
	err = ParseNDJSONLogsFromReader(strings.NewReader(sampleFailedToIndexDocument+"\n"), ParseLogsOptions{
		StartTime: start,
	}, func(log LogLine) error {
		logs = append(logs, log)
		return nil
	})
	require.NoError(t, err, "parse NDJSON")
	require.Len(t, logs, 1, "expected one log line")

	assert.Equal(t, "failed to index document", logs[0].Message, "message")
	assert.Equal(t, "illegal_argument_exception", logs[0].ErrorType, "error.type")
	assert.Contains(t, logs[0].ErrorReason, "logsdb_columnar", "error.reason should mention logsdb_columnar")
	assert.Equal(t,
		"failed to index document: field [event.original] cannot reconstruct _source from doc values; every field must be reconstructable from doc values in index using [logsdb_columnar] index mode (illegal_argument_exception)",
		logs[0].FormatError(),
		"FormatError should include message, reason, and type",
	)
}

func TestParseNDJSONLogsFromReader_FiltersByStartTime(t *testing.T) {
	start, err := time.Parse(time.RFC3339Nano, "2026-07-01T18:00:00.000Z")
	require.NoError(t, err, "parse start time")

	var count int
	err = ParseNDJSONLogsFromReader(strings.NewReader(sampleFailedToIndexDocument+"\n"), ParseLogsOptions{
		StartTime: start,
	}, func(log LogLine) error {
		count++
		return nil
	})
	require.NoError(t, err, "parse NDJSON")
	assert.Equal(t, 0, count, "log before start time should be skipped")
}

func TestParseLogsFromReader_PreservesErrorFields(t *testing.T) {
	line := "elastic-agent-1  | " + sampleFailedToIndexDocument + "\n"
	start, err := time.Parse(time.RFC3339Nano, "2026-07-01T17:00:00.000Z")
	require.NoError(t, err, "parse start time")

	var logs []LogLine
	err = ParseLogsFromReader(strings.NewReader(line), ParseLogsOptions{StartTime: start}, func(log LogLine) error {
		logs = append(logs, log)
		return nil
	})
	require.NoError(t, err, "parse docker-compose logs")
	require.Len(t, logs, 1, "expected one log line")
	assert.Equal(t, "failed to index document", logs[0].Message, "message")
	assert.Contains(t, logs[0].ErrorReason, "event.original", "error.reason")
}

func TestParseNDJSONLogsDir(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "elastic-agent-20260701.ndjson")
	require.NoError(t, os.WriteFile(path, []byte(sampleFailedToIndexDocument+"\n"), 0o644), "write fixture")

	start, err := time.Parse(time.RFC3339Nano, "2026-07-01T17:00:00.000Z")
	require.NoError(t, err, "parse start time")

	var logs []LogLine
	err = ParseNDJSONLogsDir(dir, start, func(log LogLine) error {
		logs = append(logs, log)
		return nil
	})
	require.NoError(t, err, "parse NDJSON dir")
	require.Len(t, logs, 1, "expected one log from dir")
	assert.Equal(t, "failed to index document", logs[0].Message, "message")
}

func TestLogLineFormatError(t *testing.T) {
	tests := []struct {
		name string
		log  LogLine
		want string
	}{
		{
			name: "message only",
			log:  LogLine{Message: "something failed"},
			want: "something failed",
		},
		{
			name: "message and reason",
			log:  LogLine{Message: "failed to index document", ErrorReason: "bad mapping"},
			want: "failed to index document: bad mapping",
		},
		{
			name: "message reason and type",
			log: LogLine{
				Message:     "failed to index document",
				ErrorReason: "bad mapping",
				ErrorType:   "illegal_argument_exception",
			},
			want: "failed to index document: bad mapping (illegal_argument_exception)",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, tc.log.FormatError())
		})
	}
}
