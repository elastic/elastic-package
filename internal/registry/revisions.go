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

	"github.com/google/go-querystring/query"

	"github.com/elastic/elastic-package/internal/packages"
)

// SearchOptions specify the query parameters for the search API
type SearchOptions struct {
	Internal     bool   `url:"internal"`
	Experimental bool   `url:"experimental"`
	All          bool   `url:"all"`
	Package      string `url:"package"`
}

// Revisions returns the deployed package revisions for a given package sorted by semantic version
func (c *Client) Revisions(packageName string, options SearchOptions) ([]packages.PackageManifest, error) {
	options.Package = packageName
	parameters, err := query.Values(options)
	if err != nil {
		return nil, errors.Wrap(err, "could not encode options as query parameters")
	}
	path := searchAPI + "?" + parameters.Encode()

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
