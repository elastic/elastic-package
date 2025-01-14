// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package kibana

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"
)

type FleetOutput struct {
	ID    string    `json:"id,omitempty"`
	Name  string    `json:"name,omitempty"`
	Hosts []string  `json:"hosts,omitempty"`
	Type  string    `json:"type,omitempty"`
	SSL   *AgentSSL `json:"ssl,omitempty"`
}

type FleetServerHost struct {
	ID   string   `json:"id,omitempty"`
	URLs []string `json:"host_urls"`
	Name string   `json:"name"`

	// TODO: Avoid using is_default, so a cluster can be used for multiple environments.
	IsDefault bool `json:"is_default"`
}

type AgentSSL struct {
	CertificateAuthorities []string `json:"certificate_authorities,omitempty"`
	Certificate            string   `json:"certificate,omitempty"`
	Key                    string   `json:"key,omitempty"`
}

var ErrFleetServerNotFound = errors.New("could not find a fleet server URL")

// DefaultFleetServerURL returns the default Fleet server configured in Kibana
func (c *Client) DefaultFleetServerURL(ctx context.Context) (string, error) {
	path := fmt.Sprintf("%s/fleet_server_hosts", FleetAPI)

	statusCode, respBody, err := c.get(ctx, path)
	if err != nil {
		return "", fmt.Errorf("could not reach fleet server hosts endpoint: %w", err)
	}

	if statusCode != http.StatusOK {
		return "", fmt.Errorf("could not get status data; API status code = %d; response body = %s", statusCode, respBody)
	}

	var hosts struct {
		Items []struct {
			IsDefault bool     `json:"is_default"`
			HostURLs  []string `json:"host_urls"`
		} `json:"items"`
	}
	err = json.Unmarshal(respBody, &hosts)
	if err != nil {
		return "", fmt.Errorf("failed to decode response: %w", err)
	}

	for _, server := range hosts.Items {
		if server.IsDefault && len(server.HostURLs) > 0 {
			return server.HostURLs[0], nil
		}
	}

	return "", ErrFleetServerNotFound
}

// UpdateFleetOutput updates an existing output to fleet
// For example, to update ssl certificates etc.,
func (c *Client) UpdateFleetOutput(ctx context.Context, fo FleetOutput, outputId string) error {
	reqBody, err := json.Marshal(fo)
	if err != nil {
		return fmt.Errorf("could not convert fleetOutput (request) to JSON: %w", err)
	}

	statusCode, respBody, err := c.put(ctx, fmt.Sprintf("%s/outputs/%s", FleetAPI, outputId), reqBody)
	if err != nil {
		return fmt.Errorf("could not update fleet output: %w", err)
	}

	if statusCode != http.StatusOK {
		return fmt.Errorf("could not update fleet output; API status code = %d; response body = %s", statusCode, respBody)
	}

	return nil
}

// AddFleetOutput adds an additional output to fleet eg., logstash
func (c *Client) AddFleetOutput(ctx context.Context, fo FleetOutput) error {
	reqBody, err := json.Marshal(fo)
	if err != nil {
		return fmt.Errorf("could not convert fleetOutput (request) to JSON: %w", err)
	}

	statusCode, respBody, err := c.post(ctx, fmt.Sprintf("%s/outputs", FleetAPI), reqBody)
	if err != nil {
		return fmt.Errorf("could not create fleet output: %w", err)
	}

	if statusCode == http.StatusConflict {
		return fmt.Errorf("could not add fleet output: %w", ErrConflict)
	}
	if statusCode != http.StatusOK {
		return fmt.Errorf("could not add fleet output; API status code = %d; response body = %s", statusCode, respBody)
	}

	return nil
}

// RemoveFleetOutput removes an output from Fleet
func (c *Client) RemoveFleetOutput(ctx context.Context, outputID string) error {
	statusCode, respBody, err := c.delete(ctx, fmt.Sprintf("%s/outputs/%s", FleetAPI, outputID))
	if err != nil {
		return fmt.Errorf("could not delete fleet output: %w", err)
	}

	if statusCode == http.StatusNotFound {
		// Already removed, ignore error.
		return nil
	}
	if statusCode != http.StatusOK {
		return fmt.Errorf("could not remove fleet output; API status code = %d; response body = %s", statusCode, respBody)
	}

	return nil
}

