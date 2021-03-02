// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package packages

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCheckConditions_InvalidRelease(t *testing.T) {
	manifest := PackageManifest{
		Conditions: Conditions{
			Kibana: KibanaConditions{Version: "^7.11.0"},
		},
		PolicyTemplates: nil,
	}

	err := CheckConditions(manifest, []string{"kibana.version=7.10.1"})
	assert.Error(t, err, "package requires higher major version")
}

func TestCheckConditions_ValidRelease(t *testing.T) {
	manifest := PackageManifest{
		Conditions: Conditions{
			Kibana: KibanaConditions{Version: "^7.11.0"},
		},
		PolicyTemplates: nil,
	}

	err := CheckConditions(manifest, []string{"kibana.version=7.11.1"})
	assert.NoError(t, err)
}

func TestCheckConditions_InvalidSnapshot(t *testing.T) {
	manifest := PackageManifest{
		Conditions: Conditions{
			Kibana: KibanaConditions{Version: "^7.11.0"},
		},
		PolicyTemplates: nil,
	}

	err := CheckConditions(manifest, []string{"kibana.version=7.10.1-SNAPSHOT"})
	assert.Error(t, err, "package requires higher major version")
}

func TestCheckConditions_ValidSnapshot(t *testing.T) {
	manifest := PackageManifest{
		Conditions: Conditions{
			Kibana: KibanaConditions{Version: "^7.11.0"},
		},
		PolicyTemplates: nil,
	}

	err := CheckConditions(manifest, []string{"kibana.version=7.11.1-SNAPSHOT"})
	assert.NoError(t, err)
}
