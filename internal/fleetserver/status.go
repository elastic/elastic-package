// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package fleetserver

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/url"

	"github.com/elastic/elastic-package/internal/logger"
)

type Status struct {
	Name   string `json:"name"`
	Status string `json:"status"`

	// Version is only present if client is authenticated.
	Version struct {
		Number string `json:"number"`
	} `json:"version"`
}

func (c *Client) Status(ctx context.Context) (*Status, error) {
	statusURL, err := url.JoinPath(c.address, "/api/status")
	if err != nil {
		return nil, fmt.Errorf("could not build URL: %w", err)
	}
	logger.Tracef("GET %s", statusURL)
	req, err := c.httpRequest(ctx, "GET", statusURL, nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed (url: %s): %w", statusURL, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("unexpected status code %v", resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}
	var status Status
	err = json.Unmarshal(body, &status)
	if err != nil {
		return nil, fmt.Errorf("failed to parse response body: %w", err)
	}

	return &status, nil
}
