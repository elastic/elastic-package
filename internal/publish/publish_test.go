// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package publish

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/elastic/elastic-package/internal/packages"
)

func TestCreateRewriteResourcePath(t *testing.T) {
	rewrittenVersion := "0.4.1"
	f := createRewriteResourcePath(&packages.PackageManifest{
		Name:    "nginx",
		Version: rewrittenVersion,
	})

	resourceTemplate := "packages/nginx/%s/data_stream/stubstatus/agent/stream/stream.yml.hbs"
	resourcePath := fmt.Sprintf(resourceTemplate, "0.4.0")
	content := []byte("HELLO WORLD")

	actualResourcePath, actualContent := f(resourcePath, content)
	assert.Equal(t, string(content), string(actualContent))
	assert.Equal(t, fmt.Sprintf(resourceTemplate, rewrittenVersion), actualResourcePath)
}
