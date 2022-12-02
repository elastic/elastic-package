// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package stack

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"

	"github.com/elastic/cloud-sdk-go/pkg/api"
	"github.com/elastic/cloud-sdk-go/pkg/api/deploymentapi"
	"github.com/elastic/cloud-sdk-go/pkg/api/deploymentapi/deptemplateapi"
	"github.com/elastic/cloud-sdk-go/pkg/auth"
	"github.com/elastic/cloud-sdk-go/pkg/models"
	"github.com/elastic/cloud-sdk-go/pkg/plan"
	"github.com/elastic/cloud-sdk-go/pkg/plan/planutil"

	"github.com/elastic/elastic-package/internal/logger"
	"github.com/elastic/elastic-package/internal/profile"
)

const (
	paramCloudDeploymentID    = "cloud_deployment_id"
	paramCloudDeploymentAlias = "cloud_deployment_alias"

	// Docs: https://www.elastic.co/guide/en/cloud/current/ec-api-deployment-crud.html
	cloudAPI = "https://api.elastic-cloud.com/api/v1"
)

var (
	deploymentNotExistErr     = errors.New("deployment does not exist")
	deploymentAlreadyExistErr = errors.New("deployment already exists")
)

var cloudDeploymentsAPI string

func init() {
	mustJoinURL := func(base string, elem ...string) string {
		joined, err := url.JoinPath(base, elem...)
		if err != nil {
			panic(err)
		}
		return joined
	}
	cloudDeploymentsAPI = mustJoinURL(cloudAPI, "deployments")
}

type cloudProvider struct {
	api     *api.API
	profile *profile.Profile
}

