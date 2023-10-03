// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package kibana

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/Masterminds/semver/v3"

	"github.com/elastic/elastic-package/internal/common"
	"github.com/elastic/elastic-package/internal/logger"
	"github.com/elastic/elastic-package/internal/multierror"
)

type exportedType struct {
	Objects []common.MapStr `json:"objects"`
}

// Export method exports selected dashboards using the Kibana APIs.
func (c *Client) Export(dashboardIDs []string) ([]common.MapStr, error) {
	if c.semver.LessThan(semver.MustParse("8.11.0")) {
		return c.exportWithDashboardsAPI(dashboardIDs)
	}

	return c.exportWithSavedObjectsAPI(dashboardIDs)
}

type exportSavedObjectsRequest struct {
	ExcludeExportDetails  bool                              `json:"excludeExportDetails"`
	IncludeReferencesDeep bool                              `json:"includeReferencesDeep"`
	Objects               []exportSavedObjectsRequestObject `json:"objects"`
}

type exportSavedObjectsRequestObject struct {
	ID   string `json:"id"`
	Type string `json:"type"`
}

func (c *Client) exportWithSavedObjectsAPI(dashboardIDs []string) ([]common.MapStr, error) {
	logger.Debug("Export dashboards using the Kibana Saved Objects Export API")

	exportRequest := exportSavedObjectsRequest{
		ExcludeExportDetails:  true,
		IncludeReferencesDeep: true,
	}
	for _, dashboardID := range dashboardIDs {
		exportRequest.Objects = append(exportRequest.Objects, exportSavedObjectsRequestObject{
			ID:   dashboardID,
			Type: "dashboard",
		})
	}

	body, err := json.Marshal(exportRequest)
	if err != nil {
		return nil, err
	}

	path := SavedObjectsAPI + "/_export"
	statusCode, respBody, err := c.SendRequest(http.MethodPost, path, body)
	if err != nil {
		return nil, fmt.Errorf("could not export dashboards; API status code = %d; response body = %s: %w", statusCode, respBody, err)
	}

	var dashboards []common.MapStr
	decoder := json.NewDecoder(bytes.NewReader(respBody))

	for decoder.More() {
		var dashboard common.MapStr
		err := decoder.Decode(&dashboard)
		if err != nil {
			return nil, fmt.Errorf("unmarshalling response failed (body: \n%s): %w", respBody, err)
		}

		dashboards = append(dashboards, dashboard)
	}

	return dashboards, nil
}

func (c *Client) exportWithDashboardsAPI(dashboardIDs []string) ([]common.MapStr, error) {
	logger.Debug("Export dashboards using the Kibana Export API")

	var query strings.Builder
	query.WriteByte('?')
	for _, dashboardID := range dashboardIDs {
		query.WriteString("dashboard=")
		query.WriteString(dashboardID)
		query.WriteByte('&')
	}

	path := fmt.Sprintf("%s/dashboards/export%s", CoreAPI, query.String())
	statusCode, respBody, err := c.get(path)
	if err != nil {
		return nil, fmt.Errorf("could not export dashboards; API status code = %d; response body = %s: %w", statusCode, respBody, err)
	}

	var exported exportedType
	err = json.Unmarshal(respBody, &exported)
	if err != nil {
		return nil, fmt.Errorf("unmarshalling response failed (body: \n%s): %w", respBody, err)
	}

	var multiErr multierror.Error
	for _, obj := range exported.Objects {
		errMsg, err := obj.GetValue("error.message")
		if errMsg != nil {
			multiErr = append(multiErr, errors.New(errMsg.(string)))
			continue
		}
		if err != nil && err != common.ErrKeyNotFound {
			multiErr = append(multiErr, err)
		}
	}

	if len(multiErr) > 0 {
		return nil, fmt.Errorf("at least Kibana object returned an error: %w", multiErr)
	}
	return exported.Objects, nil
}
