// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package kibana

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/pkg/errors"

	"github.com/elastic/elastic-package/internal/common"
	"github.com/elastic/elastic-package/internal/logger"
	"github.com/elastic/elastic-package/internal/multierror"
)

type exportedType struct {
	Objects []common.MapStr `json:"objects"`
}

// Export method exports selected dashboards using the Kibana Export API.
func (c *Client) Export(dashboardIDs []string) ([]common.MapStr, error) {
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
		return nil, errors.Wrapf(err, "could not export dashboards; API status code = %d; response body = %s", statusCode, string(respBody))
	}

	var exported exportedType
	err = json.Unmarshal(respBody, &exported)
	if err != nil {
		return nil, errors.Wrapf(err, "unmarshalling response failed (body: \n%s)", string(respBody))
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
		return nil, errors.Wrap(multiErr, "at least Kibana object returned an error")
	}
	return exported.Objects, nil
}
