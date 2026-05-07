// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package kibana

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/elastic/elastic-package/internal/logger"
)

// DashboardsAPI is the prefix for the Kibana dashboards-as-code import API.
const DashboardsAPI = "/api/dashboards"

// ImportDashboardAsCode imports a dashboards-as-code JSON document via the
// /api/dashboards Kibana endpoint and returns the saved-object id of the
// resulting dashboard. The same id is used to subsequently export and clean up
// the imported dashboard.
func (c *Client) ImportDashboardAsCode(ctx context.Context, body []byte) (string, error) {
	logger.Debug("Import dashboards-as-code via Kibana dashboards API")

	statusCode, respBody, err := c.post(ctx, DashboardsAPI, body)
	if err != nil {
		return "", fmt.Errorf("could not import dashboards-as-code; API status code = %d; response body = %s: %w", statusCode, string(respBody), err)
	}
	if statusCode != http.StatusOK && statusCode != http.StatusCreated {
		return "", fmt.Errorf("could not import dashboards-as-code; API status code = %d; response body = %s", statusCode, string(respBody))
	}

	var resp struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(respBody, &resp); err != nil {
		return "", fmt.Errorf("could not decode dashboards-as-code import response (body: %s): %w", respBody, err)
	}
	if resp.ID == "" {
		return "", errors.New("dashboards-as-code import response did not include an id")
	}
	return resp.ID, nil
}

// DeleteDashboard removes a dashboard by id via the dashboards-as-code API.
// This is used to clean up dashboards that were imported during the
// dashboards-as-code build step.
func (c *Client) DeleteDashboard(ctx context.Context, id string) error {
	path := fmt.Sprintf("%s/%s", DashboardsAPI, id)
	statusCode, respBody, err := c.delete(ctx, path)
	if err != nil {
		return fmt.Errorf("could not delete dashboard %s; API status code = %d; response body = %s: %w", id, statusCode, string(respBody), err)
	}
	if statusCode != http.StatusOK && statusCode != http.StatusNoContent && statusCode != http.StatusNotFound {
		return fmt.Errorf("could not delete dashboard %s; API status code = %d; response body = %s", id, statusCode, string(respBody))
	}
	return nil
}
