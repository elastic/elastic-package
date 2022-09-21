// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package kibana

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/pkg/errors"
)

const SNAPSHOT_SUFFIX = "-SNAPSHOT"

type VersionInfo struct {
	Number        string `json:"number"`
	BuildSnapshot bool   `json:"build_snapshot"`
}

func (v VersionInfo) Version() string {
	if v.BuildSnapshot {
		return fmt.Sprintf("%s%s", v.Number, SNAPSHOT_SUFFIX)
	}
	return v.Number
}

func (v VersionInfo) IsSnapshot() bool {
	return v.BuildSnapshot
}

type statusType struct {
	Version VersionInfo `json:"version"`
}

// Version method returns the version of Kibana (Elastic stack)
func (c *Client) Version() (VersionInfo, error) {
	var version VersionInfo
	statusCode, respBody, err := c.get(StatusAPI)
	if err != nil {
		return version, errors.Wrapf(err, "could not reach status endpoint")
	}

	if statusCode != http.StatusOK {
		return version, fmt.Errorf("could not get status data; API status code = %d; response body = %s", statusCode, respBody)
	}

	var status statusType
	err = json.Unmarshal(respBody, &status)
	if err != nil {
		return version, errors.Wrapf(err, "unmarshalling response failed (body: \n%s)", respBody)
	}

	return status.Version, nil
}
