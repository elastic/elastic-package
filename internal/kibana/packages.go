// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package kibana

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"

	"github.com/elastic/elastic-package/internal/packages"
)

// InstallPackage installs the given package in Fleet.
func (c *Client) InstallPackage(name, version string) ([]packages.Asset, error) {
	path := fmt.Sprintf("%s/epm/packages/%s-%s", FleetAPI, name, version)
	reqBody := []byte(`{"force":true}`) // allows installing older versions of the package being tested

	statusCode, respBody, err := c.post(path, reqBody)
	if err != nil {
		return nil, fmt.Errorf("could not install package: %w", err)
	}

	return processResults("install", statusCode, respBody)
}

// InstallZipPackage installs the local zip package in Fleet.
func (c *Client) InstallZipPackage(zipFile string) ([]packages.Asset, error) {
	path := fmt.Sprintf("%s/epm/packages", FleetAPI)

	body, err := os.Open(zipFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read zip file: %w", err)
	}
	defer body.Close()

	req, err := c.newRequest(http.MethodPost, path, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/zip")

	statusCode, respBody, err := c.doRequest(req)
	if err != nil {
		return nil, fmt.Errorf("could not install zip package: %w", err)
	}

	return processResults("zip-install", statusCode, respBody)
}

// RemovePackage removes the given package from Fleet.
func (c *Client) RemovePackage(name, version string) ([]packages.Asset, error) {
	path := fmt.Sprintf("%s/epm/packages/%s-%s", FleetAPI, name, version)
	statusCode, respBody, err := c.delete(path)
	if err != nil {
		return nil, fmt.Errorf("could not delete package: %w", err)
	}

	return processResults("remove", statusCode, respBody)
}

func processResults(action string, statusCode int, respBody []byte) ([]packages.Asset, error) {
	if statusCode != http.StatusOK {
		return nil, fmt.Errorf("could not %s package; API status code = %d; response body = %s", action, statusCode, respBody)
	}

	var resp struct {
		Assets []packages.Asset `json:"response"`
	}

	if err := json.Unmarshal(respBody, &resp); err != nil {
		return nil, fmt.Errorf("could not convert %s package (response) to JSON: %w", action, err)
	}

	return resp.Assets, nil
}
