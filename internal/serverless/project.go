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
	"time"

	"github.com/elastic/elastic-package/internal/elasticsearch"
	"github.com/elastic/elastic-package/internal/kibana"
	"github.com/elastic/elastic-package/internal/logger"
)

const eprURL = "https://epr.elastic.co"

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

	ElasticsearchClient *elasticsearch.Client
	KibanaClient        *kibana.Client
}

func (p *Project) EnsureHealthy(ctx context.Context) error {
	if err := p.ensureElasticsearchHealthy(ctx); err != nil {
		return fmt.Errorf("elasticsearch not healthy: %w", err)
	}
	if err := p.ensureKibanaHealthy(ctx); err != nil {
		return fmt.Errorf("kibana not healthy: %w", err)
	}
	if err := p.ensureFleetHealthy(ctx); err != nil {
		return fmt.Errorf("fleet not healthy: %w", err)
	}
	return nil
}

func (p *Project) Status(ctx context.Context) (map[string]string, error) {
	var status map[string]string
	healthStatus := func(err error) string {
		if err != nil {
			return fmt.Sprintf("unhealthy: %s", err.Error())
		}
		return "healthy"
	}

	status = map[string]string{
		"elasticsearch": healthStatus(p.getESHealth(ctx)),
		"kibana":        healthStatus(p.getKibanaHealth()),
		"fleet":         healthStatus(p.getFleetHealth(ctx)),
	}
	return status, nil
}

func (p *Project) ensureElasticsearchHealthy(ctx context.Context) error {
	for {
		err := p.ElasticsearchClient.CheckHealth(ctx)
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

func (p *Project) ensureKibanaHealthy(ctx context.Context) error {
	for {
		err := p.KibanaClient.CheckHealth()
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

func (p *Project) DefaultFleetServerURL() (string, error) {
	fleetURL, err := p.KibanaClient.DefaultFleetServerURL()
	if err != nil {
		return "", fmt.Errorf("failed to query fleet server hosts: %w", err)
	}

	return fleetURL, nil
}

func (p *Project) getESHealth(ctx context.Context) error {
	return p.ElasticsearchClient.CheckHealth(ctx)
}

func (p *Project) getKibanaHealth() error {
	return p.KibanaClient.CheckHealth()
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

func (p *Project) CreateAgentPolicy(stackVersion string) error {
	systemVersion, err := getPackageVersion(eprURL, "system", stackVersion)
	if err != nil {
		return fmt.Errorf("could not get the system package version for kibana %v: %w", stackVersion, err)
	}

	policy := kibana.Policy{
		ID:                "elastic-agent-managed-ep",
		Name:              "Elastic-Agent (elastic-package)",
		Description:       "Policy created by elastic-package",
		Namespace:         "default",
		MonitoringEnabled: []string{"logs", "metrics"},
	}
	newPolicy, err := p.KibanaClient.CreatePolicy(policy)
	if err != nil {
		return fmt.Errorf("error while creating agent policy: %w", err)
	}

	packagePolicy := kibana.PackagePolicy{
		Name:      "system-1",
		PolicyID:  newPolicy.ID,
		Namespace: newPolicy.Namespace,
	}
	packagePolicy.Package.Name = "system"
	packagePolicy.Package.Version = systemVersion

	_, err = p.KibanaClient.CreatePackagePolicy(packagePolicy)
	if err != nil {
		return fmt.Errorf("error while creating package policy: %w", err)
	}

	return nil
}

func getPackageVersion(registryURL, packageName, stackVersion string) (string, error) {
	searchURL, err := url.JoinPath(registryURL, "search")
	if err != nil {
		return "", fmt.Errorf("could not build URL: %w", err)
	}
	// TODO: add capabilities or spec.minVersion?
	queryValues := url.Values{}
	queryValues.Add("package", packageName)
	queryValues.Add("kibana.version", stackVersion)

	searchURL = fmt.Sprintf("%s?%s", searchURL, queryValues.Encode())
	logger.Debugf("GET %s", searchURL)
	resp, err := http.Get(searchURL)
	if err != nil {
		return "", fmt.Errorf("request failed (url: %s): %w", searchURL, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return "", fmt.Errorf("unexpected status code %v", resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response body: %w", err)
	}
	var packages []struct {
		Name    string `json:"name"`
		Version string `json:"version"`
	}
	err = json.Unmarshal(body, &packages)
	if err != nil {
		return "", fmt.Errorf("failed to parse response body: %w", err)
	}
	if len(packages) != 1 {
		return "", fmt.Errorf("expected 1 package, obtained %v", len(packages))
	}
	if found := packages[0].Name; found != packageName {
		return "", fmt.Errorf("expected package %s, found %s", packageName, found)
	}

	return packages[0].Version, nil
}
