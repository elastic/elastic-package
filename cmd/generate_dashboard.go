// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/elastic/elastic-package/internal/cobraext"
	"github.com/elastic/elastic-package/internal/install"
	"github.com/elastic/elastic-package/internal/kibana"
	"github.com/elastic/elastic-package/internal/packages"
	"github.com/elastic/elastic-package/internal/stack"

	"io/ioutil"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v2"

	jsoniter "github.com/json-iterator/go"
)

const generateDashboardLongDescription = `Use this command to generateDashboard assets relevant for the package, e.g. Kibana dashboards.`

const generateDashboardDashboardsLongDescription = `Use this command to generateDashboard dashboards with referenced objects from the Kibana instance.

Use this command to download selected dashboards and other associated saved objects from Kibana. This command adjusts the downloaded saved objects according to package naming conventions (prefixes, unique IDs) and writes them locally into folders corresponding to saved object types (dashboard, visualization, map, etc.).`

func setupGenerateDashboardCommand() *cobraext.Command {
	generateDashboardDashboardCmd := &cobra.Command{
		Use:   "dashboards",
		Short: "generate kibana dashboards",
		Long:  generateDashboardDashboardsLongDescription,
		Args:  cobra.NoArgs,
		RunE:  generateDashboardDashboardsCmd,
	}
	generateDashboardDashboardCmd.Flags().Bool(cobraext.TLSSkipVerifyFlagName, false, cobraext.TLSSkipVerifyFlagDescription)
	generateDashboardDashboardCmd.Flags().Bool(cobraext.AllowSnapshotFlagName, false, cobraext.AllowSnapshotDescription)

	cmd := &cobra.Command{
		Use:   "generate",
		Short: "Generate package assets",
		Long:  generateDashboardLongDescription,
	}
	cmd.AddCommand(generateDashboardDashboardCmd)
	cmd.PersistentFlags().StringP(cobraext.ProfileFlagName, "p", "", fmt.Sprintf(cobraext.ProfileFlagDescription, install.ProfileNameEnvVar))

	return cobraext.NewCommand(cmd, cobraext.ContextPackage)
}

func generateDashboardDashboardsCmd(cmd *cobra.Command, args []string) error {
	cmd.Println("GenerateDashboard Kibana dashboards")

	var opts []kibana.ClientOption
	tlsSkipVerify, _ := cmd.Flags().GetBool(cobraext.TLSSkipVerifyFlagName)
	if tlsSkipVerify {
		opts = append(opts, kibana.TLSSkipVerify())
	}

	allowSnapshot, _ := cmd.Flags().GetBool(cobraext.AllowSnapshotFlagName)

	profile, err := cobraext.GetProfileFlag(cmd)
	if err != nil {
		return err
	}

	kibanaClient, err := stack.NewKibanaClientFromProfile(profile, opts...)
	if err != nil {
		return fmt.Errorf("can't create Kibana client: %w", err)
	}

	kibanaVersion, err := kibanaClient.Version()
	if err != nil {
		return fmt.Errorf("can't get Kibana status information: %w", err)
	}

	if kibanaVersion.IsSnapshot() {
		message := fmt.Sprintf("generateDashboarding dashboards from a SNAPSHOT version of Kibana (%s) is discouraged. It could lead to invalid dashboards (for example if they use features that are reverted or modified before the final release)", kibanaVersion.Version())
		if !allowSnapshot {
			return fmt.Errorf("%s. --%s flag can be used to ignore this error", message, cobraext.AllowSnapshotFlagName)
		}
		fmt.Printf("Warning: %s\n", message)
	}

	packageRoot, err := packages.MustFindPackageRoot()
	if err != nil {
		return fmt.Errorf("locating package root failed: %w", err)
	}

	cmd.Println(packageRoot)

	// Initialize the map to store parsed YAML data
	yamlData := make(map[string]interface{})

	// Recursively scan the directory
	err = filepath.Walk(packageRoot, func(path string, info os.FileInfo, err error) error {
		// Check for errors
		if err != nil {
			return err
		}
		// Skip directories
		if info.IsDir() {
			return nil
		}
		// Check if the file has a .yml extension
		if filepath.Ext(path) == ".yml" {
			// Read the file
			fileData, err := ioutil.ReadFile(path)
			if err != nil {
				return err
			}
			// Parse the YAML data
			var parsedData interface{}
			err = yaml.Unmarshal(fileData, &parsedData)
			if err != nil {
				return err
			}
			relativePath, err := filepath.Rel(packageRoot, path)
			// Store the parsed data in the map
			yamlData[relativePath] = parsedData
		}
		return nil
	})

	// Check for errors during directory scanning
	if err != nil {
		fmt.Println("Error:", err)
		return nil
	}

	// Print the parsed YAML data
	for filePath, data := range yamlData {
		fmt.Printf("File: %s\n", filePath)
		// get the first 300 characters of the data
		truncated_data := fmt.Sprintf("%v", data)
		fmt.Printf("Data: %#v\n", truncated_data)
	}

	// Convert the map to JSON
	var json = jsoniter.ConfigCompatibleWithStandardLibrary
	jsonData, err := json.Marshal(yamlData)
	if err != nil {
		fmt.Println("Error marshalling JSON:", err)
		return nil
	}

	fmt.Println(string(jsonData))
	kibanaClient.SendRequest(cmd.Context(), "POST", "/api/dashboard_generator/generate", jsonData)

	cmd.Println("Done")
	return nil
}
