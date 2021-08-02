// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package registry

import (
	"io"
	"net/http"
	"net/url"

	"github.com/pkg/errors"
)

const (
	productionURL = "https://epr.elastic.co"
	stagingURL    = "https://epr-staging.elastic.co"
	snapshotURL   = "https://epr-snapshot.elastic.co"
)

var (
	// Production is a pre-configured production client
	Production = NewClient(productionURL)
	// Staging is a pre-configured staging client
	Staging = NewClient(stagingURL)
	// Snapshot is a pre-configured snapshot client
	Snapshot = NewClient(snapshotURL)
)

// Client is responsible for exporting dashboards from Kibana.
type Client struct {
	baseURL string
}

// NewClient creates a new instance of the client.
func NewClient(baseURL string) *Client {
	return &Client{
		baseURL: baseURL,
	}
}

func (c *Client) get(resourcePath string) (int, []byte, error) {
	base, err := url.Parse(c.baseURL)
	if err != nil {
		return 0, nil, errors.Wrapf(err, "could not parse base URL: %v", c.baseURL)
	}

	rel, err := url.Parse(resourcePath)
	if err != nil {
		return 0, nil, errors.Wrapf(err, "could not create relative URL from resource path: %v", resourcePath)
	}

	u := base.ResolveReference(rel)

	req, err := http.NewRequest(http.MethodGet, u.String(), nil)
	if err != nil {
		return 0, nil, errors.Wrapf(err, "could not create request to Package Registry API resource: %s", resourcePath)
	}

	client := http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return 0, nil, errors.Wrap(err, "could not send request to Package Registry API")
	}

	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return resp.StatusCode, nil, errors.Wrap(err, "could not read response body")
	}

	return resp.StatusCode, body, nil
}
