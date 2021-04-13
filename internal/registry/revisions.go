// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package registry

import (
	"encoding/json"
	"fmt"
	"sort"

	"github.com/Masterminds/semver"
	"github.com/pkg/errors"

	"github.com/elastic/elastic-package/internal/logger"
	"github.com/elastic/elastic-package/internal/packages"
)

// Revisions returns the deployed package revisions for a given package sorted by semantic version
func (c *Client) Revisions(packageName string, showAll bool) ([]packages.PackageManifest, error) {
	logger.Debug("Export dashboards using the Kibana Export API")

	path := searchAPI + "?internal=true&experimental=true&package=" + packageName
	if showAll {
		path += "&all=true"
	}

	statusCode, respBody, err := c.get(path)
	if err != nil {
		return nil, errors.Wrap(err, "could not retrieve package")
	}
	if statusCode != 200 {
		return nil, fmt.Errorf("could not retrieve package; API status code = %d; response body = %s", statusCode, respBody)
	}

	var packageManifests []packages.PackageManifest
	if err := json.Unmarshal(respBody, &packageManifests); err != nil {
		return nil, errors.Wrap(err, "could not convert package manifests from JSON")
	}
	sort.Slice(packageManifests, func(i, j int) bool {
		firstVersion, err := semver.NewVersion(packageManifests[i].Version)
		if err != nil {
			return true
		}
		secondVersion, err := semver.NewVersion(packageManifests[j].Version)
		if err != nil {
			return false
		}
		return firstVersion.LessThan(secondVersion)
	})
	return packageManifests, nil
}
