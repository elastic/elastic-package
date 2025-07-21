// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package serverless

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"time"

	"github.com/elastic/elastic-package/internal/logger"
)

const (
	defaultHostURL = "https://cloud.elastic.co"

	projectsAPI = "/api/v1/serverless/projects"
)

type Client struct {
	host   string
	apiKey string
}

// ClientOption is functional option modifying Serverless API client.
type ClientOption func(*Client)

var (
	elasticCloudAPIKeyEnv   = "EC_API_KEY"
	elasticCloudEndpointEnv = "EC_HOST"

	ErrProjectNotExist = errors.New("project does not exist")
)

func NewClient(opts ...ClientOption) (*Client, error) {
	apiKey := os.Getenv(elasticCloudAPIKeyEnv)
	if apiKey == "" {
		return nil, fmt.Errorf("unable to obtain value from %s environment variable", elasticCloudAPIKeyEnv)
	}
	c := &Client{
		host:   defaultHostURL,
		apiKey: apiKey,
	}
	for _, opt := range opts {
		opt(c)
	}

	host := os.Getenv(elasticCloudEndpointEnv)
	if host != "" {
		c.host = host
	}
	logger.Debugf("Using Elastic Cloud URL: %s", c.host)
	return c, nil
}

// WithAddress option sets the host to use to connect to Kibana.
func WithAddress(address string) ClientOption {
	return func(c *Client) {
		c.host = address
	}
}

// WithApiKey option sets the host to use to connect to Kibana.
func WithApiKey(apiKey string) ClientOption {
	return func(c *Client) {
		c.apiKey = apiKey
	}
}

func (c *Client) get(ctx context.Context, resourcePath string) (int, []byte, error) {
	return c.sendRequest(ctx, http.MethodGet, resourcePath, nil)
}

func (c *Client) post(ctx context.Context, resourcePath string, body []byte) (int, []byte, error) {
	return c.sendRequest(ctx, http.MethodPost, resourcePath, body)
}

func (c *Client) delete(ctx context.Context, resourcePath string) (int, []byte, error) {
	return c.sendRequest(ctx, http.MethodDelete, resourcePath, nil)
}

func (c *Client) sendRequest(ctx context.Context, method, resourcePath string, body []byte) (int, []byte, error) {
	request, err := c.newRequest(ctx, method, resourcePath, bytes.NewReader(body))
	if err != nil {
		return 0, nil, err
	}

	return c.doRequest(request)
}

func (c *Client) newRequest(ctx context.Context, method, resourcePath string, reqBody io.Reader) (*http.Request, error) {
	base, err := url.Parse(c.host)
	if err != nil {
		return nil, fmt.Errorf("could not create base URL from host: %v: %w", c.host, err)
	}

	rel, err := url.Parse(resourcePath)
	if err != nil {
		return nil, fmt.Errorf("could not create relative URL from resource path: %v: %w", resourcePath, err)
	}

	u := base.JoinPath(rel.EscapedPath())
	u.RawQuery = rel.RawQuery

	logger.Tracef("%s %s", method, u)

	req, err := http.NewRequestWithContext(ctx, method, u.String(), reqBody)
	if err != nil {
		return nil, fmt.Errorf("could not create %v request to Kibana API resource: %s: %w", method, resourcePath, err)
	}

	req.Header.Add("content-type", "application/json")
	req.Header.Add("Authorization", fmt.Sprintf("ApiKey %s", c.apiKey))

	return req, nil
}

func (c *Client) doRequest(request *http.Request) (int, []byte, error) {
	client := http.Client{}

	resp, err := client.Do(request)
	if err != nil {
		return 0, nil, fmt.Errorf("could not send request to Kibana API: %w", err)
	}

	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return resp.StatusCode, nil, fmt.Errorf("could not read response body: %w", err)
	}

	return resp.StatusCode, body, nil
}

