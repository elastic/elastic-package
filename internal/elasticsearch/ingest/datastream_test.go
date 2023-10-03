// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package ingest

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLoadRoutingRuleFileGoodSingleRule(t *testing.T) {
	mockDataStreamPath := "../testdata/routing_rules/good/single_rule"
	rerouteProcessors, err := loadRoutingRuleFile(mockDataStreamPath)
	assert.NoError(t, err)
	assert.Equal(t, 3, len(rerouteProcessors))

	expectedProcessors := map[string]struct {
		expectedIf        string
		expectedDataset   []string
		expectedNamespace []string
	}{
		"multiple_namespace": {
			"ctx['aws.cloudwatch.log_stream'].contains('CloudTrail')",
			[]string{"aws.cloudtrail"},
			[]string{"{{labels.data_stream.namespace}}", "default"},
		},
		"multiple_target_dataset": {
			"ctx['aws.cloudwatch.log_stream'].contains('Firewall')",
			[]string{"aws.firewall_logs", "aws.test_logs"},
			[]string{"default"},
		},
		"single_namespace_target_dataset": {
			"ctx['aws.cloudwatch.log_stream'].contains('Route53')",
			[]string{"aws.route53_public_logs"},
			[]string{"{{labels.data_stream.namespace}}"},
		},
	}

	for _, rerouteProcessor := range rerouteProcessors {
		p := rerouteProcessor["reroute"].(RerouteProcessor)
		assert.Equal(t, expectedProcessors[p.Tag].expectedIf, p.If)
		assert.Equal(t, expectedProcessors[p.Tag].expectedDataset, p.Dataset)
		assert.Equal(t, expectedProcessors[p.Tag].expectedNamespace, p.Namespace)
	}
}

func TestLoadRoutingRuleFileGoodMultipleRules(t *testing.T) {
	mockDataStreamPath := "../testdata/routing_rules/good/multiple_rules"
	rerouteProcessors, err := loadRoutingRuleFile(mockDataStreamPath)
	assert.NoError(t, err)
	assert.Equal(t, 2, len(rerouteProcessors))

	expectedProcessors := map[string]struct {
		expectedSourceDataset string
		expectedDataset       []string
		expectedNamespace     []string
	}{
		"ctx['aws.cloudwatch.log_stream'].contains('Test1')": {
			"multiple_rules",
			[]string{"aws.test1_logs"},
			[]string{"default"},
		},
		"ctx['aws.cloudwatch.log_stream'].contains('Test2')": {
			"multiple_rules",
			[]string{"aws.test2_logs"},
			[]string{"{{labels.data_stream.namespace}}"},
		},
	}

	for _, rerouteProcessor := range rerouteProcessors {
		p := rerouteProcessor["reroute"].(RerouteProcessor)
		assert.Equal(t, expectedProcessors[p.If].expectedSourceDataset, p.Tag)
		assert.Equal(t, expectedProcessors[p.If].expectedDataset, p.Dataset)
		assert.Equal(t, expectedProcessors[p.If].expectedNamespace, p.Namespace)
	}
}

func TestLoadRoutingRuleFileGoodEmpty(t *testing.T) {
	mockDataStreamPath := "../testdata/routing_rules/good/empty"
	rerouteProcessors, err := loadRoutingRuleFile(mockDataStreamPath)
	assert.Equal(t, 0, len(rerouteProcessors))
	assert.NoError(t, err)
}

func TestLoadRoutingRuleFileGoodOptionalConfigs(t *testing.T) {
	mockDataStreamPath := "../testdata/routing_rules/good/no_namespace"
	rerouteProcessors, err := loadRoutingRuleFile(mockDataStreamPath)
	assert.NoError(t, err)
	assert.Equal(t, 1, len(rerouteProcessors))

	expectedProcessors := map[string]struct {
		expectedIf        string
		expectedDataset   []string
		expectedNamespace []string
	}{
		"missing_namespace": {
			"ctx['aws.cloudwatch.log_stream'].contains('Test2')",
			[]string{"aws.test2_logs"},
			nil,
		},
	}

	for _, rerouteProcessor := range rerouteProcessors {
		p := rerouteProcessor["reroute"].(RerouteProcessor)
		assert.Equal(t, expectedProcessors[p.Tag].expectedIf, p.If)
		assert.Equal(t, expectedProcessors[p.Tag].expectedDataset, p.Dataset)
		assert.Equal(t, expectedProcessors[p.Tag].expectedNamespace, p.Namespace)
	}
}

func TestLoadRoutingRuleFileBadMultipleSourceDataset(t *testing.T) {
	mockDataStreamPath := "../testdata/routing_rules/bad/multiple_source_dataset"
	rerouteProcessors, err := loadRoutingRuleFile(mockDataStreamPath)
	assert.Equal(t, 0, len(rerouteProcessors))
	assert.Error(t, err)
}

func TestLoadRoutingRuleFileBadNotString(t *testing.T) {
	mockDataStreamPath := "../testdata/routing_rules/bad/not_string"
	rerouteProcessors, err := loadRoutingRuleFile(mockDataStreamPath)
	assert.Equal(t, 0, len(rerouteProcessors))
	assert.Error(t, err)
}

func TestLoadRoutingRuleFileBadMissingConfigs(t *testing.T) {
	mockDataStreamPath := "../testdata/routing_rules/bad/missing_configs"
	rerouteProcessors, err := loadRoutingRuleFile(mockDataStreamPath)
	assert.Equal(t, 0, len(rerouteProcessors))
	assert.Error(t, err)
}
