package runners

import (
	// Registered test runners
	_ "github.com/elastic/elastic-package/internal/testrunner/runners/pipeline"
	_ "github.com/elastic/elastic-package/internal/testrunner/runners/system"
)
