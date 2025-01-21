// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package serverless

import (
	"context"
	"fmt"
	"time"

	"github.com/elastic/elastic-package/internal/elasticsearch"
	"github.com/elastic/elastic-package/internal/fleetserver"
	"github.com/elastic/elastic-package/internal/kibana"
	"github.com/elastic/elastic-package/internal/logger"
)

const (
	FleetLogstashOutput = "fleet-logstash-output"
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
		Username string `json:"username,omitempty"`
		Password string `json:"password,omitempty"`
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
		"kibana":        healthStatus(p.getKibanaHealth(ctx, kibanaClient)),
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
		err := kibanaClient.CheckHealth(ctx)
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

func (p *Project) DefaultFleetServerURL(ctx context.Context, kibanaClient *kibana.Client) (string, error) {
	fleetURL, err := kibanaClient.DefaultFleetServerURL(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to query fleet server hosts: %w", err)
	}

	return fleetURL, nil
}

func (p *Project) getESHealth(ctx context.Context, elasticsearchClient *elasticsearch.Client) error {
	return elasticsearchClient.CheckHealth(ctx)
}

func (p *Project) getKibanaHealth(ctx context.Context, kibanaClient *kibana.Client) error {
	return kibanaClient.CheckHealth(ctx)
}

func (p *Project) getFleetHealth(ctx context.Context) error {
	client, err := fleetserver.NewClient(p.Endpoints.Fleet)
	if err != nil {
		return fmt.Errorf("could not create Fleet Server client: %w", err)
	}
	status, err := client.Status(ctx)
	if err != nil {
		return err
	}

	if status.Status != "HEALTHY" {
		return fmt.Errorf("fleet status %s", status.Status)
	}

	return nil
}