func (c *Client) CreateProject(ctx context.Context, name, region, projectType string) (*Project, error) {
	ReqBody := struct {
		Name     string `json:"name"`
		RegionID string `json:"region_id"`
	}{
		Name:     name,
		RegionID: region,
	}
	p, err := json.Marshal(ReqBody)
	if err != nil {
		return nil, err
	}
	resourcePath, err := url.JoinPath(c.host, projectsAPI, projectType)
	if err != nil {
		return nil, fmt.Errorf("could not build the URL: %w", err)
	}
	statusCode, respBody, err := c.post(ctx, resourcePath, p)
	if err != nil {
		return nil, fmt.Errorf("error creating project: %w", err)
	}

	if statusCode != http.StatusCreated {
		return nil, fmt.Errorf("unexpected status code %d, body: %s", statusCode, string(respBody))
	}

	serverlessProject := &Project{url: c.host, apiKey: c.apiKey}
	err = json.Unmarshal(respBody, &serverlessProject)
	if err != nil {
		return nil, fmt.Errorf("error while decoding create project response: %w", err)
	}

	return serverlessProject, nil
}

func (c *Client) EnsureProjectInitialized(ctx context.Context, project *Project) error {
	timer := time.NewTimer(time.Millisecond)
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-timer.C:
		}

		status, err := c.StatusProject(ctx, project)
		if err != nil {
			logger.Debugf("error querying for status: %s", err.Error())
			timer.Reset(time.Second * 5)
			continue
		}

		if status != "initialized" {
			logger.Debugf("project not initialized, status: %s", status)
			timer.Reset(time.Second * 5)
			continue
		}

		return nil
	}
}

func (c *Client) StatusProject(ctx context.Context, project *Project) (string, error) {
	resourcePath, err := url.JoinPath(c.host, projectsAPI, project.Type, project.ID, "status")
	if err != nil {
		return "", fmt.Errorf("could not build the URL: %w", err)
	}
	statusCode, respBody, err := c.get(ctx, resourcePath)
	if err != nil {
		return "", fmt.Errorf("error getting status project: %w", err)
	}

	if statusCode != http.StatusOK {
		return "", fmt.Errorf("unexpected status code %d", statusCode)
	}

	var status struct {
		Phase string `json:"phase"`
	}

	if err := json.Unmarshal(respBody, &status); err != nil {
		return "", fmt.Errorf("unable to decode status: %w", err)
	}

	return status.Phase, nil
}

func (c *Client) DeleteProject(ctx context.Context, project *Project) error {
	resourcePath, err := url.JoinPath(c.host, projectsAPI, project.Type, project.ID)
	if err != nil {
		return fmt.Errorf("could not build the URL: %w", err)
	}
	statusCode, _, err := c.delete(ctx, resourcePath)
	if err != nil {
		return fmt.Errorf("error deleting project: %w", err)
	}

	if statusCode != http.StatusOK {
		return fmt.Errorf("unexpected status code %d", statusCode)
	}

	return nil
}

func (c *Client) GetProject(ctx context.Context, projectType, projectID string) (*Project, error) {
	resourcePath, err := url.JoinPath(c.host, projectsAPI, projectType, projectID)
	if err != nil {
		return nil, fmt.Errorf("could not build the URL: %w", err)
	}
	statusCode, respBody, err := c.get(ctx, resourcePath)
	if err != nil {
		return nil, fmt.Errorf("error getting project: %w", err)
	}

	if statusCode == http.StatusNotFound {
		return nil, ErrProjectNotExist
	}

	if statusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code %d", statusCode)
	}

	project := &Project{url: c.host, apiKey: c.apiKey}
	err = json.Unmarshal(respBody, &project)
	if err != nil {
		return nil, fmt.Errorf("failed to decode project: %w", err)
	}

	return project, nil
}

func (c *Client) EnsureEndpoints(ctx context.Context, project *Project) error {
	if project.Endpoints.Elasticsearch != "" {
		return nil
	}

	for {
		newProject, err := c.GetProject(ctx, project.Type, project.ID)
		switch {
		case err != nil:
			logger.Debugf("request error: %s", err.Error())
		case newProject.Endpoints.Elasticsearch != "":
			project.Endpoints = newProject.Endpoints
			return nil
		}
		logger.Debugf("Waiting for Elasticsearch endpoint for %s project %q", project.Type, project.ID)
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(time.Second * 5):
		}
	}
}
