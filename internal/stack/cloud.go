// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package stack

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"

	"github.com/elastic/cloud-sdk-go/pkg/api"
	"github.com/elastic/cloud-sdk-go/pkg/api/deploymentapi"
	"github.com/elastic/cloud-sdk-go/pkg/api/deploymentapi/depresourceapi"
	"github.com/elastic/cloud-sdk-go/pkg/auth"

	"github.com/elastic/elastic-package/internal/profile"
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

	// TODO: Parameterize this.
	name := "elastic-package-test"
	region := "gcp-europe-west3"
	templateID := "gcp-storage-optimized"

	payload, err := depresourceapi.NewPayload(depresourceapi.NewPayloadParams{
		API:                  api,
		Name:                 name,
		Version:              options.StackVersion,
		Region:               region,
		DeploymentTemplateID: templateID,
	})
	if err != nil {
		return fmt.Errorf("failed to initialize payload: %w", err)
	}

	res, err := deploymentapi.Create(deploymentapi.CreateParams{
		API:     api,
		Request: payload,
	})
	if err != nil {
		return fmt.Errorf("failed to create deployment: %w", err)
	}

	fmt.Printf("%+v\n", res)
	return nil
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
