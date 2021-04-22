// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package surveyext

import (
	"fmt"
	"os"
)

// PackageAlreadyExists function checks if the package has been already created.
func PackageAlreadyExists(val interface{}) error {
	if baseDir, ok := val.(string); ok {
		_, err := os.Stat(baseDir)
		if err == nil {
			return fmt.Errorf(`package "%s" already exists`, baseDir)
		}
	}
	return nil
}
