// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package serverless

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

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
}

type serviceHealthy func(context.Context, *Project) error

func (p *Project) EnsureHealthy(ctx context.Context) error {
	if err := p.ensureServiceHealthy(ctx, getESHealthy); err != nil {
		return fmt.Errorf("elasticsearch not healthy: %w", err)
	}
	if err := p.ensureServiceHealthy(ctx, getKibanaHealthy); err != nil {
		return fmt.Errorf("kibana not healthy: %w", err)
	}
	if err := p.ensureServiceHealthy(ctx, getFleetHealthy); err != nil {
		return fmt.Errorf("fleet not healthy: %w", err)
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

func getESHealthy(ctx context.Context, project *Project) error {
	client, err := NewClient(
		WithAddress(project.Endpoints.Elasticsearch),
		WithUsername(project.Credentials.Username),
		WithPassword(project.Credentials.Password),
	)
	if err != nil {
		return err
	}

	statusCode, respBody, err := client.get(ctx, "/_cluster/health")
	if err != nil {
		return fmt.Errorf("failed to query elasticsearch health: %w", err)
	}

	if statusCode != http.StatusOK {
		return fmt.Errorf("unexpected status code %d, body: %s", statusCode, string(respBody))
	}

	var health struct {
		Status string `json:"status"`
	}
	if err := json.Unmarshal(respBody, &health); err != nil {
		logger.Debugf("Unable to decode response: %v body: %s", err, string(respBody))
		return err
	}
	if health.Status == "green" {
		return nil
	}
	return fmt.Errorf("elasticsearch unhealthy: %s", health.Status)
}

func getKibanaHealthy(ctx context.Context, project *Project) error {
	client, err := NewClient(
		WithAddress(project.Endpoints.Kibana),
		WithUsername(project.Credentials.Username),
		WithPassword(project.Credentials.Password),
	)
	if err != nil {
		return err
	}

	statusCode, respBody, err := client.get(ctx, "/api/status")
	if err != nil {
		return fmt.Errorf("failed to query kibana status: %w", err)
	}
	if statusCode != http.StatusOK {
		return fmt.Errorf("unexpected status code %d, body: %s", statusCode, string(respBody))
	}

	var status struct {
		Status struct {
			Overall struct {
				Level string `json:"level"`
			} `json:"overall"`
		} `json:"status"`
	}
	if err := json.Unmarshal(respBody, &status); err != nil {
		logger.Debugf("Unable to decode response: %v body: %s", err, string(respBody))
		return err
	}
	if status.Status.Overall.Level == "available" {
		return nil
	}
	return fmt.Errorf("kibana unhealthy: %s", status.Status.Overall.Level)
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
