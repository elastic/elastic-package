// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package corpusgenerator

import (
	"bytes"
	"io"
	"testing"

	"github.com/elastic/elastic-integration-corpus-generator-tool/pkg/genlib"
	"github.com/stretchr/testify/assert"
)

type mockClient struct {
}

func (c mockClient) GetGoTextTemplate(packageName, dataStreamName string) ([]byte, error) {
	return []byte("7 bytes"), nil
}
func (c mockClient) GetConf(packageName, dataStreamName string) (genlib.Config, error) {
	return genlib.Config{}, nil
}
func (c mockClient) GetFields(packageName, dataStreamName string) (genlib.Fields, error) {
	return genlib.Fields{}, nil
}

func TestGeneratorEmitTotEvents(t *testing.T) {
	generator, err := NewGenerator(mockClient{}, "packageName", "dataSetName", 7)
	assert.NoError(t, err)

	totEvents := 0
	buf := bytes.NewBufferString("")
	for {
		err := generator.Emit(buf)
		if err == io.EOF {
			break
		}

		totEvents += 1
	}

	assert.Equal(t, 7, totEvents, "expected 7 totEvents, got %d", totEvents)
}
