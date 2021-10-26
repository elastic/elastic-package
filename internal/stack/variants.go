// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package stack

import (
	"fmt"
	"strings"
)

// stackVariantAsEnv function returns a stack variant based on the given stack version.
// We identified two variants:
// * default, covers all of 7.x branches
// * 8x, supports different configuration options in Kibana
func stackVariantAsEnv(version string) string {
	return fmt.Sprintf("STACK_VERSION_VARIANT=%s", selectStackVersion(version))
}

func selectStackVersion(version string) string {
	if strings.HasPrefix(version, "8.") {
		return "8x"
	}
	return "default"
}
