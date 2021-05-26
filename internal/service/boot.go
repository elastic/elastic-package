// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package service

import (
	"time"

	"github.com/pkg/errors"

	"github.com/elastic/elastic-package/internal/configuration/locations"
	"github.com/elastic/elastic-package/internal/testrunner/runners/system"
	"github.com/elastic/elastic-package/internal/testrunner/runners/system/servicedeployer"
)

// Options define the details of the service which should be booted up.
type Options struct {
	ServiceName        string
	PackageRootPath    string
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
	locationManager, err := locations.NewLocationManager()
	if err != nil {
		return errors.Wrap(err, "reading service logs directory failed")
	}

	var serviceCtxt servicedeployer.ServiceContext
	serviceCtxt.Name = options.ServiceName
	serviceCtxt.Logs.Folder.Agent = system.ServiceLogsAgentDir
	serviceCtxt.Logs.Folder.Local = locationManager.ServiceLogDir()
	deployed, err := serviceDeployer.SetUp(serviceCtxt)
	if err != nil {
		return errors.Wrap(err, "can't set up the service deployer")
	}

	// TODO Wait for ctrl+c
	time.Sleep(15 * time.Second)

	// Tear down the service
	err = deployed.TearDown()
	if err != nil {
		return errors.Wrap(err, "can't tear down the service")
	}
	return nil
}
