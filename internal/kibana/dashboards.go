// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package kibana

import (
	"encoding/json"
	"io/ioutil"
	"net/http"
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

	request, err := http.NewRequest(http.MethodGet, c.host+"/api/kibana/dashboards/export"+query.String(), nil)
	if err != nil {
		return nil, errors.Wrap(err, "building HTTP request failed")
	}
	request.SetBasicAuth(c.username, c.password)

	response, err := http.DefaultClient.Do(request)
	if err != nil {
		return nil, errors.Wrap(err, "sending HTTP request failed")
	}
	defer response.Body.Close()

	body, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return nil, errors.Wrap(err, "reading response body failed")
	}

	var exported exportedType
	err = json.Unmarshal(body, &exported)
	if err != nil {
		return nil, errors.Wrapf(err, "unmarshalling response failed (body: \n%s)", string(body))
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
