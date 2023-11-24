// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package serverless

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/elastic/elastic-package/internal/elasticsearch"
	"github.com/elastic/elastic-package/internal/kibana"
	"github.com/elastic/elastic-package/internal/logger"
	"github.com/elastic/elastic-package/internal/registry"
)

// Project represents a serverless project
type Project struct {
	url    string
	apiKey string

	Name   string `json:"name"`
	ID     string `json:"id"`
	Alias  string `json:"alias"`
	Type   string `json:"type"`
	Region string `json:"region_id"`

	Credentials struct {
		Username string `json:"username"`
		Password string `json:"password"`
	} `json:"credentials"`

	Endpoints struct {
		Elasticsearch string `json:"elasticsearch"`
		Kibana        string `json:"kibana"`
		Fleet         string `json:"fleet,omitempty"`
		APM           string `json:"apm,omitempty"`
	} `json:"endpoints"`
}

func (p *Project) EnsureHealthy(ctx context.Context, elasticsearchClient *elasticsearch.Client, kibanaClient *kibana.Client) error {
	if err := p.ensureElasticsearchHealthy(ctx, elasticsearchClient); err != nil {
		return fmt.Errorf("elasticsearch not healthy: %w", err)
	}
	if err := p.ensureKibanaHealthy(ctx, kibanaClient); err != nil {
		return fmt.Errorf("kibana not healthy: %w", err)
	}
	if err := p.ensureFleetHealthy(ctx); err != nil {
		return fmt.Errorf("fleet not healthy: %w", err)
	}
	return nil
}

func (p *Project) Status(ctx context.Context, elasticsearchClient *elasticsearch.Client, kibanaClient *kibana.Client) (map[string]string, error) {
	var status map[string]string
	healthStatus := func(err error) string {
		if err != nil {
			return fmt.Sprintf("unhealthy: %s", err.Error())
		}
		return "healthy"
	}

	status = map[string]string{
		"elasticsearch": healthStatus(p.getESHealth(ctx, elasticsearchClient)),
		"kibana":        healthStatus(p.getKibanaHealth(kibanaClient)),
		"fleet":         healthStatus(p.getFleetHealth(ctx)),
	}
	return status, nil
}

func (p *Project) ensureElasticsearchHealthy(ctx context.Context, elasticsearchClient *elasticsearch.Client) error {
	for {
		err := elasticsearchClient.CheckHealth(ctx)
		if err == nil {
			return nil
		}

		logger.Debugf("Elasticsearch service not ready: %s", err.Error())
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(5 * time.Second):
		}
	}
}

func (p *Project) ensureKibanaHealthy(ctx context.Context, kibanaClient *kibana.Client) error {
	for {
		err := kibanaClient.CheckHealth()
		if err == nil {
			return nil
		}

		logger.Debugf("Kibana service not ready: %s", err.Error())
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(5 * time.Second):
		}
	}
}

func (p *Project) ensureFleetHealthy(ctx context.Context) error {
	for {
		err := p.getFleetHealth(ctx)
		if err == nil {
			return nil
		}

		logger.Debugf("Fleet service not ready: %s", err.Error())
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(5 * time.Second):
		}
	}
}

func (p *Project) DefaultFleetServerURL(kibanaClient *kibana.Client) (string, error) {
	fleetURL, err := kibanaClient.DefaultFleetServerURL()
	if err != nil {
		return "", fmt.Errorf("failed to query fleet server hosts: %w", err)
	}

	return fleetURL, nil
}

func (p *Project) getESHealth(ctx context.Context, elasticsearchClient *elasticsearch.Client) error {
	return elasticsearchClient.CheckHealth(ctx)
}

func (p *Project) getKibanaHealth(kibanaClient *kibana.Client) error {
	return kibanaClient.CheckHealth()
}

func (p *Project) getFleetHealth(ctx context.Context) error {
	statusURL, err := url.JoinPath(p.Endpoints.Fleet, "/api/status")
	if err != nil {
		return fmt.Errorf("could not build URL: %w", err)
	}
	logger.Debugf("GET %s", statusURL)
	req, err := http.NewRequestWithContext(ctx, "GET", statusURL, nil)
	if err != nil {
		return err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("request failed (url: %s): %w", statusURL, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("unexpected status code %v", resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body: %w", err)
	}
	var status struct {
		Name   string `json:"name"`
		Status string `json:"status"`
	}
	err = json.Unmarshal(body, &status)
	if err != nil {
		return fmt.Errorf("failed to parse response body: %w", err)
	}

	if status.Status != "HEALTHY" {
		return fmt.Errorf("fleet status %s", status.Status)

	}
	return nil
}

func (p *Project) CreateAgentPolicy(stackVersion string, kibanaClient *kibana.Client) error {
	systemPackages, err := registry.Production.Revisions("system", registry.SearchOptions{
		KibanaVersion: strings.TrimSuffix(stackVersion, kibana.SNAPSHOT_SUFFIX),
	})
	if err != nil {
		return fmt.Errorf("could not get the system package version for Kibana %v: %w", stackVersion, err)
	}
	if len(systemPackages) != 1 {
		return fmt.Errorf("unexpected number of system package versions for Kibana %s - found %d expected 1", stackVersion, len(systemPackages))
	}
	logger.Debugf("Found %s package - version %s", systemPackages[0].Name, systemPackages[0].Version)

	policy := kibana.Policy{
		ID:                "elastic-agent-managed-ep",
		Name:              "Elastic-Agent (elastic-package)",
		Description:       "Policy created by elastic-package",
		Namespace:         "default",
		MonitoringEnabled: []string{"logs", "metrics"},
	}
	newPolicy, err := kibanaClient.CreatePolicy(policy)
	if err != nil {
		return fmt.Errorf("error while creating agent policy: %w", err)
	}

	packagePolicy := kibana.PackagePolicy{
		Name:      "system-1",
		PolicyID:  newPolicy.ID,
		Namespace: newPolicy.Namespace,
	}
	packagePolicy.Package.Name = "system"
	packagePolicy.Package.Version = systemPackages[0].Version

	_, err = kibanaClient.CreatePackagePolicy(packagePolicy)
	if err != nil {
		return fmt.Errorf("error while creating package policy: %w", err)
	}

	return nil
}
