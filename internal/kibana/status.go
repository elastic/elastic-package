// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package kibana

import (
	"encoding/json"
	"fmt"

	"github.com/pkg/errors"
)

type statusType struct {
	Version struct {
		Number        string `json:"number"`
		BuildSnapshot bool   `json:"build_snapshot"`
	} `json:"version"`
}

// Version method returns the version of Kibana (Elastic stack)
func (c *Client) Version() (string, bool, error) {
	statusCode, respBody, err := c.get(StatusAPI)
	if err != nil {
		return "", false, errors.Wrapf(err, "could not get Status data; API status code = %d; response body = %s", statusCode, respBody)
	}

	var status statusType
	err = json.Unmarshal(respBody, &status)
	if err != nil {
		return "", false, errors.Wrapf(err, "unmarshalling response failed (body: \n%s)", respBody)
	}

	stackVersion := status.Version.Number
	if status.Version.BuildSnapshot {
		stackVersion = fmt.Sprintf("%s-SNAPSHOT", stackVersion)
	}

	return stackVersion, status.Version.BuildSnapshot, nil
}