func newCloudProvider(profile *profile.Profile) (*cloudProvider, error) {
	apiKey := os.Getenv("EC_API_KEY")
	if apiKey == "" {
		return nil, errors.New("unable to obtain value from EC_API_KEY environment variable")
	}
	api, err := api.NewAPI(api.Config{
		Client:     new(http.Client),
		AuthWriter: auth.APIKey(apiKey),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to initialize API client: %w", err)
	}

	return &cloudProvider{
		api:     api,
		profile: profile,
	}, nil
}

func (cp *cloudProvider) BootUp(options Options) error {
	_, err := cp.currentDeployment()
	if err == nil {
		// Do nothing, deployment already exists.
		// TODO: Migrate configuration if changed.
		config, err := loadConfig(cp.profile)
		if err != nil {
			return err
		}
		printUserConfig(options.Printer, config)
		return nil
	} else if err != nil && err != deploymentNotExistErr {
		return err
	}

	// TODO: Parameterize this.
	name := "elastic-package-test"
	region := "gcp-europe-west3"
	templateID := "gcp-io-optimized"

	logger.Debugf("Getting deployment template %q", templateID)
	template, err := deptemplateapi.Get(deptemplateapi.GetParams{
		API:          cp.api,
		TemplateID:   templateID,
		Region:       region,
		StackVersion: options.StackVersion,
	})
	if err != nil {
		return fmt.Errorf("failed to get deployment template %q: %w", templateID, err)
	}

	payload := template.DeploymentTemplate

	// Remove the resources that we don't need.
	payload.Resources.Apm = nil
	payload.Resources.Appsearch = nil
	payload.Resources.EnterpriseSearch = nil

	// Initialize the plan with the id of the template, otherwise the create request fails.
	if es := payload.Resources.Elasticsearch; len(es) > 0 {
		plan := es[0].Plan
		if plan.DeploymentTemplate == nil {
			plan.DeploymentTemplate = &models.DeploymentTemplateReference{}
		}
		plan.DeploymentTemplate.ID = &templateID
	}

	logger.Debugf("Creating deployment %q", name)
	res, err := deploymentapi.Create(deploymentapi.CreateParams{
		API:     cp.api,
		Request: payload,
		Overrides: &deploymentapi.PayloadOverrides{
			Name: name,
		},
	})
	if err != nil {
		return fmt.Errorf("failed to create deployment: %w", err)
	}
	if created := res.Created; created == nil || !*created {
		return fmt.Errorf("request succeeded, but deployment was not created, check in the console UI")
	}

	var config Config
	config.Parameters = map[string]string{
		"cloud_deployment_alias": res.Alias,
	}
	deploymentID := res.ID
	if deploymentID == nil {
		return fmt.Errorf("deployment created, but couldn't get its ID, check in the console UI")
	}
	config.Parameters["cloud_deployment_id"] = *deploymentID

	for _, resource := range res.Resources {
		kind := resource.Kind
		if kind == nil {
			continue
		}
		if *kind == "elasticsearch" {
			if creds := resource.Credentials; creds != nil {
				if creds.Username != nil {
					config.ElasticsearchUsername = *creds.Username
				}
				if creds.Password != nil {
					config.ElasticsearchPassword = *creds.Password
				}
			}
		}
	}

	deployment, err := deploymentapi.Get(deploymentapi.GetParams{
		API:          cp.api,
		DeploymentID: *deploymentID,
	})
	if err != nil {
		return fmt.Errorf("couldn't check deployment health: %w", err)
	}

	config.ElasticsearchHost, err = cp.getServiceURL(deployment.Resources.Elasticsearch)
	if err != nil {
		return fmt.Errorf("failed to get elasticsearch host: %w", err)
	}
	config.KibanaHost, err = cp.getServiceURL(deployment.Resources.Kibana)
	if err != nil {
		return fmt.Errorf("failed to get kibana host: %w", err)
	}
	config.Parameters["fleet_url"], err = cp.getServiceURL(deployment.Resources.IntegrationsServer)
	if err != nil {
		return fmt.Errorf("failed to get fleet host: %w", err)
	}

	printUserConfig(options.Printer, config)

	err = storeConfig(cp.profile, config)
	if err != nil {
		return fmt.Errorf("failed to store config: %w", err)
	}

	logger.Debug("Waiting for creation plan to be completed")
	err = planutil.TrackChange(planutil.TrackChangeParams{
		TrackChangeParams: plan.TrackChangeParams{
			API:          cp.api,
			DeploymentID: *deploymentID,
		},
		Writer: &cloudTrackWriter{},
		Format: "text",
	})
	if err != nil {
		return fmt.Errorf("failed to track cluster creation", err)
	}

	return nil
}

func (cp *cloudProvider) TearDown(options Options) error {
	deployment, err := cp.currentDeployment()
	if err != nil {
		return fmt.Errorf("failed to find current deployment: %w", err)
	}
	if deployment.ID == nil {
		return fmt.Errorf("deployment doesn't have id?")
	}

	logger.Debugf("Deleting deployment %q", *deployment.ID)

	_, err = deploymentapi.Shutdown(deploymentapi.ShutdownParams{
		API:          cp.api,
		DeploymentID: *deployment.ID,
		SkipSnapshot: true,
	})
	if err != nil {
		return fmt.Errorf("failed to shutdown deployment: %w", err)
	}

	return nil
}

func (*cloudProvider) Update(options Options) error {
	fmt.Println("Nothing to do.")
	return nil
}

func (*cloudProvider) Dump(options DumpOptions) (string, error) {
	return "", fmt.Errorf("not implemented")
}

func (cp *cloudProvider) Status(options Options) ([]ServiceStatus, error) {
	deployment, err := cp.currentDeployment()
	if err != nil {
		return nil, err
	}

	status, _ := cp.deploymentStatus(deployment)
	return status, nil
}

func (*cloudProvider) deploymentStatus(deployment *models.DeploymentGetResponse) ([]ServiceStatus, bool) {
	allHealthy := true
	healthStatus := func(healthy *bool) string {
		if healthy != nil && *healthy {
			return "healthy"
		}
		allHealthy = false
		return "unhealthy"
	}
	if healthy := deployment.Healthy; healthy == nil || !*healthy {
		allHealthy = false
	}

	var status []ServiceStatus
	for _, resource := range deployment.Resources.Elasticsearch {
		for i, instance := range resource.Info.Topology.Instances {
			var name string
			if instance.InstanceName == nil {
				name = fmt.Sprintf("elasticsearch-%d", i)
			} else {
				name = fmt.Sprintf("elasticsearch-%s", *instance.InstanceName)
			}
			status = append(status, ServiceStatus{
				Name:    name,
				Version: instance.ServiceVersion,
				Status:  healthStatus(instance.Healthy),
			})
		}
	}
	for _, resource := range deployment.Resources.Kibana {
		for i, instance := range resource.Info.Topology.Instances {
			var name string
			if instance.InstanceName == nil {
				name = fmt.Sprintf("kibana-%d", i)
			} else {
				name = fmt.Sprintf("kibana-%s", *instance.InstanceName)
			}
			status = append(status, ServiceStatus{
				Name:    name,
				Version: instance.ServiceVersion,
				Status:  healthStatus(instance.Healthy),
			})
		}
	}
	for _, resource := range deployment.Resources.IntegrationsServer {
		for i, instance := range resource.Info.Topology.Instances {
			var name string
			if instance.InstanceName == nil {
				name = fmt.Sprintf("integrations-server-%d", i)
			} else {
				name = fmt.Sprintf("integrations-server-%s", *instance.InstanceName)
			}
			status = append(status, ServiceStatus{
				Name:    name,
				Version: instance.ServiceVersion,
				Status:  healthStatus(instance.Healthy),
			})
		}
	}
	return status, allHealthy
}

func (cp *cloudProvider) currentDeployment() (*models.DeploymentGetResponse, error) {
	config, err := loadConfig(cp.profile)
	if err != nil {
		return nil, err
	}
	deploymentID, found := config.Parameters[paramCloudDeploymentID]
	if !found {
		return nil, deploymentNotExistErr
	}
	deployment, err := deploymentapi.Get(deploymentapi.GetParams{
		API:          cp.api,
		DeploymentID: deploymentID,
	})
	if err != nil {
		return nil, fmt.Errorf("couldn't check deployment health: %w", err)
	}

	// It seems that terminated deployments still exist, but hidden.
	if hidden := deployment.Metadata.Hidden; hidden != nil && *hidden {
		return nil, deploymentNotExistErr
	}

	return deployment, nil
}

func (*cloudProvider) getServiceURL(resourcesResponse any) (string, error) {
	// Converting back and forth for easier access.
	var resources []struct {
		Info struct {
			Metadata struct {
				ServiceURL string `json:"service_url"`
			} `json:"metadata"`
		} `json:"info"`
	}

	d, err := json.Marshal(resourcesResponse)
	if err != nil {
		return "", fmt.Errorf("failed to marshal resources: %w", err)
	}
	err = json.Unmarshal(d, &resources)
	if err != nil {
		return "", fmt.Errorf("failed to unmarshal resources back: %w", err)
	}

	for _, resource := range resources {
		if serviceURL := resource.Info.Metadata.ServiceURL; serviceURL != "" {
			return serviceURL, nil
		}
	}
	return "", fmt.Errorf("url not found")
}

type cloudTrackWriter struct{}

func (*cloudTrackWriter) Write(p []byte) (n int, err error) {
	logger.Debug(string(p))
	return len(p), nil
}
