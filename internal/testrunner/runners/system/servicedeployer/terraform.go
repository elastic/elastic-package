// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package servicedeployer

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/elastic/go-resource"

	"github.com/elastic/elastic-package/internal/compose"
	"github.com/elastic/elastic-package/internal/configuration/locations"
	"github.com/elastic/elastic-package/internal/files"
	"github.com/elastic/elastic-package/internal/logger"
)

const (
	terraformDeployerDir        = "terraform"
	terraformDeployerYml        = "terraform-deployer.yml"
	localstackDeployerYml       = "localstack-deployer.yml"
	terraformDeployerDockerfile = "Dockerfile"
	terraformDeployerRun        = "run.sh"
	terraformOutputPrefix       = "TF_OUTPUT_"
	terraformOutputJsonFile     = "tfOutputValues.json"
)

//go:embed _static/localstack_deployer.yml
var localstackDeployerYmlContent string

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

// addTerraformOutputs method reads the terraform outputs generated in the json format and
// adds them to the custom properties of ServiceContext and can be used in the handlebars template
// like `{{TF_OUTPUT_queue_url}}` where `queue_url` is the output configured
func addTerraformOutputs(outCtxt ServiceContext) error {
	// Read the `output.json` file where terraform outputs are generated
	outputFile := filepath.Join(outCtxt.OutputDir, terraformOutputJsonFile)
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

	if outCtxt.CustomProperties == nil {
		outCtxt.CustomProperties = make(map[string]any, len(terraformOutputs))
	}
	// Prefix variables names with TF_OUTPUT_
	for k, outputs := range terraformOutputs {
		outCtxt.CustomProperties[terraformOutputPrefix+k] = outputs.Value
	}
	return nil
}

// NewTerraformServiceDeployer creates an instance of TerraformServiceDeployer.
func NewTerraformServiceDeployer(definitionsDir string) (*TerraformServiceDeployer, error) {
	return &TerraformServiceDeployer{
		definitionsDir: definitionsDir,
	}, nil
}

// SetUp method boots up the Docker Compose with Terraform executor and mounted .tf definitions.
func (tsd TerraformServiceDeployer) SetUp(inCtxt ServiceContext) (DeployedService, error) {
	logger.Debug("setting up service using Terraform deployer")

	configDir, err := tsd.installDockerfile()
	if err != nil {
		return nil, fmt.Errorf("can't install Docker Compose definitions: %w", err)
	}

	ymlPaths := []string{filepath.Join(configDir, terraformDeployerYml)}

	localstackYmlPath := filepath.Join(configDir, localstackDeployerYml)
	_, err = os.Stat(localstackYmlPath)
	if err == nil {
		ymlPaths = append(ymlPaths, localstackYmlPath)
	}

	envYmlPath := filepath.Join(tsd.definitionsDir, envYmlFile)
	_, err = os.Stat(envYmlPath)
	if err == nil {
		ymlPaths = append(ymlPaths, envYmlPath)
	}

	logger.Debug("Print the yml Paths %s", ymlPaths)

	tfEnvironment := tsd.buildTerraformExecutorEnvironment(inCtxt)

	service := dockerComposeDeployedService{
		ymlPaths: ymlPaths,
		project:  "elastic-package-service",
		env:      tfEnvironment,
	}
	outCtxt := inCtxt

	p, err := compose.NewProject(service.project, service.ymlPaths...)
	if err != nil {
		return nil, fmt.Errorf("could not create Docker Compose project for service: %w", err)
	}

	// Clean service logs
	err = files.RemoveContent(outCtxt.Logs.Folder.Local)
	if err != nil {
		return nil, fmt.Errorf("removing service logs failed: %w", err)
	}

	opts := compose.CommandOptions{
		Env: service.env,
	}
	// Set custom aliases, which may be used in agent policies.
	serviceComposeConfig, err := p.Config(opts)
	if err != nil {
		return nil, fmt.Errorf("could not get Docker Compose configuration for service: %w", err)
	}
	outCtxt.CustomProperties, err = buildTerraformAliases(serviceComposeConfig)
	if err != nil {
		return nil, fmt.Errorf("can't build Terraform aliases: %w", err)
	}

	// Boot up service
	opts = compose.CommandOptions{
		Env:       service.env,
		ExtraArgs: []string{"--build", "-d"},
	}
	err = p.Up(opts)
	if err != nil {
		return nil, fmt.Errorf("could not boot up service using Docker Compose: %w", err)
	}

	err = p.WaitForHealthy(opts)
	if err != nil {
		processServiceContainerLogs(p, compose.CommandOptions{
			Env: opts.Env,
		}, outCtxt.Name)
		//lint:ignore ST1005 error starting with product name can be capitalized
		return nil, fmt.Errorf("Terraform deployer is unhealthy: %w", err)
	}

	outCtxt.Agent.Host.NamePrefix = "docker-fleet-agent"

	err = addTerraformOutputs(outCtxt)
	if err != nil {
		return nil, fmt.Errorf("could not handle terraform output: %w", err)
	}
	service.ctxt = outCtxt
	return &service, nil
}

func (tsd TerraformServiceDeployer) installDockerfile() (string, error) {
	locationManager, err := locations.NewLocationManager()
	if err != nil {
		return "", fmt.Errorf("failed to find the configuration directory: %w", err)
	}

	tfDir := filepath.Join(locationManager.DeployerDir(), terraformDeployerDir)

	resources := []resource.Resource{
		&resource.File{
			Path:         localstackDeployerYml,
			Content:      resource.FileContentLiteral(localstackDeployerYmlContent),
			CreateParent: true,
		},
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
		var errors []string
		for _, result := range results {
			if err := result.Err(); err != nil {
				errors = append(errors, err.Error())
			}
		}
		return "", fmt.Errorf("%w: %s", err, strings.Join(errors, ", "))
	}

	return tfDir, nil
}

var _ ServiceDeployer = new(TerraformServiceDeployer)
