// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package cmd

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/elastic/elastic-package/internal/licenses"
	"github.com/elastic/elastic-package/internal/tui"
)

func TestCreatePackageDescriptorFromAnswers_Integration(t *testing.T) {
	answers := newPackageAnswers{
		Type:                "integration",
		Name:                "my_integration",
		Version:             "0.0.1",
		SourceLicense:       licenses.Elastic20,
		Title:               "my_integration",
		Description:         "This is a new package.",
		Categories:          []string{"custom"},
		KibanaVersion:       tui.DefaultKibanaVersionConditionValue(),
		ElasticSubscription: "basic",
		GithubOwner:         "elastic/integrations",
		OwnerType:           "elastic",
	}

	descriptor := createPackageDescriptorFromAnswers(answers)

	assert.Equal(t, "my_integration", descriptor.Manifest.Name)
	assert.Equal(t, "my_integration", descriptor.Manifest.Title)
	assert.Equal(t, "integration", descriptor.Manifest.Type)
	assert.Equal(t, "0.0.1", descriptor.Manifest.Version)
	assert.Equal(t, licenses.Elastic20, descriptor.Manifest.Source.License)
	assert.Equal(t, "This is a new package.", descriptor.Manifest.Description)
	assert.Equal(t, []string{"custom"}, descriptor.Manifest.Categories)
	assert.Equal(t, "basic", descriptor.Manifest.Conditions.Elastic.Subscription)
	assert.Equal(t, "elastic/integrations", descriptor.Manifest.Owner.Github)
	assert.Equal(t, "elastic", descriptor.Manifest.Owner.Type)
	assert.Empty(t, descriptor.InputDataStreamType)
	assert.Nil(t, descriptor.Manifest.Elasticsearch)
}

func TestCreatePackageDescriptorFromAnswers_Input(t *testing.T) {
	answers := newPackageAnswers{
		Type:                "input",
		Name:                "my_input",
		Version:             "0.0.1",
		SourceLicense:       licenses.Elastic20,
		Title:               "my_input",
		Description:         "This is a new package.",
		Categories:          []string{"custom"},
		KibanaVersion:       tui.DefaultKibanaVersionConditionValue(),
		ElasticSubscription: "basic",
		GithubOwner:         "elastic/integrations",
		OwnerType:           "elastic",
		DataStreamType:      "logs",
		Subobjects:          false,
	}

	descriptor := createPackageDescriptorFromAnswers(answers)

	assert.Equal(t, "input", descriptor.Manifest.Type)
	assert.Equal(t, "logs", descriptor.InputDataStreamType)
	require.NotNil(t, descriptor.Manifest.Elasticsearch)
	require.NotNil(t, descriptor.Manifest.Elasticsearch.IndexTemplate)
	require.NotNil(t, descriptor.Manifest.Elasticsearch.IndexTemplate.Mappings)
	assert.False(t, descriptor.Manifest.Elasticsearch.IndexTemplate.Mappings.Subobjects)
}

func TestCreatePackageDescriptorFromAnswers_InputWithSubobjects(t *testing.T) {
	answers := newPackageAnswers{
		Type:                "input",
		Name:                "my_input",
		Version:             "0.0.1",
		SourceLicense:       licenses.Elastic20,
		Title:               "my_input",
		Description:         "This is a new package.",
		Categories:          []string{"custom"},
		KibanaVersion:       tui.DefaultKibanaVersionConditionValue(),
		ElasticSubscription: "basic",
		GithubOwner:         "elastic/integrations",
		OwnerType:           "elastic",
		DataStreamType:      "logs",
		Subobjects:          true,
	}

	descriptor := createPackageDescriptorFromAnswers(answers)

	assert.Nil(t, descriptor.Manifest.Elasticsearch)
}

func TestCreatePackageDescriptorFromAnswers_NoLicense(t *testing.T) {
	answers := newPackageAnswers{
		Type:                "integration",
		Name:                "no_license_pkg",
		Version:             "0.0.1",
		SourceLicense:       noLicenseValue,
		Title:               "No License",
		Description:         "Package without a license.",
		Categories:          []string{"custom"},
		KibanaVersion:       tui.DefaultKibanaVersionConditionValue(),
		ElasticSubscription: "basic",
		GithubOwner:         "elastic/integrations",
		OwnerType:           "elastic",
	}

	descriptor := createPackageDescriptorFromAnswers(answers)

	assert.Empty(t, descriptor.Manifest.Source.License)
}

func TestAllowedPackageTypes(t *testing.T) {
	valid := []string{"input", "integration", "content"}
	for _, v := range valid {
		found := false
		for _, a := range allowedPackageTypes {
			if a == v {
				found = true
				break
			}
		}
		assert.True(t, found, "expected %q to be in allowedPackageTypes", v)
	}

	invalid := []string{"", "foo", "INPUT", "Integration"}
	for _, v := range invalid {
		found := false
		for _, a := range allowedPackageTypes {
			if a == v {
				found = true
				break
			}
		}
		assert.False(t, found, "expected %q to NOT be in allowedPackageTypes", v)
	}
}
