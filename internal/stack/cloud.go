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
	"time"

	"github.com/elastic/cloud-sdk-go/pkg/api"
	"github.com/elastic/cloud-sdk-go/pkg/api/deploymentapi"
	"github.com/elastic/cloud-sdk-go/pkg/api/deploymentapi/deptemplateapi"
	"github.com/elastic/cloud-sdk-go/pkg/auth"
	"github.com/elastic/cloud-sdk-go/pkg/models"

	"github.com/elastic/elastic-package/internal/profile"
	"github.com/elastic/elastic-package/internal/signal"
)

// Docs: https://www.elastic.co/guide/en/cloud/current/ec-api-deployment-crud.html
const cloudAPI = "https://api.elastic-cloud.com/api/v1"

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
	profile *profile.Profile
}

func (cp *cloudProvider) getAPI() (*api.API, error) {
	apiKey := os.Getenv("EC_API_KEY")
	if apiKey == "" {
		return nil, errors.New("unable to obtain value from EC_API_KEY environment variable")
	}

	ec, err := api.NewAPI(api.Config{
		Client:     new(http.Client),
		AuthWriter: auth.APIKey(apiKey),
	})
	if err != nil {
		return nil, err
	}

	return ec, nil

}

func newCloudProvider(profile *profile.Profile) (*cloudProvider, error) {
	return &cloudProvider{
		profile: profile,
	}, nil
}

func (cp *cloudProvider) BootUp(options Options) error {
	api, err := cp.getAPI()
	if err != nil {
		return err
	}

	// TODO: Check if the deployment already exists.

	// TODO: Parameterize this.
	name := "elastic-package-test"
	region := "gcp-europe-west3"
	templateID := "gcp-io-optimized"

	template, err := deptemplateapi.Get(deptemplateapi.GetParams{
		API:          api,
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

	res, err := deploymentapi.Create(deploymentapi.CreateParams{
		API:     api,
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
		"alias": res.Alias,
	}
	deploymentID := res.ID
	if deploymentID == nil {
		return fmt.Errorf("deployment created, but couldn't get its ID, check in the console UI")
	}
	config.Parameters["id"] = *deploymentID
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

	// Storing once before getting the endpoints, so we have the ID.
	err = storeConfig(cp.profile, config)
	if err != nil {
		return fmt.Errorf("failed to store config: %w", err)
	}

	for {
		deployment, err := deploymentapi.Get(deploymentapi.GetParams{
			API:          api,
			DeploymentID: *deploymentID,
		})
		if err != nil {
			return fmt.Errorf("couldn't check deployment health: %w", err)
		}

		if healthy := deployment.Healthy; healthy == nil || !*healthy {
			if signal.SIGINT() {
				return fmt.Errorf("wait interrupted")
			}
			time.Sleep(1 * time.Second)
			continue
		}

		// TODO: Check that resources are healthy too.

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

		break
	}

	// Store the configuration again now with the service urls.
	err = storeConfig(cp.profile, config)
	if err != nil {
		return fmt.Errorf("failed to store config: %w", err)
	}

	return nil
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

func (*cloudProvider) TearDown(options Options) error {
	return fmt.Errorf("not implemented")
}

func (*cloudProvider) Update(options Options) error {
	fmt.Println("Nothing to do.")
	return nil
}

func (*cloudProvider) Dump(options DumpOptions) (string, error) {
	return "", fmt.Errorf("not implemented")
}

func (*cloudProvider) Status(options Options) ([]ServiceStatus, error) {
	return nil, fmt.Errorf("not implemented")
}
