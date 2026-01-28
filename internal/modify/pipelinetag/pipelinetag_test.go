// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package pipelinetag

import (
	"errors"
	"testing"

	"github.com/goccy/go-yaml"
	"github.com/goccy/go-yaml/ast"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/elastic/elastic-package/internal/fleetpkg"
	"github.com/elastic/elastic-package/internal/yamledit"
)

func getNodeString(f *ast.File, path string) (string, error) {
	p, err := yaml.PathString(path)
	if err != nil {
		return "", err
	}

	n, err := p.FilterFile(f)
	if err != nil {
		return "", err
	}

	sn, ok := n.(*ast.StringNode)
	if !ok {
		return "", errors.New("expected string node")
	}

	return sn.Value, nil
}

func assertProcessorTag(t *testing.T, pipeline *fleetpkg.Pipeline, path string, want string) {
	got, err := getNodeString(pipeline.Doc.AST(), path)
	assert.NoError(t, err)
	assert.Equal(t, want, got)
}

func Test_GenerateTags(t *testing.T) {
	var pipeline fleetpkg.Pipeline
	_, err := yamledit.ParseDocumentFile("testdata/default.yml", &pipeline)
	require.NoError(t, err)

	err = processPipeline(&pipeline)
	require.NoError(t, err)

	assert.True(t, pipeline.Doc.Modified())

	assertProcessorTag(t, &pipeline, "$.processors[0].set.tag", "set_sample_field_71a88542")
	assertProcessorTag(t, &pipeline, "$.processors[1].set.tag", "valid_tag")
	assertProcessorTag(t, &pipeline, "$.processors[1].set.on_failure[0].set.tag", "set_sample_field_a7a6e0d7")
}