func (c *Client) SetAgentLogLevel(ctx context.Context, agentID, level string) error {
	path := fmt.Sprintf("%s/agents/%s/actions", FleetAPI, agentID)

	type fleetAction struct {
		Action struct {
			Type string `json:"type"`
			Data struct {
				LogLevel string `json:"log_level"`
			} `json:"data"`
		} `json:"action"`
	}

	action := fleetAction{}
	action.Action.Type = "SETTINGS"
	action.Action.Data.LogLevel = level

	reqBody, err := json.Marshal(action)
	if err != nil {
		return fmt.Errorf("could not convert action settingr (request) to JSON: %w", err)
	}

	statusCode, respBody, err := c.post(ctx, path, reqBody)
	if err != nil {
		return fmt.Errorf("could not update agent settings: %w", err)
	}

	if statusCode != http.StatusOK {
		return fmt.Errorf("could not set new log level; API status code = %d; response body = %s", statusCode, respBody)
	}

	type actionResponse struct {
		ID        string    `json:"id"`
		CreatedAt time.Time `json:"created_at"`
		Type      string    `json:"type"`
		Data      struct {
			LogLevel string `json:"log_level"`
		} `json:"data"`
		Agents []string `json:"agents"`
	}
	var resp struct {
		Item actionResponse `json:"item"`
	}

	if err := json.Unmarshal(respBody, &resp); err != nil {
		return fmt.Errorf("could not convert actions agent (response) to JSON: %w", err)
	}
	return nil
}

func (c *Client) AddFleetServerHost(ctx context.Context, host FleetServerHost) error {
	reqBody, err := json.Marshal(host)
	if err != nil {
		return fmt.Errorf("could not convert fleet server host to JSON: %w", err)
	}

	statusCode, respBody, err := c.post(ctx, fmt.Sprintf("%s/fleet_server_hosts", FleetAPI), reqBody)
	if err != nil {
		return fmt.Errorf("could not add fleet server host: %w", err)
	}

	if statusCode == http.StatusConflict {
		return fmt.Errorf("could not add fleet server host: %w", ErrConflict)
	}
	if statusCode != http.StatusOK {
		return fmt.Errorf("could not add fleet server host; API status code = %d; response body = %s", statusCode, respBody)
	}

	return nil
}

func (c *Client) UpdateFleetServerHost(ctx context.Context, host FleetServerHost) error {
	if host.ID == "" {
		return fmt.Errorf("host id required when updating fleet server host")
	}

	// Payload should not contain the ID, it is set in the URL.
	id := host.ID
	host.ID = ""
	reqBody, err := json.Marshal(host)
	if err != nil {
		return fmt.Errorf("could not convert fleet server host to JSON: %w", err)
	}

	statusCode, respBody, err := c.put(ctx, fmt.Sprintf("%s/fleet_server_hosts/%s", FleetAPI, id), reqBody)
	if err != nil {
		return fmt.Errorf("could not update fleet server host: %w", err)
	}

	if statusCode != http.StatusOK {
		return fmt.Errorf("could not update fleet server host; API status code = %d; response body = %s", statusCode, respBody)
	}

	return nil
}

// CreateFleetServiceToken creates a service token for Fleet, to be used when enrolling Fleet Servers.
func (c *Client) CreateFleetServiceToken(ctx context.Context) (string, error) {
	statusCode, respBody, err := c.post(ctx, fmt.Sprintf("%s/service_tokens", FleetAPI), nil)
	if err != nil {
		return "", fmt.Errorf("could not request fleet service token: %w", err)
	}

	if statusCode != http.StatusOK {
		return "", fmt.Errorf("could not request fleet service token; API status code = %d; response body = %s", statusCode, respBody)
	}

	var resp struct {
		Name  string `json:"name"`
		Value string `json:"value"`
	}
	if err := json.Unmarshal(respBody, &resp); err != nil {
		return "", fmt.Errorf("could not convert actions agent (response) to JSON: %w", err)
	}

	return resp.Value, nil
}
