// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package cmd

import (
	"fmt"
	"path/filepath"
	"time"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"

	"github.com/elastic/elastic-package/internal/cobraext"
	"github.com/elastic/elastic-package/internal/kibana"
	"github.com/elastic/elastic-package/internal/logger"
	"github.com/elastic/elastic-package/internal/packages"

	"github.com/elastic/elastic-package/internal/configuration/locations"
	"github.com/elastic/elastic-package/internal/testrunner/runners/system"
	"github.com/elastic/elastic-package/internal/testrunner/runners/system/servicedeployer"
)

const enrollLongDescription = `Use this command to enroll an agent in Fleet.

The command spawns a new agent automatically enrolled in Fleet. A policy configuration is provided as a flag to attach integrations to the agent.`

func setupEnrollCommand() *cobraext.Command {
	cmd := &cobra.Command{
		Use:   "enroll",
		Short: "Enroll an agent with a preconfigured policy",
		Long:  enrollLongDescription,
		RunE:  enrollCommandAction,
	}
	cmd.Flags().StringP(cobraext.PackageRootFlagName, cobraext.PackageRootFlagShorthand, "", cobraext.PackageRootFlagDescription)
	cmd.Flags().StringP("config", "C", "", "Policy configuration file")

	return cobraext.NewCommand(cmd, cobraext.ContextPackage)
}

/**
 * Creates a docker container with a running agent connected to the stack network.
 * The command accepts a policy configuration that can define which package and
 * datasets the agent should collect. the policy is currently mandatory but could
 * be made optional, agent would fallback to the default Elastic Package policy.
 * agents are spawned with a unique container name in the elastic-package-service
 * project and can be run in parallel.
 */
func enrollCommandAction(cmd *cobra.Command, _ []string) error {
	packageRootPath, err := cmd.Flags().GetString(cobraext.PackageRootFlagName)
	if err != nil {
		return cobraext.FlagParsingError(err, cobraext.PackageRootFlagName)
	}
	if packageRootPath == "" {
		var found bool
		packageRootPath, found, err = packages.FindPackageRoot()
		if !found {
			return errors.New("package root not found")
		}
		if err != nil {
			return errors.Wrap(err, "locating package root failed")
		}
	}

	policyConfigurationPath, err := cmd.Flags().GetString("config")
	if err != nil {
		return cobraext.FlagParsingError(err, cobraext.PackageRootFlagName)
	}

	_, err = Enroll(EnrollOptions{packageRootPath, policyConfigurationPath})
	if err != nil {
		return errors.Wrap(err, "can't enroll agent")
	}

	cmd.Println("Done")
	return nil
}

type EnrollOptions struct {
	packageRootPath         string
	policyConfigurationPath string
}

func Enroll(options EnrollOptions) (servicedeployer.DeployedService, error) {
	kib, err := kibana.NewClient()
	if err != nil {
		return nil, errors.Wrap(err, "can't create Kibana client")
	}

	locationManager, err := locations.NewLocationManager()
	if err != nil {
		return nil, errors.Wrap(err, "reading service logs directory failed")
	}

	packageName := filepath.Base(options.packageRootPath)
	policyID := fmt.Sprintf("elastic-package_%s_%d", packageName, time.Now().UnixNano())

	var serviceCtxt servicedeployer.ServiceContext
	serviceCtxt.Logs.Folder.Agent = system.ServiceLogsAgentDir
	serviceCtxt.Logs.Folder.Local = locationManager.ServiceLogDir()
	serviceCtxt.CustomProperties = map[string]interface{}{
		"policy_name": policyID,
		"tags":        []string{packageName},
	}

	configPath := filepath.Join(options.packageRootPath, "_dev", "policy", options.policyConfigurationPath)
	config, err := system.NewPackageConfig(configPath, options.packageRootPath, serviceCtxt)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create package configuration")
	}
	packageDataStreams := config.CreatePackageDataStreams(policyID)

	_, err = kib.CreatePolicy(kibana.Policy{
		ID:          policyID,
		Name:        policyID,
		Description: "",
		Namespace:   "default",
	})
	if err != nil {
		return nil, errors.Wrap(err, "failed to create policy")
	}

	err = kib.AddPackageDataStreamToPolicy(packageDataStreams)
	if err != nil {
		return nil, errors.Wrap(err, "failed to add data streams to policy")
	}

	logger.Debug("deploying agent with policy ", policyID)
	service, err := deployAgent(serviceCtxt)
	if err != nil {
		return nil, errors.Wrap(err, "failed to deploy agent")
	}
	logger.Debug("deployed agent ", service.Context().Name)

	return service, nil
}

func deployAgent(ctx servicedeployer.ServiceContext) (servicedeployer.DeployedService, error) {
	deployer := servicedeployer.NewAgentDeployer()
	service, err := deployer.SetUp(ctx)
	if err != nil {
		return nil, err
	}
	return service, nil
}
