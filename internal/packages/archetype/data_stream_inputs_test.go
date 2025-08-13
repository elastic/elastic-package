// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package archetype

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/elastic/elastic-package/internal/packages"
)

func TestDataStreamInputs(t *testing.T) {
	t.Run("valid", func(t *testing.T) {
		descriptor := DataStreamDescriptor{
			Manifest: packages.DataStreamManifest{
				Name:  "test",
				Title: "Test",
				Type:  "log",
				Streams: []packages.Stream{
					{
						Input: "udp",
					},
					{
						Input: "gcs",
					},
				},
			},
			PackageRoot: "",
		}

		err := populateInputVariables(&descriptor)

		assert.Nil(t, err)
		assert.Len(t, descriptor.Manifest.Streams[0].Vars, 14)
		assert.Len(t, descriptor.Manifest.Streams[1].Vars, 16)
	})
}
