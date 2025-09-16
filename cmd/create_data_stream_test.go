// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package cmd

import (
	"testing"

	"github.com/AlecAivazis/survey/v2"
	"github.com/Masterminds/semver/v3"
	"github.com/stretchr/testify/assert"
)

func TestGetSurveyQuestionsForVersion_BelowSemver3_2_0(t *testing.T) {
	version := semver.MustParse("3.1.9")
	questions := getInitialSurveyQuestionsForVersion(version)

	assert.Len(t, questions, 3, "should return 3 questions for spec version < 3.2.0")

	assert.Equal(t, "name", questions[0].Name)
	assert.IsType(t, &survey.Input{}, questions[0].Prompt)
	assert.Equal(t, "title", questions[1].Name)
	assert.IsType(t, &survey.Input{}, questions[1].Prompt)
	assert.Equal(t, "type", questions[2].Name)
	assert.IsType(t, &survey.Select{}, questions[2].Prompt)
}

func TestGetSurveyQuestionsForVersion_EqualSemver3_2_0(t *testing.T) {
	version := semver.MustParse("3.2.0")
	questions := getInitialSurveyQuestionsForVersion(version)

	assert.Len(t, questions, 4, "should return 4 questions for spec version >= 3.2.0")

	assert.Equal(t, "subobjects", questions[3].Name)
	assert.IsType(t, &survey.Confirm{}, questions[3].Prompt)
}

func TestGetSurveyQuestionsForVersion_AboveSemver3_2_0(t *testing.T) {
	version := semver.MustParse("3.3.0")
	questions := getInitialSurveyQuestionsForVersion(version)

	assert.Len(t, questions, 4, "should return 4 questions for spec version > 3.2.0")

	assert.Equal(t, "subobjects", questions[3].Name)
	assert.IsType(t, &survey.Confirm{}, questions[3].Prompt)
}
func TestCreateDataStreamDescriptorFromAnswers_SubobjectsFalseForSpecVersionBelow3_2_0(t *testing.T) {
	specVersion := semver.MustParse("3.1.0")
	answers := newDataStreamAnswers{
		Name:       "test_stream",
		Title:      "Test Stream",
		Type:       "logs",
		Subobjects: false,
	}
	descriptor := createDataStreamDescriptorFromAnswers(answers, "/tmp/package", specVersion)

	assert.Equal(t, "test_stream", descriptor.Manifest.Name)
	assert.Equal(t, "Test Stream", descriptor.Manifest.Title)
	assert.Equal(t, "logs", descriptor.Manifest.Type)
	assert.Equal(t, "/tmp/package", descriptor.PackageRoot)
	assert.Nil(t, descriptor.Manifest.Elasticsearch)
}

func TestCreateDataStreamDescriptorFromAnswers_SubobjectsFalseForSpecVersionGTE3_2_0(t *testing.T) {
	specVersion := semver.MustParse("3.2.0")
	answers := newDataStreamAnswers{
		Name:       "test_stream",
		Title:      "Test Stream",
		Type:       "logs",
		Subobjects: false,
	}
	descriptor := createDataStreamDescriptorFromAnswers(answers, "/tmp/package", specVersion)

	assert.NotNil(t, descriptor.Manifest.Elasticsearch)
	assert.NotNil(t, descriptor.Manifest.Elasticsearch.IndexTemplate)
	assert.NotNil(t, descriptor.Manifest.Elasticsearch.IndexTemplate.Mappings)
	assert.False(t, descriptor.Manifest.Elasticsearch.IndexTemplate.Mappings.Subobjects)
}

func TestCreateDataStreamDescriptorFromAnswers_SubobjectsTrueForSpecVersionGTE3_2_0(t *testing.T) {
	specVersion := semver.MustParse("3.2.0")
	answers := newDataStreamAnswers{
		Name:       "test_stream",
		Title:      "Test Stream",
		Type:       "logs",
		Subobjects: true,
	}
	descriptor := createDataStreamDescriptorFromAnswers(answers, "/tmp/package", specVersion)

	require.NotNil(t, descriptor.Manifest.Elasticsearch)
	require.NotNil(t, descriptor.Manifest.Elasticsearch.IndexTemplate)
	require.NotNil(t, descriptor.Manifest.Elasticsearch.IndexTemplate.Mappings)
	assert.False(t, descriptor.Manifest.Elasticsearch.IndexTemplate.Mappings.Subobjects)
}
