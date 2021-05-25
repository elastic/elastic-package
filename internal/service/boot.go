package service

import (
	"github.com/pkg/errors"

	"github.com/elastic/elastic-package/internal/testrunner/runners/system/servicedeployer"
)

type Options struct {
	PackageRootPath string
	DataStreamRootPath string
}

// BootUp function boots up the service stack.
func BootUp(options Options) error {
	serviceDeployer, err := servicedeployer.Factory(servicedeployer.FactoryOptions{
		PackageRootPath:    options.DataStreamRootPath,
		DataStreamRootPath: options.DataStreamRootPath,
	})
	if err != nil {
		return errors.Wrap(err, "can't create the service deployer instance")
	}

	// Boot up the service
	// TODO Create ServiceContext
	var serviceCtxt servicedeployer.ServiceContext
	deployed, err := serviceDeployer.SetUp(serviceCtxt)

	// TODO try to connect to the Elastic stack network

	// TODO Wait for ctrl+c

	// Tear down the service
	err = deployed.TearDown()
	if err != nil {
		return errors.Wrap(err, "can't tear down the service")
	}
	return nil
}