// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package service

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/elastic/elastic-package/internal/common"
	"github.com/elastic/elastic-package/internal/logger"
	"github.com/elastic/elastic-package/internal/profile"

	"github.com/elastic/elastic-package/internal/configuration/locations"
	"github.com/elastic/elastic-package/internal/servicedeployer"
	"github.com/elastic/elastic-package/internal/testrunner/runners/system"
)

// Options define the details of the service which should be booted up.
type Options struct {
	Profile *profile.Profile

	ServiceName        string
	PackageRootPath    string
	DevDeployDir       string
	DataStreamRootPath string
	StackVersion       string
	AgentVersion       string

	Variant string
}

// BootUp function boots up the service stack.
func BootUp(ctx context.Context, options Options) error {
	logger.Debugf("Create new instance of the service deployer")
	serviceDeployer, err := servicedeployer.Factory(servicedeployer.FactoryOptions{
		Profile:                options.Profile,
		PackageRootPath:        options.DataStreamRootPath,
		DataStreamRootPath:     options.DataStreamRootPath,
		DevDeployDir:           options.DevDeployDir,
		Variant:                options.Variant,
		StackVersion:           options.StackVersion,
		DeployIndependentAgent: false,
		AgentVersion:           options.AgentVersion,
	})
	if errors.Is(err, os.ErrNotExist) {
		fmt.Println("No service defined.")
		return nil
	}
	if err != nil {
		return fmt.Errorf("can't create the service deployer instance: %w", err)
	}

	// Boot up the service
	logger.Debugf("Boot up the service instance")
	locationManager, err := locations.NewLocationManager()
	if err != nil {
		return fmt.Errorf("reading service logs directory failed: %w", err)
	}

	var svcInfo servicedeployer.ServiceInfo
	svcInfo.Name = options.ServiceName
	svcInfo.Logs.Folder.Agent = system.ServiceLogsAgentDir
	svcInfo.Logs.Folder.Local = locationManager.ServiceLogDir()
	svcInfo.Test.RunID = common.CreateTestRunID()
	deployed, err := serviceDeployer.SetUp(ctx, svcInfo)
	if err != nil {
		return fmt.Errorf("can't set up the service deployer: %w", err)
	}

	fmt.Println("Service is up, please use ctrl+c to take it down")
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(ch)
	<-ch

	// Tear down the service
	fmt.Println("Take down the service")
	err = deployed.TearDown(ctx)
	if err != nil {
		return fmt.Errorf("can't tear down the service: %w", err)
	}
	return nil
}
