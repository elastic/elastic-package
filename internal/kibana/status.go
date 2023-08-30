// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package kibana

import (
	"encoding/json"
	"fmt"
	"net/http"
)

const SNAPSHOT_SUFFIX = "-SNAPSHOT"

type VersionInfo struct {
	Number        string `json:"number"`
	BuildSnapshot bool   `json:"build_snapshot"`
}

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

func (c *Client) requestVersion() (VersionInfo, error) {
	var version VersionInfo
	statusCode, respBody, err := c.get(StatusAPI)
	if err != nil {
		return version, fmt.Errorf("could not reach status endpoint: %w", err)
	}

	if statusCode != http.StatusOK {
		return version, fmt.Errorf("could not get status data; API status code = %d; response body = %s", statusCode, respBody)
	}

	var status statusType
	err = json.Unmarshal(respBody, &status)
	if err != nil {
		return version, fmt.Errorf("unmarshalling response failed (body: \n%s): %w", respBody, err)
	}

	return status.Version, nil
}

// CheckHealth checks the Kibana health
func (c *Client) CheckHealth() error {
	statusCode, respBody, err := c.get(StatusAPI)
	if err != nil {
		return fmt.Errorf("could not reach status endpoint: %w", err)
	}

	if statusCode != http.StatusOK {
		return fmt.Errorf("could not get status data; API status code = %d; response body = %s", statusCode, respBody)
	}

	var status statusType
	err = json.Unmarshal(respBody, &status)
	if err != nil {
		return fmt.Errorf("unmarshalling response failed (body: \n%s): %w", respBody, err)
	}

	if status.Status.Overall.Level != "available" {
		return fmt.Errorf("kibana in unhealthy state: %s", status.Status.Overall.Level)
	}
	return nil
}
