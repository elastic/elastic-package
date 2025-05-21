// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package stack

import (
	"strings"
	"testing"

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
