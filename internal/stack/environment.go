// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package stack

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/elastic/elastic-package/internal/elasticsearch"
	"github.com/elastic/elastic-package/internal/fleetserver"
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
	config.Parameters[ParamServerlessLocalStackVersion] = options.StackVersion

	config, err = p.setupFleet(ctx, config, options.StackVersion)
	if err != nil {
		return fmt.Errorf("failed to setup Fleet: %w", err)
	}

	// We need to store the config here to be able to clean up Fleet if something
	// fails later.
	err = storeConfig(options.Profile, config)
	if err != nil {
		return fmt.Errorf("failed to store config: %w", err)
	}

	outputID := ""
	logstashEnabled := options.Profile.Config(configLogstashEnabled, "false") == "true"
	if logstashEnabled {
		err := addLogstashFleetOutput(ctx, p.kibana)
		if err != nil {
			return fmt.Errorf("failed to create logstash output: %w", err)
		}
		outputID = fleetLogstashOutput
		config.Parameters[paramLogstashOutputID] = fleetLogstashOutput
	}

	// We need to store the config here to be able to clean up the logstash output if something
	// fails later.
	err = storeConfig(options.Profile, config)
	if err != nil {
		return fmt.Errorf("failed to store config: %w", err)
	}

	selfMonitor := options.Profile.Config(configSelfMonitorEnabled, "false") == "true"
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

	if logstashEnabled {
		err = updateLogstashFleetOutput(ctx, options.Profile, p.kibana)
		if err != nil {
			return fmt.Errorf("cannot configure fleet output: %w", err)
		}
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

func (p environmentProvider) setupFleet(ctx context.Context, config Config, stackVersion string) (Config, error) {
	const localFleetServerURL = "https://fleet-server:8220"

	fleetServerURL, err := p.kibana.DefaultFleetServerURL(ctx)
	if errors.Is(err, kibana.ErrFleetServerNotFound) || !isFleetServerReachable(ctx, fleetServerURL) {
		// We need to setup a local Fleet Server
		fleetServerURL = localFleetServerURL
		config.Parameters[paramFleetServerManaged] = "true"

		host := kibana.FleetServerHost{
			URLs:      []string{fleetServerURL},
			IsDefault: true,
			Name:      "elastic-package-managed-fleet-server",
		}
		// TODO: Check if it is already there to avoid creating many of them.
		err := p.kibana.AddFleetServerHost(ctx, host)
		if err != nil && !errors.Is(err, kibana.ErrConflict) {
			return config, fmt.Errorf("failed to add Fleet Server host: %w", err)
		}

		_, err = createFleetServerPolicy(ctx, p.kibana, stackVersion)
		if err != nil {
			return config, fmt.Errorf("failed to create agent policy for Fleet Server: %w", err)
		}

		config.FleetServiceToken, err = p.kibana.CreateFleetServiceToken(ctx)
		if err != nil {
			return config, fmt.Errorf("failed to create service token for Fleet Server: %w", err)
		}
	} else if err != nil {
		return config, fmt.Errorf("failed to discover Fleet Server URL: %w", err)
	}

	config.Parameters[ParamServerlessFleetURL] = fleetServerURL
	return config, nil
}

func isFleetServerReachable(ctx context.Context, address string) bool {
	status, err := fleetserver.NewClient(address).Status(ctx)
	return err == nil && strings.ToLower(status.Status) == "healthy"
}

// TearDown stops and/or removes a stack.
func (p *environmentProvider) TearDown(ctx context.Context, options Options) error {
	localServices := &localServicesManager{
		profile: options.Profile,
	}
	err := localServices.destroy(ctx)
	if err != nil {
		return fmt.Errorf("failed to destroy local services: %w", err)
	}

	kibanaClient, err := NewKibanaClientFromProfile(options.Profile)
	if err != nil {
		return fmt.Errorf("failed to create kibana client: %w", err)
	}
	err = forceUnenrollAgentsWithPolicy(ctx, kibanaClient)
	if err != nil {
		return fmt.Errorf("failed to remove agents associated to test policy: %w", err)
	}
	err = deleteAgentPolicy(ctx, kibanaClient)
	if err != nil {
		return fmt.Errorf("failed to delete agent policy: %v", err)
	}

	config, err := LoadConfig(options.Profile)
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}
	if managed, found := config.Parameters[paramFleetServerManaged]; found && managed == "true" {
		err = forceUnenrollFleetServerWithPolicy(ctx, kibanaClient)
		if err != nil {
			return fmt.Errorf("failed to remove managed fleet servers: %w", err)
		}

		err = deleteFleetServerPolicy(ctx, kibanaClient)
		if err != nil {
			return fmt.Errorf("failed to delete fleet server policy: %w", err)
		}
	}

	logstashOutput, logstashEnabled := config.Parameters[paramLogstashOutputID]
	if logstashEnabled && logstashOutput != "" {
		err := kibanaClient.RemoveFleetOutput(ctx, logstashOutput)
		if err != nil {
			return fmt.Errorf("failed to delete %s output: %s", fleetLogstashOutput, err)
		}
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
func (p *environmentProvider) Status(ctx context.Context, options Options) ([]ServiceStatus, error) {
	status := []ServiceStatus{
		p.elasticsearchStatus(ctx, options),
		p.kibanaStatus(ctx, options),
	}

	config, err := LoadConfig(options.Profile)
	if err != nil {
		return nil, fmt.Errorf("failed to load configuration: %w", err)
	}
	// If fleet is managed, it will be included in the local services status.
	if managed, ok := config.Parameters[paramFleetServerManaged]; !ok && managed != "true" {
		status = append(status, p.fleetStatus(ctx, options, config))
	}

	localServices := &localServicesManager{
		profile: options.Profile,
	}
	localStatus, err := localServices.status()
	if err != nil {
		return nil, fmt.Errorf("cannot obtain status of local services: %w", err)
	}

	status = append(status, localStatus...)
	return status, nil
}

func (p *environmentProvider) elasticsearchStatus(ctx context.Context, options Options) ServiceStatus {
	status := ServiceStatus{
		Name: "elasticsearch",
	}
	client, err := NewElasticsearchClientFromProfile(options.Profile)
	if err != nil {
		status.Status = "unknown: failed to create client: " + err.Error()
		return status
	}

	err = client.CheckHealth(ctx)
	if err != nil {
		status.Status = "unhealthy: " + err.Error()
	} else {
		status.Status = "healthy"
	}

	info, err := client.Info(ctx)
	if err != nil {
		status.Version = "unknown"
	} else if info.Version.BuildFlavor == "serverless" {
		status.Version = "serverless"
	} else {
		status.Version = info.Version.Number
	}

	return status
}

func (p *environmentProvider) kibanaStatus(ctx context.Context, options Options) ServiceStatus {
	status := ServiceStatus{
		Name: "kibana",
	}
	client, err := NewKibanaClientFromProfile(options.Profile)
	if err != nil {
		status.Status = "unknown: failed to create client: " + err.Error()
		return status
	}

	err = client.CheckHealth(ctx)
	if err != nil {
		status.Status = "unhealthy: " + err.Error()
	} else {
		status.Status = "healthy"
	}

	versionInfo, err := client.Version()
	if err != nil {
		status.Version = "unknown"
	} else if versionInfo.BuildFlavor == "serverless" {
		status.Version = "serverless"
	} else {
		status.Version = versionInfo.Version()
	}

	return status
}

func (p *environmentProvider) fleetStatus(ctx context.Context, options Options, config Config) ServiceStatus {
	status := ServiceStatus{
		Name: "fleet",
	}

	address, ok := config.Parameters[ParamServerlessFleetURL]
	if !ok || address == "" {
		status.Status = "unknown address"
		return status
	}

	// TODO: Add authentication for fleet server client.
	client := fleetserver.NewClient(address)
	fleetServerStatus, err := client.Status(ctx)
	if err != nil {
		status.Status = "unknown: " + err.Error()
	} else if fleetServerStatus.Status != "" {
		status.Status = strings.ToLower(fleetServerStatus.Status)
	} else {
		status.Status = "unknown"
	}

	if fleetServerStatus != nil {
		if version := fleetServerStatus.Version.Number; version != "" {
			status.Version = version
		} else {
			status.Version = "unknown"
		}
	}

	return status
}