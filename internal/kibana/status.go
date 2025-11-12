// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package kibana

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

const SNAPSHOT_SUFFIX = "-SNAPSHOT"

type VersionInfo struct {
	Number        string `json:"number"`
	BuildSnapshot bool   `json:"build_snapshot"`
	BuildFlavor   string `json:"build_flavor"`
}

const ServerlessFlavor = "serverless"

func (v VersionInfo) Version() string {
	if v.BuildSnapshot {
		return fmt.Sprintf("%s%s", v.Number, SNAPSHOT_SUFFIX)
	}
	return v.Number
}

func (v VersionInfo) IsSnapshot() bool {
	return v.BuildSnapshot
}

type statusType struct {
	Version VersionInfo `json:"version"`
	Status  struct {
		Overall struct {
			Level string `json:"level"`
		} `json:"overall"`
	} `json:"status"`
}

// Version method returns the version of Kibana (Elastic stack)
func (c *Client) Version() (VersionInfo, error) {
	return c.versionInfo, nil
}

func (c *Client) requestStatus(ctx context.Context) (statusType, error) {
	var status statusType
	statusCode, respBody, err := c.get(ctx, StatusAPI)
	if err != nil {
		return status, fmt.Errorf("could not reach status endpoint: %w", err)
	}

	// Kibana can respond with 503 when it is unavailable, but its status response is valid.
	if statusCode != http.StatusOK && statusCode != http.StatusServiceUnavailable {
		return status, fmt.Errorf("could not get status data; API status code = %d; response body = %s", statusCode, respBody)
	}

	err = json.Unmarshal(respBody, &status)
	if err != nil {
		return status, fmt.Errorf("unmarshalling response failed (body: \n%s): %w", respBody, err)
	}

	return status, nil
}

// CheckHealth checks the Kibana health
func (c *Client) CheckHealth(ctx context.Context) error {
	status, err := c.requestStatus(ctx)
	if err != nil {
		return fmt.Errorf("could not reach status endpoint: %w", err)
	}

	if status.Status.Overall.Level != "available" {
		return fmt.Errorf("kibana in unhealthy state: %s", status.Status.Overall.Level)
	}
	return nil
}
