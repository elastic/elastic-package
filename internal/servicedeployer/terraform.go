// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package servicedeployer

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/elastic/go-resource"

	"github.com/elastic/elastic-package/internal/common"
	"github.com/elastic/elastic-package/internal/compose"
	"github.com/elastic/elastic-package/internal/configuration/locations"
	"github.com/elastic/elastic-package/internal/files"
	"github.com/elastic/elastic-package/internal/logger"
)

const (
	terraformDeployerDir        = "terraform"
	terraformDeployerYml        = "terraform-deployer.yml"
	terraformDeployerDockerfile = "Dockerfile"
	terraformDeployerRun        = "run.sh"
	terraformOutputPrefix       = "TF_OUTPUT_"
	terraformOutputJSONFile     = "tfOutputValues.json"
)

//go:embed _static/terraform_deployer.yml
var terraformDeployerYmlContent string

//go:embed _static/terraform_deployer_run.sh
var terraformDeployerRunContent string

//go:embed _static/Dockerfile.terraform_deployer
var terraformDeployerDockerfileContent string

// TerraformServiceDeployer is responsible for deploying infrastructure described with Terraform definitions.
type TerraformServiceDeployer struct {
	definitionsDir string
}

type TerraformServiceDeployerOptions struct {
	DefinitionsDir string
}

// addTerraformOutputs method reads the terraform outputs generated in the json format and
// adds them to the custom properties of ServiceInfo and can be used in the handlebars template
// like `{{TF_OUTPUT_queue_url}}` where `queue_url` is the output configured
func addTerraformOutputs(svcInfo *ServiceInfo) error {
	// Read the `output.json` file where terraform outputs are generated
	outputFile := filepath.Join(svcInfo.OutputDir, terraformOutputJSONFile)
	content, err := os.ReadFile(outputFile)
	if err != nil {
		return fmt.Errorf("failed to read terraform output file: %w", err)
	}

	// https://github.com/hashicorp/terraform/blob/v1.4.6/internal/command/views/output.go#L217-L222
	type OutputMeta struct {
		Value interface{} `json:"value"`
	}

	// Unmarshall the data into `terraformOutputs`
	logger.Debug("Unmarshalling terraform output JSON")
	var terraformOutputs map[string]OutputMeta
	if err = json.Unmarshal(content, &terraformOutputs); err != nil {
		return fmt.Errorf("error during JSON Unmarshal: %w", err)
	}

	if len(terraformOutputs) == 0 {
		return nil
	}

	if svcInfo.CustomProperties == nil {
		svcInfo.CustomProperties = make(map[string]any, len(terraformOutputs))
	}
	// Prefix variables names with TF_OUTPUT_
	for k, outputs := range terraformOutputs {
		svcInfo.CustomProperties[terraformOutputPrefix+k] = outputs.Value
	}
	return nil
}

// NewTerraformServiceDeployer creates an instance of TerraformServiceDeployer.
func NewTerraformServiceDeployer(opts TerraformServiceDeployerOptions) (*TerraformServiceDeployer, error) {
	return &TerraformServiceDeployer{
		definitionsDir: opts.DefinitionsDir,
	}, nil
}

