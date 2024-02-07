// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package kibana

import (
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

type AgentSSL struct {
	Ca_authorities []string `json:"certificate_authorities,omitempty"`
	Certificate    string   `json:"certificate,omitempty"`
	Key            string   `json:"key,omitempty"`
}

// DefaultFleetServerURL returns the default Fleet server configured in Kibana
func (c *Client) DefaultFleetServerURL() (string, error) {
	path := fmt.Sprintf("%s/fleet_server_hosts", FleetAPI)

	statusCode, respBody, err := c.get(path)
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

	return "", errors.New("could not find the fleet server URL for this project")
}

// UpdateFleetOutput updates an existing output to fleet
// For example, to update ssl certificates etc.,
func (c *Client) UpdateFleetOutput(fo FleetOutput, outputId string) error {
	reqBody, err := json.Marshal(fo)
	if err != nil {
		return fmt.Errorf("could not convert fleetOutput (request) to JSON: %w", err)
	}

	statusCode, respBody, err := c.put(fmt.Sprintf("%s/outputs/%s", FleetAPI, outputId), reqBody)
	if err != nil {
		return fmt.Errorf("could not update fleet output: %w", err)
	}

	if statusCode != http.StatusOK {
		return fmt.Errorf("could not update fleet output; API status code = %d; response body = %s", statusCode, respBody)
	}

	return nil
}

// AddFleetOutput adds an additional output to fleet eg., logstash
func (c *Client) AddFleetOutput(fo FleetOutput) error {
	reqBody, err := json.Marshal(fo)
	if err != nil {
		return fmt.Errorf("could not convert fleetOutput (request) to JSON: %w", err)
	}

	statusCode, respBody, err := c.post(fmt.Sprintf("%s/outputs", FleetAPI), reqBody)
	if err != nil {
		return fmt.Errorf("could not create fleet output: %w", err)
	}

	if statusCode != http.StatusOK {
		return fmt.Errorf("could not add fleet output; API status code = %d; response body = %s", statusCode, respBody)
	}

	return nil
}

func (c *Client) SetAgentLogLevel(agentID, level string) error {
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

	statusCode, respBody, err := c.post(path, reqBody)
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
