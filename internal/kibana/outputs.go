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

// Output represents an Output in Fleet.
type Output struct {
	ID                  string   `json:"id,omitempty"`
	Name                string   `json:"name"`
	Type                string   `json:"type"`
	IsDefault           bool     `json:"is_default"`
	IsDefaultMonitoring bool     `json:"is_default_monitoring"`
	Hosts               []string `json:"hosts"`
}

// CreateOutput persists the given Output in Fleet.
func (c *Client) CreateOutput(o Output) (*Output, error) {
	reqBody, err := json.Marshal(o)
	if err != nil {
		return nil, errors.Wrap(err, "could not convert output (request) to JSON")
	}

	statusCode, respBody, err := c.post(fmt.Sprintf("%s/outputs", FleetAPI), reqBody)
	if err != nil {
		return nil, errors.Wrap(err, "could not create output")
	}

	if statusCode != http.StatusOK {
		return nil, fmt.Errorf("could not create output; API status code = %d; response body = %s", statusCode, respBody)
	}

	var resp struct {
		Item Output `json:"item"`
	}

	if err := json.Unmarshal(respBody, &resp); err != nil {
		return nil, errors.Wrap(err, "could not convert output (response) to JSON")
	}

	return &resp.Item, nil
}

// DeleteOutput removes the given Output from Fleet.
func (c *Client) DeleteOutput(o Output) error {
	statusCode, respBody, err := c.delete(fmt.Sprintf("%s/outputs/%s", FleetAPI, o.ID))
	if err != nil {
		return errors.Wrap(err, "could not delete output")
	}

	if statusCode != http.StatusOK {
		return fmt.Errorf("could not delete output; API status code = %d; response body = %s", statusCode, respBody)
	}

	return nil
}
