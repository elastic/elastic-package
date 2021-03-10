// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package kibana

import (
	"encoding/json"
	"fmt"

	"github.com/pkg/errors"

	"github.com/elastic/elastic-package/internal/packages"
)

// InstallPackage installs the given package in Fleet.
func (c *Client) InstallPackage(pkg packages.PackageManifest) ([]packages.Asset, error) {
	path := fmt.Sprintf("%s/epm/packages/%s-%s", FleetAPI, pkg.Name, pkg.Version)
	reqBody := []byte(`{"force":true}`) // allows installing older versions of the package being tested

	statusCode, respBody, err := c.post(path, reqBody)
	if err != nil {
		return nil, errors.Wrap(err, "could not install package")
	}

	return processResults("install", statusCode, respBody)
}

// RemovePackage removes the given package from Fleet.
func (c *Client) RemovePackage(pkg packages.PackageManifest) ([]packages.Asset, error) {
	path := fmt.Sprintf("%s/epm/packages/%s-%s", FleetAPI, pkg.Name, pkg.Version)
	statusCode, respBody, err := c.delete(path)
	if err != nil {
		return nil, errors.Wrap(err, "could not delete package")
	}

	return processResults("remove", statusCode, respBody)
}

func processResults(action string, statusCode int, respBody []byte) ([]packages.Asset, error) {
	if statusCode != 200 {
		return nil, fmt.Errorf("could not %s package; API status code = %d; response body = %s", action, statusCode, respBody)
	}

	var resp struct {
		Assets []packages.Asset `json:"response"`
	}

	if err := json.Unmarshal(respBody, &resp); err != nil {
		return nil, errors.Wrapf(err, "could not convert %s package (response) to JSON", action)
	}

	return resp.Assets, nil
}
