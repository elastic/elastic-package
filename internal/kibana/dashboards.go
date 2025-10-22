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
func (c *Client) Export(ctx context.Context, dashboardIDs []string) ([]common.MapStr, error) {
	if c.semver.LessThan(semver.MustParse("9.0.0")) {
		return c.exportWithDashboardsAPI(ctx, dashboardIDs)
	}

	return c.exportWithSavedObjectsAPI(ctx, dashboardIDs)
}

func (c *Client) exportWithSavedObjectsAPI(ctx context.Context, dashboardIDs []string) ([]common.MapStr, error) {
	logger.Debug("Export dashboards using the Kibana Saved Objects Export API")

	request := ExportSavedObjectsRequest{
		ExcludeExportDetails:  true,
		IncludeReferencesDeep: true,
	}
	for _, dashboardID := range dashboardIDs {
		request.Objects = append(request.Objects, ExportSavedObjectsRequestObject{
			ID:   dashboardID,
			Type: "dashboard",
		})
	}

	return c.ExportSavedObjects(ctx, request)
}

func (c *Client) exportWithDashboardsAPI(ctx context.Context, dashboardIDs []string) ([]common.MapStr, error) {
	logger.Debug("Export dashboards using the Kibana Export API")

	var query strings.Builder
	query.WriteByte('?')
	for _, dashboardID := range dashboardIDs {
		query.WriteString("dashboard=")
		query.WriteString(dashboardID)
		query.WriteByte('&')
	}

	path := fmt.Sprintf("%s/dashboards/export%s", CoreAPI, query.String())
	statusCode, respBody, err := c.get(ctx, path)
	if err != nil {
		return nil, fmt.Errorf("could not export dashboards; API status code = %d; response body = %s: %w", statusCode, respBody, err)
	}

	if statusCode != http.StatusOK {
		return nil, fmt.Errorf("could not export dashboards; API status code = %d; response body = %s", statusCode, respBody)
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
