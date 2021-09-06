// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package service

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/elastic/elastic-package/internal/logger"

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

	Variant string
}

// BootUp function boots up the service stack.
func BootUp(options Options) error {
	logger.Debugf("Create new instance of the service deployer")
	serviceDeployer, err := servicedeployer.Factory(servicedeployer.FactoryOptions{
		PackageRootPath:    options.DataStreamRootPath,
		DataStreamRootPath: options.DataStreamRootPath,
		Variant:            options.Variant,
	})
	if err != nil {
		return errors.Wrap(err, "can't create the service deployer instance")
	}

	// Boot up the service
	logger.Debugf("Boot up the service instance")
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

	fmt.Println("Service is up, please use ctrl+c to take it down")
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, os.Interrupt, syscall.SIGTERM)
	<-ch

	// Tear down the service
	fmt.Println("Take down the service")
	err = deployed.TearDown()
	if err != nil {
		return errors.Wrap(err, "can't tear down the service")
	}
	return nil
}
