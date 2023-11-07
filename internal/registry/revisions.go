// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package registry

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sort"

	"github.com/Masterminds/semver/v3"
	"github.com/google/go-querystring/query"

	"github.com/elastic/elastic-package/internal/packages"
)

// SearchOptions specify the query parameters without the package name for the search API
type SearchOptions struct {
	All           bool     `url:"all"`
	Capabilities  []string `url:"capabilities,omitempty"`
	KibanaVersion string   `url:"kibana.version,omitempty"`
	Prerelease    bool     `url:"prerelease"`
	SpecMax       string   `url:"spec.max,omitempty"`
	SpecMin       string   `url:"spec.min,omitempty"`

	// Deprecated
	Experimental bool `url:"experimental"`
}

// searchQuery specify the package and query parameters for the search API
type searchQuery struct {
	SearchOptions
	Package string `url:"package"`
}

// Revisions returns the deployed package revisions for a given package sorted by semantic version
func (c *Client) Revisions(packageName string, options SearchOptions) ([]packages.PackageManifest, error) {
	parameters, err := query.Values(searchQuery{
		SearchOptions: options,
		Package:       packageName,
	})
	if err != nil {
		return nil, fmt.Errorf("could not encode options as query parameters: %w", err)
	}
	path := searchAPI + "?" + parameters.Encode()

	statusCode, respBody, err := c.get(path)
	if err != nil {
		return nil, fmt.Errorf("could not retrieve package: %w", err)
	}
	if statusCode != http.StatusOK {
		return nil, fmt.Errorf("could not retrieve package; API status code = %d; response body = %s", statusCode, respBody)
	}

	var packageManifests []packages.PackageManifest
	if err := json.Unmarshal(respBody, &packageManifests); err != nil {
		return nil, fmt.Errorf("could not convert package manifests from JSON: %w", err)
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
