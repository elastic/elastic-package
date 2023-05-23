// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package stack

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"strings"
	"time"

	"github.com/elastic/go-elasticsearch/v7"
	"github.com/sethvargo/go-retry"

	"github.com/elastic/cloud-sdk-go/pkg/api"
	"github.com/elastic/cloud-sdk-go/pkg/api/deploymentapi"
	"github.com/elastic/cloud-sdk-go/pkg/api/deploymentapi/deptemplateapi"
	"github.com/elastic/cloud-sdk-go/pkg/api/deploymentapi/extensionapi"
	"github.com/elastic/cloud-sdk-go/pkg/auth"
	"github.com/elastic/cloud-sdk-go/pkg/models"
	"github.com/elastic/cloud-sdk-go/pkg/plan"
	"github.com/elastic/cloud-sdk-go/pkg/plan/planutil"

	"github.com/elastic/elastic-package/internal/compose"
	"github.com/elastic/elastic-package/internal/docker"
	"github.com/elastic/elastic-package/internal/logger"
	"github.com/elastic/elastic-package/internal/profile"
)

const (
	paramCloudDeploymentID    = "cloud_deployment_id"
	paramCloudDeploymentAlias = "cloud_deployment_alias"
	paramGeoIPExtensionID     = "geoip_extension_id"
)

var (
	errDeploymentNotExist = errors.New("deployment does not exist")
)

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
	logger.Warn("Elastic Cloud provider is in technical preview")

	_, err := cp.currentDeployment()
	switch err {
	case nil:
		// Do nothing, deployment already exists.
		// TODO: Migrate configuration if changed.
		config, err := LoadConfig(cp.profile)
		if err != nil {
			return err
		}
		printUserConfig(options.Printer, config)
		return nil
	case errDeploymentNotExist:
		// Deployment doesn't exist, let's continue.
		break
	default:
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
			Name:    name,
			Version: options.StackVersion,
		},
	})
	if err != nil {
		return fmt.Errorf("failed to create deployment: %w", err)
	}
	if created := res.Created; created == nil || !*created {
		return fmt.Errorf("request succeeded, but deployment was not created, check in the console UI")
	}

	var config Config
	config.Provider = ProviderCloud
	config.Parameters = map[string]string{
		paramCloudDeploymentAlias: res.Alias,
	}
	deploymentID := res.ID
	if deploymentID == nil {
		return fmt.Errorf("deployment created, but couldn't get its ID, check in the console UI")
	}
	config.Parameters[paramCloudDeploymentID] = *deploymentID

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
		return fmt.Errorf("failed to track cluster creation: %w", err)
	}

	// Storing configuration now, so if something fails with the extension, we still
	// keep track of the deployment id.
	err = storeConfig(cp.profile, config)
	if err != nil {
		return fmt.Errorf("failed to store config: %w", err)
	}

	logger.Debugf("Replacing GeoIP databases")

	client, err := elasticsearch.NewClient(elasticsearch.Config{
		Addresses: []string{config.ElasticsearchHost},
		Username:  config.ElasticsearchUsername,
		Password:  config.ElasticsearchPassword,
	})
	if err != nil {
		return fmt.Errorf("failed to initialize Elasticsearch client: %w", err)
	}

	settingsPayload := `{"persistent": {"ingest.geoip.downloader.enabled":false}}`
	resp, err := client.Cluster.PutSettings(strings.NewReader(settingsPayload))
	if err != nil {
		return fmt.Errorf("failed to disable geoip automatic downloader: %w", err)
	}
	if resp.IsError() {
		return fmt.Errorf("failed to disable geoip automatic downloader (status: %v): %v", resp.StatusCode, resp.String())
	}

	geoIPExtension, err := cp.createGeoIPExtension()
	if err != nil {
		return fmt.Errorf("failed to create GeoIP extension: %w", err)
	}

	config.Parameters[paramGeoIPExtensionID] = *geoIPExtension.ID
	err = storeConfig(cp.profile, config)
	if err != nil {
		return fmt.Errorf("failed to store config: %w", err)
	}

	// Add the GeoIP bundle.
	updatePlan := models.ElasticsearchClusterPlan{
		// If no cluster topology is included, cluster is terminated.
		ClusterTopology: payload.Resources.Elasticsearch[0].Plan.ClusterTopology,
		Elasticsearch: &models.ElasticsearchConfiguration{
			UserBundles: []*models.ElasticsearchUserBundle{
				&models.ElasticsearchUserBundle{
					ElasticsearchVersion: &options.StackVersion,
					Name:                 geoIPExtension.Name,
					URL:                  geoIPExtension.URL,
				},
			},
			Version: options.StackVersion,
		},
		DeploymentTemplate: &models.DeploymentTemplateReference{
			ID: &templateID,
		},
	}
	pruneOrphans := false
	_, err = deploymentapi.Update(deploymentapi.UpdateParams{
		API:          cp.api,
		DeploymentID: *deploymentID,
		Request: &models.DeploymentUpdateRequest{
			PruneOrphans: &pruneOrphans,
			Resources: &models.DeploymentUpdateResources{
				Elasticsearch: []*models.ElasticsearchPayload{
					&models.ElasticsearchPayload{
						RefID:  res.Resources[0].RefID,
						Region: res.Resources[0].Region,
						Plan:   &updatePlan,
					},
				},
			},
		},
	})
	if err != nil {
		return fmt.Errorf("failed to add extension to deployment: %w", err)
	}

	err = planutil.TrackChange(planutil.TrackChangeParams{
		TrackChangeParams: plan.TrackChangeParams{
			API:          cp.api,
			DeploymentID: *deploymentID,
		},
		Writer: &cloudTrackWriter{},
		Format: "text",
	})
	if err != nil {
		return fmt.Errorf("failed to track cluster creation: %w", err)
	}

	// FIXME: Create initial agent policy.

	logger.Debugf("Starting local agent")

	err = cp.startLocalAgent(options, config)
	if err != nil {
		return fmt.Errorf("failed to start local agent: %w", err)
	}

	return nil
}

