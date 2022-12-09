// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package cmd

import (
	"github.com/pkg/errors"
	"github.com/spf13/cobra"

	"github.com/elastic/elastic-package/internal/cobraext"
	"github.com/elastic/elastic-package/internal/kibana"
	"github.com/elastic/elastic-package/internal/packages"

	"github.com/elastic/elastic-package/internal/configuration/locations"
	"github.com/elastic/elastic-package/internal/testrunner/runners/system"
	"github.com/elastic/elastic-package/internal/testrunner/runners/system/servicedeployer"
)

const enrollLongDescription = `Use this command to enroll an agent in Fleet.

The command spawns a new agent automatically enrolled in Fleet. A policy configuration can be provided to the command to attach integrations to the agent.`

func setupEnrollCommand() *cobraext.Command {
	cmd := &cobra.Command{
		Use:   "enroll",
		Short: "Enroll an agent",
		Long:  enrollLongDescription,
		RunE:  enrollCommandAction,
	}
	cmd.Flags().StringP(cobraext.PackageRootFlagName, cobraext.PackageRootFlagShorthand, "", cobraext.PackageRootFlagDescription)

	return cobraext.NewCommand(cmd, cobraext.ContextPackage)
}

func enrollCommandAction(cmd *cobra.Command, _ []string) error {
	packageRootPath, err := cmd.Flags().GetString(cobraext.PackageRootFlagName)
	if err != nil {
		return cobraext.FlagParsingError(err, cobraext.PackageRootFlagName)
	}

	err = enroll(cmd)
	if err != nil {
		return errors.Wrap(err, "can't enroll agent")
	}

	cmd.Println("Done")
	return nil
}

func enroll(cmd *cobra.Command) error {
	kib, err := kibana.NewClient()
	if err != nil {
		return errors.Wrap(err, "can't create Kibana client")
	}

	p := kibana.Policy{
		Name:        "elasticsearch-package",
		Description: "",
		Namespace:   "default",
	}
	policy, err := kib.CreatePolicy(p)
	if err != nil {
		return err
	}

	config := packageConfig{
		PolicyTemplates: []packages.PolicyTemplate{
			{
				Name: "elasticsearch",
				Inputs: []packages.Input{
					{
						Type: "elasticsearch/metrics",
						Streams: []packages.ManifestStream{
							{
								Dataset: "elasticsearch.stack_monitoring.cluster_stats",
								Vars:    []packages.Variable{},
							},
						},
					},
				},
			},
		},
	}
	packageDs := createIntegrationPackageDatastream(kibana.Policy{ID: policy.ID}, *packageManifest, dsManifests, config)

	err = kib.AddPackageDataStreamToPolicy(packageDs)
	if err != nil {
		return errors.Wrap(err, "failed to add data streams to policy")
	}

	deployer, _ := servicedeployer.NewCustomAgentDeployer("")
	locationManager, err := locations.NewLocationManager()
	if err != nil {
		return errors.Wrap(err, "reading service logs directory failed")
	}

	var serviceCtxt servicedeployer.ServiceContext
	serviceCtxt.Name = "elasticsearch-package_agent"
	serviceCtxt.Logs.Folder.Agent = system.ServiceLogsAgentDir
	serviceCtxt.Logs.Folder.Local = locationManager.ServiceLogDir()
	serviceCtxt.CustomProperties = map[string]interface{}{
		"tags":        "elasticsearch,v870",
		"policy_name": policy.Name,
	}

	_, err = deployer.SetUp(serviceCtxt)
	if err != nil {
		return err
	}

	return nil
}
