// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package export

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/elastic/elastic-package/internal/common"
)

func TestTransform(t *testing.T) {
	b, err := os.ReadFile("./testdata/system-navigation.json")
	require.NoError(t, err)

	var given common.MapStr
	err = json.Unmarshal(b, &given)
	require.NoError(t, err)

	ctx := &transformationContext{
		packageName: "system",
	}

	results, err := applyTransformations(ctx, []common.MapStr{given})
	require.NoError(t, err)
	require.Len(t, results, 1)

	result, err := json.MarshalIndent(&results[0], "", "  ")
	require.NoError(t, err)

	expected, err := os.ReadFile("./test/system-navigation.json-expected.json")
	require.NoError(t, err)

	require.Equal(t, string(expected), string(result))
}
