// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package validation

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

const (
	builtPackageDir            = "testdata/built_package"
	builtPackageZip            = "testdata/built_package.zip"
	sourcePackageDir           = "testdata/source_package"
	composableSourcePackageDir = "testdata/composable_source_package"
)

func TestValidateSourceFromPath(t *testing.T) {
	tests := []struct {
		name        string
		dir         string
		expectError bool
	}{
		// source_package has _dev/; ModeSource allows it.
		{name: "valid source package", dir: sourcePackageDir},
		// built_package has _embedded_ecs dynamic templates auto-injected at build time;
		// ModeSource rejects them via ValidateNoEmbeddedEcsInDynamicTemplates, confirming
		// the built fixture is meaningfully distinct from the source fixture.
		{name: "rejects _embedded_ecs dynamic templates", dir: builtPackageDir, expectError: true},
		// composable_source_package uses package: in stream definitions; ModeSource allows it.
		{name: "valid composable source package", dir: composableSourcePackageDir},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			validationErr, filteredErr := ValidateSourceFromPath(tt.dir)
			if tt.expectError {
				assert.Error(t, validationErr)
				return
			}
			assert.NoError(t, validationErr)
			assert.NoError(t, filteredErr)
		})
	}
}

func TestValidateBuiltFromPath(t *testing.T) {
	tests := []struct {
		name        string
		dir         string
		expectError bool
	}{
		// built_package has no _dev/ or source-only artifacts; ModeBuild accepts it.
		{name: "valid built package", dir: builtPackageDir},
		// source_package has _dev/; ModeBuild rejects it via ValidateNoDevFolder.
		// Guards against regression if ValidateBuiltFromPath is ever switched back to ModeLegacy.
		{name: "rejects _dev/ directory", dir: sourcePackageDir, expectError: true},
		// composable_source_package has package: in stream definitions and policy template inputs;
		// ModeBuild rejects these via ValidateStreamInputMaterialized.
		{name: "rejects un-materialized package: references", dir: composableSourcePackageDir, expectError: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			validationErr, filteredErr := ValidateBuiltFromPath(tt.dir)
			if tt.expectError {
				assert.Error(t, validationErr)
				return
			}
			assert.NoError(t, validationErr)
			assert.NoError(t, filteredErr)
		})
	}
}

func TestValidateBuiltFromZip(t *testing.T) {
	validationErr, filteredErr := ValidateBuiltFromZip(builtPackageZip)
	assert.NoError(t, validationErr)
	assert.NoError(t, filteredErr)
}
