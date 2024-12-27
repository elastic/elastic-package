// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package stack

import (
	"context"
	"fmt"
	"os"

	"github.com/elastic/elastic-package/internal/elasticsearch"
	"github.com/elastic/elastic-package/internal/kibana"
	"github.com/elastic/elastic-package/internal/profile"
)

type environmentProvider struct {
	kibana        *kibana.Client
	elasticsearch *elasticsearch.Client
}

func newEnvironmentProvider(profile *profile.Profile) (*environmentProvider, error) {
	return &environmentProvider{}, nil
}

// BootUp configures the profile to use as stack the one indicated using environment variables.
func (p *environmentProvider) BootUp(ctx context.Context, options Options) error {
	config := Config{
		Provider:              ProviderEnvironment,
		ElasticsearchAPIKey:   os.Getenv(ElasticsearchAPIKeyEnv),
		ElasticsearchHost:     os.Getenv(ElasticsearchHostEnv),
		ElasticsearchUsername: os.Getenv(ElasticsearchUsernameEnv),
		ElasticsearchPassword: os.Getenv(ElasticsearchPasswordEnv),
		KibanaHost:            os.Getenv(KibanaHostEnv),
		CACertFile:            os.Getenv(CACertificateEnv),

		Parameters: make(map[string]string),
	}
	if err := requiredEnv(config.ElasticsearchHost, ElasticsearchHostEnv); err != nil {
		return err
	}
	if err := requiredEnv(config.KibanaHost, KibanaHostEnv); err != nil {
		return err
	}

	err := p.initClients()
	if err != nil {
		return err
	}
	// TODO: Migrate from serverless variables.
	config.Parameters[ParamServerlessFleetURL], err = p.kibana.DefaultFleetServerURL(ctx)
	if err != nil {
		return fmt.Errorf("cannot discover default fleet server URL: %w", err)
	}

	selfMonitor := options.Profile.Config(configSelfMonitorEnabled, "false") == "true"
	logstashEnabled := options.Profile.Config(configLogstashEnabled, "false") == "true"
	outputID := ""
	if logstashEnabled {
		outputID = "TODO"
	}

	// TODO: Handle policy already present.
	// TODO: Handle deletion of policy on tear down.
	policy, err := createAgentPolicy(ctx, p.kibana, options.StackVersion, outputID, selfMonitor)
	if err != nil {
		return fmt.Errorf("failed to create agent policy: %w", err)
	}
	if config.ElasticsearchAPIKey != "" {
		config.EnrollmentToken, err = p.kibana.GetEnrollmentTokenForPolicyID(ctx, policy.ID)
		if err != nil {
			return fmt.Errorf("failed to get an enrollment token for policy %s: %w", policy.Name, err)
		}
	}

	localServices := &localServicesManager{
		profile: options.Profile,
	}
	err = localServices.start(ctx, options, config)
	if err != nil {
		return fmt.Errorf("failed to start local services: %w", err)
	}

	err = storeConfig(options.Profile, config)
	if err != nil {
		return fmt.Errorf("failed to store config: %w", err)
	}

	return nil
}

func requiredEnv(value string, envVarName string) error {
	if value == "" {
		return fmt.Errorf("environment variable %s required", envVarName)
	}
	return nil
}

func (p *environmentProvider) initClients() error {
	kibana, err := NewKibanaClient()
	if err != nil {
		return fmt.Errorf("cannot create Kibana client: %w", err)
	}
	p.kibana = kibana

	elasticsearch, err := NewElasticsearchClient()
	if err != nil {
		return fmt.Errorf("cannot create Elasticsearch client: %w", err)
	}
	p.elasticsearch = elasticsearch

	return nil
}

// TearDown stops and/or removes a stack.
func (p *environmentProvider) TearDown(ctx context.Context, options Options) error {
	localServices := &localServicesManager{
		profile: options.Profile,
	}
	err := localServices.destroy(ctx)
	if err != nil {
		return fmt.Errorf("failed ot destroy local services: %v", err)
	}
	return nil
}

// Update updates resources associated to a stack.
func (p *environmentProvider) Update(context.Context, Options) error {
	return fmt.Errorf("not implemented")
}

// Dump dumps data for debug purpouses.
func (p *environmentProvider) Dump(ctx context.Context, options DumpOptions) ([]DumpResult, error) {
	for _, service := range options.Services {
		if service != "elastic-agent" {
			return nil, &ErrNotImplemented{
				Operation: fmt.Sprintf("logs dump for service %s", service),
				Provider:  ProviderServerless,
			}
		}
	}
	return Dump(ctx, options)
}

// Status obtains status information of the stack.
func (p *environmentProvider) Status(context.Context, Options) ([]ServiceStatus, error) {
	return nil, fmt.Errorf("not implemented")
}
