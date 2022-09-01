// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package runners

import (
	// Registered test runners
	_ "github.com/elastic/elastic-package/internal/benchrunner/runners/asset"
	_ "github.com/elastic/elastic-package/internal/benchrunner/runners/pipeline"
	_ "github.com/elastic/elastic-package/internal/benchrunner/runners/static"
	_ "github.com/elastic/elastic-package/internal/benchrunner/runners/system"
)
