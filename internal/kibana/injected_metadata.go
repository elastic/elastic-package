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

// InjectedMetadata represents the Kibana metadata structure exposed in the web UI.
type InjectedMetadata struct {
	// Stack version
	Version string `json:"version"`
}

// InjectedMetadata method returns the Kibana metadata. The metadata is always present, even if the client is
// unauthorized.
func (c *Client) InjectedMetadata() (*InjectedMetadata, error) {
	statusCode, respBody, err := c.get("/login")
	if err != nil {
		return nil, errors.Wrap(err, "could not reach login endpoint")
	}
	if statusCode != http.StatusOK {
		return nil, fmt.Errorf("could not reach login endpoint; API status code = %d; response body = %s", statusCode, string(respBody))
	}

	im, err := extractInjectedMetadata(respBody)
	if err != nil {
		return nil, errors.Wrap(err, "can't extract injected metadata")
	}
	return im, nil
}

func extractInjectedMetadata(body []byte) (*InjectedMetadata, error) {
	rawInjectedMetadata, err := extractRawInjectedMetadata(body)
	if err != nil {
		return nil, errors.Wrap(err, "can't extract raw metadata")
	}

	var im InjectedMetadata
	err = json.Unmarshal(rawInjectedMetadata, &im)
	if err != nil {
		return nil, errors.Wrap(err, "can't unmarshal raw injected metadata")
	}
	return &im, nil
}

func extractRawInjectedMetadata(body []byte) ([]byte, error) {
	return nil, nil // TODO
}
