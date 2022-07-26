// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package registry

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sort"

	"github.com/Masterminds/semver"
	"github.com/google/go-querystring/query"
	"github.com/pkg/errors"

	"github.com/elastic/elastic-package/internal/packages"
)

// SearchOptions specify the query parameters without the package name for the search API
type SearchOptions struct {
	Prerelease    bool   `url:"prerelease"`
	All           bool   `url:"all"`
	KibanaVersion string `url:"kibana.version,omitempty"`

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
		return nil, errors.Wrap(err, "could not encode options as query parameters")
	}
	path := searchAPI + "?" + parameters.Encode()

	statusCode, respBody, err := c.get(path)
	if err != nil {
		return nil, errors.Wrap(err, "could not retrieve package")
	}
	if statusCode != http.StatusOK {
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
