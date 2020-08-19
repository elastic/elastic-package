// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package system

import (
	"fmt"

	"github.com/pkg/errors"

	"github.com/elastic/elastic-package/internal/cluster"
	"github.com/elastic/elastic-package/internal/common"
	"github.com/elastic/elastic-package/internal/testrunner"
	"github.com/elastic/elastic-package/internal/testrunner/runners/system/service"
)

func init() {
	testrunner.RegisterRunner(TestType, Run)
}

const (
	// TestType defining system tests
	TestType testrunner.TestType = "system"
)

type runner struct {
	testFolderPath  string
	packageRootPath string
}

// Run runs the system tests defined under the given folder
func Run(options testrunner.TestOptions) error {
	r := runner{
		options.TestFolderPath,
		options.PackageRootPath,
	}
	return r.run()
}

func (r *runner) run() error {
	fmt.Println("system run", r.testFolderPath)

	// Step 1. Setup test cluster (ES + Kibana + Agent).
	if err := cluster.BootUp(false); err != nil {
		return errors.Wrap(err, "could not setup test cluster")
	}

	ctxt := common.MapStr{}
	ctxt.Put("DOCKER_COMPOSE_NETWORK", "TODO") // TODO: get value from cluster.BootUp()?

	// Step 2. Setup service.
	// Step 2a. (Deferred) Tear down service.
	serviceRunner, err := service.Factory(r.packageRootPath)
	if err != nil {
		return errors.Wrap(err, "could not create service runner")
	}

	ctxt, err = serviceRunner.SetUp(ctxt)
	defer serviceRunner.TearDown(ctxt)
	if err != nil {
		return errors.Wrap(err, "could not setup service")
	}

	// Step 3. Configure package (single data stream) via Ingest Manager APIs.

	// Step 4. (TODO in future) Optionally exercise service to generate load.

	// Step 5. Assert that there's expected data in data stream.

	return nil
}
