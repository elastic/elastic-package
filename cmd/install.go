// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"

	"github.com/elastic/elastic-package/internal/cobraext"
	"github.com/elastic/elastic-package/internal/common"
	"github.com/elastic/elastic-package/internal/kibana"
	"github.com/elastic/elastic-package/internal/packages"
	"github.com/elastic/elastic-package/internal/packages/installer"

	"github.com/elastic/elastic-package/internal/configuration/locations"
	"github.com/elastic/elastic-package/internal/testrunner/runners/system"
	"github.com/elastic/elastic-package/internal/testrunner/runners/system/servicedeployer"
)

const installLongDescription = `Use this command to install the package in Kibana.

The command uses Kibana API to install the package in Kibana. The package must be exposed via the Package Registry.`

func setupInstallCommand() *cobraext.Command {
	cmd := &cobra.Command{
		Use:   "install",
		Short: "Install the package",
		Long:  installLongDescription,
		RunE:  installCommandAction,
	}
	cmd.Flags().StringSliceP(cobraext.CheckConditionFlagName, "c", nil, cobraext.CheckConditionFlagDescription)
	cmd.Flags().StringP(cobraext.PackageRootFlagName, cobraext.PackageRootFlagShorthand, "", cobraext.PackageRootFlagDescription)

	return cobraext.NewCommand(cmd, cobraext.ContextPackage)
}

func installCommandAction(cmd *cobra.Command, _ []string) error {
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

	m, err := packages.ReadPackageManifestFromPackageRoot(packageRootPath)
	if err != nil {
		return errors.Wrapf(err, "reading package manifest failed (path: %s)", packageRootPath)
	}

	// Check conditions
	keyValuePairs, err := cmd.Flags().GetStringSlice(cobraext.CheckConditionFlagName)
	if err != nil {
		return errors.Wrap(err, "can't process check-condition flag")
	}
	if len(keyValuePairs) > 0 {
		cmd.Println("Check conditions for package")
		err = packages.CheckConditions(*m, keyValuePairs)
		if err != nil {
			return errors.Wrap(err, "checking conditions failed")
		}
		cmd.Println("Requirements satisfied - the package can be installed.")
		cmd.Println("Done")
		return nil
	}

	packageInstaller, err := installer.CreateForManifest(*m)
	if err != nil {
		return errors.Wrap(err, "can't create the package installer")
	}

	// Install the package
	cmd.Println("Install the package")
	installedPackage, err := packageInstaller.Install()
	if err != nil {
		return errors.Wrap(err, "can't install the package")
	}

	cmd.Println("Installed assets:")
	for _, asset := range installedPackage.Assets {
		cmd.Printf("- %s (type: %s)\n", asset.ID, asset.Type)
	}

	err = attach(cmd)
	if err != nil {
		return errors.Wrap(err, "can't attach the policy")
	}

	cmd.Println("Done")
	return nil
}

// - get package configuration
// - get package root
// - get package manifest
// - get data streams manifest
// - build integration
// - replace with variables configuration
func attach(cmd *cobra.Command) error {
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

	packageManifest, err := packages.ReadPackageManifestFromPackageRoot(packageRootPath)
	if err != nil {
		return errors.Wrapf(err, "reading package manifest failed (path: %s)", packageRootPath)
	}

	dataStreamsPath := filepath.Join(packageRootPath, "data_stream")

	var dsManifests []packages.DataStreamManifest
	dataStreams, err := os.ReadDir(dataStreamsPath)
	for _, dataStream := range dataStreams {
		dataStreamManifest, err := packages.ReadDataStreamManifest(filepath.Join(dataStreamsPath, dataStream.Name(), packages.DataStreamManifestFile))
		if err != nil {
			return errors.Wrapf(err, "reading data stream manifest failed (path: %s)", dataStream)
		}
		dsManifests = append(dsManifests, *dataStreamManifest)
	}

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

type packageConfig struct {
	PolicyTemplates []packages.PolicyTemplate
}

func isDataStreamEnabled() bool {
	return true
}

func createIntegrationPackageDatastream(
	kibanaPolicy kibana.Policy,
	pkgManifest packages.PackageManifest,
	dsManifests []packages.DataStreamManifest,
	config packageConfig,
) kibana.PackageDataStream {
	r := kibana.PackageDataStream{
		Name:      fmt.Sprintf("%s-foo", pkgManifest.Name),
		Namespace: "default",
		PolicyID:  kibanaPolicy.ID,
		Enabled:   true,
	}

	r.Package.Name = pkgManifest.Name
	r.Package.Title = pkgManifest.Title
	r.Package.Version = pkgManifest.Version

	for _, policyTemplate := range config.PolicyTemplates {
		for _, input := range policyTemplate.Inputs {
			inputType := input.Type

			packageInput := kibana.Input{
				Type:           inputType,
				PolicyTemplate: policyTemplate.Name,
				Enabled:        true,
				Vars: setKibanaVariables(pkgManifest.PolicyTemplates[0].Inputs[1].Vars, common.MapStr{
					"hosts":    []string{"https://elasticsearch:9200"},
					"username": "elastic",
					"password": "changeme",
				}),
			}

			for _, stream := range input.Streams {
				dataset := stream.Dataset

				for _, dsManifest := range dsManifests {
					if dsManifest.Dataset == dataset {
						stream := kibana.Stream{
							ID:      fmt.Sprintf("%s-%s.%s", inputType, pkgManifest.Name, dsManifest.Name),
							Enabled: true,
							DataStream: kibana.DataStream{
								Type:    dsManifest.Type,
								Dataset: dsManifest.Dataset,
							},
							Vars: setKibanaVariables(dsManifest.Streams[0].Vars, common.MapStr{}),
						}
						packageInput.Streams = append(packageInput.Streams, stream)

						fmt.Println("added stream with dataset \"%s\"", dataset)
						break
					}
				}

			}

			r.Inputs = append(r.Inputs, packageInput)
		}
	}

	return r
}

func setKibanaVariables(definitions []packages.Variable, values common.MapStr) kibana.Vars {
	vars := kibana.Vars{}
	for _, definition := range definitions {
		val := definition.Default

		value, err := values.GetValue(definition.Name)
		if err == nil {
			val = packages.VarValue{}
			val.Unpack(value)
		}

		vars[definition.Name] = kibana.Var{
			Type:  definition.Type,
			Value: val,
		}
	}
	return vars
}
