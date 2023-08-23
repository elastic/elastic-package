// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package kibana

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
)

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
