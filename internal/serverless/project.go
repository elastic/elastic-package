// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package serverless

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/elastic/elastic-package/internal/elasticsearch"
	"github.com/elastic/elastic-package/internal/kibana"
	"github.com/elastic/elastic-package/internal/logger"
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

	ElasticsearchClient *elasticsearch.Client
	KibanaClient        *kibana.Client
}

type serviceHealthy func(context.Context, *Project) error

func (p *Project) EnsureHealthy(ctx context.Context) error {
	if err := p.ensureElasticserchHealthy(ctx); err != nil {
		return fmt.Errorf("elasticsearch not healthy: %w", err)
	}
	if err := p.ensureKibanaHealthy(ctx); err != nil {
		return fmt.Errorf("kibana not healthy: %w", err)
	}
	if err := p.ensureServiceHealthy(ctx, getFleetHealthy); err != nil {
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
		"fleet":         healthStatus(getFleetHealthy(ctx, p)),
	}
	return status, nil
}

func (p *Project) ensureElasticserchHealthy(ctx context.Context) error {
	timer := time.NewTimer(time.Millisecond)
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-timer.C:
		}

		err := p.ElasticsearchClient.CheckHealth(ctx)
		if err != nil {
			logger.Debugf("service not ready: %s", err.Error())
			timer.Reset(time.Second * 5)
			continue
		}

		return nil
	}
	return nil
}

func (p *Project) ensureKibanaHealthy(ctx context.Context) error {
	timer := time.NewTimer(time.Millisecond)
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-timer.C:
		}

		err := p.KibanaClient.CheckHealth()
		if err != nil {
			logger.Debugf("service not ready: %s", err.Error())
			timer.Reset(time.Second * 5)
			continue
		}

		return nil
	}
	return nil
}

func (p *Project) ensureServiceHealthy(ctx context.Context, serviceFunc serviceHealthy) error {
	timer := time.NewTimer(time.Millisecond)
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-timer.C:
		}

		err := serviceFunc(ctx, p)
		if err != nil {
			logger.Debugf("service not ready: %s", err.Error())
			timer.Reset(time.Second * 5)
			continue
		}

		return nil
	}
	return nil
}

func (p *Project) DefaultFleetServerURL(ctx context.Context) (string, error) {
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

func getFleetHealthy(ctx context.Context, project *Project) error {
	client, err := NewClient(
		WithAddress(project.Endpoints.Fleet),
		WithUsername(project.Credentials.Username),
		WithPassword(project.Credentials.Password),
	)
	if err != nil {
		return err
	}

	statusCode, respBody, err := client.get(ctx, "/api/status")
	if err != nil {
		return fmt.Errorf("failed to query fleet status: %w", err)
	}
	if statusCode != http.StatusOK {
		return fmt.Errorf("fleet unhealthy: status code %d, body: %s", statusCode, string(respBody))
	}

	return nil
}