func (cp *cloudProvider) createGeoIPExtension() (*models.Extension, error) {
	bundle, err := zipGeoIPBundle()
	if err != nil {
		return nil, fmt.Errorf("failed to create GeoIP bundle: %w", err)
	}

	// TODO: Parameterize extension Name.
	extensionName := "geoip-extension"
	extension, err := extensionapi.Create(extensionapi.CreateParams{
		API:         cp.api,
		Name:        extensionName,
		Description: "GeoIP extension for elastic-package tests",
		Type:        "bundle",
		Version:     "*",
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create extension: %w", err)
	}
	if extension.ID == nil {
		return nil, fmt.Errorf("missing identifier in extension")
	}

	extension, err = extensionapi.Upload(extensionapi.UploadParams{
		API:         cp.api,
		ExtensionID: *extension.ID,
		File:        bundle,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to upload bundle: %w", err)
	}

	return extension, nil
}

func zipGeoIPBundle() (*bytes.Buffer, error) {
	// From https://www.elastic.co/guide/en/cloud/current/ec-custom-bundles.html
	const baseDir = "ingest-geoip"

	files := []string{
		"GeoLite2-ASN.mmdb",
		"GeoLite2-City.mmdb",
		"GeoLite2-Country.mmdb",
	}

	var bundle bytes.Buffer
	w := zip.NewWriter(&bundle)
	for _, fileName := range files {
		fw, err := w.Create(path.Join(baseDir, fileName))
		if err != nil {
			return nil, fmt.Errorf("failed to create file %q in bundle: %w", fileName, err)
		}

		fr, err := static.Open(path.Join("_static", fileName))
		if err != nil {
			return nil, fmt.Errorf("failed to open static file %q: %w", fileName, err)
		}

		_, err = io.Copy(fw, fr)
		if err != nil {
			fr.Close()
			return nil, fmt.Errorf("failed to copy contents of file %q: %w", fileName, err)
		}
		fr.Close()
	}
	if err := w.Close(); err != nil {
		return nil, fmt.Errorf("failed to close bundle: %w", err)
	}

	return &bundle, nil
}

func (cp *cloudProvider) localAgentComposeProject() (*compose.Project, error) {
	composeFile := cp.profile.Path(profileStackPath, CloudComposeFile)
	return compose.NewProject(DockerComposeProjectName, composeFile)
}

func (cp *cloudProvider) startLocalAgent(options Options, config Config) error {
	err := applyCloudResources(cp.profile, options.StackVersion, config)
	if err != nil {
		return fmt.Errorf("could not initialize compose files for local agent: %w", err)
	}

	project, err := cp.localAgentComposeProject()
	if err != nil {
		return fmt.Errorf("could not initialize local agent compose project")
	}

	err = project.Build(compose.CommandOptions{})
	if err != nil {
		return fmt.Errorf("failed to build images for local agent: %w", err)
	}

	err = project.Up(compose.CommandOptions{})
	if err != nil {
		return fmt.Errorf("failed to start local agent: %w", err)
	}

	return nil
}

func (cp *cloudProvider) TearDown(options Options) error {
	err := cp.destroyLocalAgent()
	if err != nil {
		return fmt.Errorf("failed to destroy local agent: %w", err)
	}

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

	logger.Debugf("Deleting GeoIP bundle.")
	err = cp.deleteGeoIPExtension()
	if err != nil {
		return fmt.Errorf("failed to delete GeoIP extension: %w", err)
	}

	return nil
}

func (cp *cloudProvider) deleteGeoIPExtension() error {
	config, err := LoadConfig(cp.profile)
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}
	extensionID, found := config.Parameters[paramGeoIPExtensionID]
	if !found {
		return nil
	}

	backoff := retry.NewFibonacci(1 * time.Second)
	backoff = retry.WithMaxDuration(180*time.Second, backoff)
	retry.Do(context.TODO(), backoff, func(ctx context.Context) error {
		err = extensionapi.Delete(extensionapi.DeleteParams{
			API:         cp.api,
			ExtensionID: extensionID,
		})
		// Actually, we should only retry on extensions.extension_in_use errors.
		return retry.RetryableError(err)
	})
	if err != nil {
		return fmt.Errorf("delete API call failed: %w", err)
	}
	return nil
}

func (cp *cloudProvider) destroyLocalAgent() error {
	project, err := cp.localAgentComposeProject()
	if err != nil {
		return fmt.Errorf("could not initialize local agent compose project")
	}

	err = project.Down(compose.CommandOptions{})
	if err != nil {
		return fmt.Errorf("failed to destroy local agent: %w", err)
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

	agentStatus, err := cp.localAgentStatus()
	if err != nil {
		return nil, fmt.Errorf("failed to get local agent status: %w", err)
	}

	status = append(status, agentStatus...)

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

func (cp *cloudProvider) localAgentStatus() ([]ServiceStatus, error) {
	var services []ServiceStatus
	// query directly to docker to avoid load environment variables (e.g. STACK_VERSION_VARIANT) and profiles
	containerIDs, err := docker.ContainerIDsWithLabel(projectLabelDockerCompose, DockerComposeProjectName)
	if err != nil {
		return nil, err
	}

	if len(containerIDs) == 0 {
		return services, nil
	}

	containerDescriptions, err := docker.InspectContainers(containerIDs...)
	if err != nil {
		return nil, err
	}

	for _, containerDescription := range containerDescriptions {
		service, err := newServiceStatus(&containerDescription)
		if err != nil {
			return nil, err
		}
		logger.Debugf("Adding Service: \"%v\"", service.Name)
		services = append(services, *service)
	}

	return services, nil
}

func (cp *cloudProvider) currentDeployment() (*models.DeploymentGetResponse, error) {
	config, err := LoadConfig(cp.profile)
	if err != nil {
		return nil, err
	}
	deploymentID, found := config.Parameters[paramCloudDeploymentID]
	if !found {
		return nil, errDeploymentNotExist
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
		return nil, errDeploymentNotExist
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