// SetUp method boots up the Docker Compose with Terraform executor and mounted .tf definitions.
func (tsd TerraformServiceDeployer) SetUp(ctx context.Context, svcInfo ServiceInfo) (DeployedService, error) {
	logger.Debug("setting up service using Terraform deployer")

	configDir, err := tsd.installDockerfile(deployerFolderName(svcInfo))
	if err != nil {
		return nil, fmt.Errorf("can't install Docker Compose definitions: %w", err)
	}

	ymlPaths := []string{filepath.Join(configDir, terraformDeployerYml)}
	envYmlPath := filepath.Join(tsd.definitionsDir, envYmlFile)
	_, err = os.Stat(envYmlPath)
	if err == nil {
		ymlPaths = append(ymlPaths, envYmlPath)
	}

	tfEnvironment := tsd.buildTerraformExecutorEnvironment(svcInfo)

	service := dockerComposeDeployedService{
		ymlPaths:        ymlPaths,
		project:         svcInfo.ProjectName(),
		env:             tfEnvironment,
		shutdownTimeout: 300 * time.Second,
		configDir:       configDir,
	}

	p, err := compose.NewProject(service.project, service.ymlPaths...)
	if err != nil {
		return nil, fmt.Errorf("could not create Docker Compose project for service: %w", err)
	}

	// Clean service logs
	err = files.RemoveContent(svcInfo.Logs.Folder.Local)
	if err != nil {
		return nil, fmt.Errorf("removing service logs failed: %w", err)
	}

	opts := compose.CommandOptions{
		Env: service.env,
	}
	// Set custom aliases, which may be used in agent policies.
	serviceComposeConfig, err := p.Config(ctx, opts)
	if err != nil {
		return nil, fmt.Errorf("could not get Docker Compose configuration for service: %w", err)
	}
	svcInfo.CustomProperties, err = buildTerraformAliases(serviceComposeConfig)
	if err != nil {
		return nil, fmt.Errorf("can't build Terraform aliases: %w", err)
	}

	// Boot up service
	opts = compose.CommandOptions{
		Env:       service.env,
		ExtraArgs: []string{"--build", "-d"},
	}
	err = p.Up(ctx, opts)
	if err != nil {
		return nil, fmt.Errorf("could not boot up service using Docker Compose: %w", err)
	}

	err = p.WaitForHealthy(ctx, opts)
	if err != nil {
		processServiceContainerLogs(ctx, p, compose.CommandOptions{
			Env: opts.Env,
		}, svcInfo.Name)
		//lint:ignore ST1005 error starting with product name can be capitalized
		return nil, fmt.Errorf("Terraform deployer is unhealthy: %w", err)
	}

	svcInfo.Agent.Host.NamePrefix = "docker-fleet-agent"

	err = addTerraformOutputs(&svcInfo)
	if err != nil {
		return nil, fmt.Errorf("could not handle terraform output: %w", err)
	}
	service.svcInfo = svcInfo
	return &service, nil
}

func (tsd TerraformServiceDeployer) installDockerfile(folder string) (string, error) {
	locationManager, err := locations.NewLocationManager()
	if err != nil {
		return "", fmt.Errorf("failed to find the configuration directory: %w", err)
	}

	tfDir := filepath.Join(locationManager.DeployerDir(), terraformDeployerDir, folder)

	resources := []resource.Resource{
		&resource.File{
			Path:         terraformDeployerYml,
			Content:      resource.FileContentLiteral(terraformDeployerYmlContent),
			CreateParent: true,
		},
		&resource.File{
			Path:         terraformDeployerRun,
			Content:      resource.FileContentLiteral(terraformDeployerRunContent),
			CreateParent: true,
		},
		&resource.File{
			Path:         terraformDeployerDockerfile,
			Content:      resource.FileContentLiteral(terraformDeployerDockerfileContent),
			CreateParent: true,
		},
	}

	resourceManager := resource.NewManager()
	resourceManager.RegisterProvider("file", &resource.FileProvider{
		Prefix: tfDir,
	})

	results, err := resourceManager.Apply(resources)
	if err != nil {
		return "", fmt.Errorf("%w: %s", err, common.ProcessResourceApplyResults(results))
	}

	return tfDir, nil
}

func CreateOutputDir(locationManager *locations.LocationManager, runID string) (string, error) {
	outputDir := filepath.Join(locationManager.ServiceOutputDir(), runID)
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create output directory: %w", err)
	}
	return outputDir, nil
}

var _ ServiceDeployer = new(TerraformServiceDeployer)

func deployerFolderName(svcInfo ServiceInfo) string {
	return fmt.Sprintf("%s-%s", svcInfo.Name, svcInfo.Test.RunID)
}
