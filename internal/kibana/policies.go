// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package kibana

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/pkg/errors"

	"github.com/elastic/elastic-package/internal/packages"
)

// Policy represents an Agent Policy in Fleet.
type Policy struct {
	ID          string `json:"id,omitempty"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Namespace   string `json:"namespace"`
	Revision    int    `json:"revision,omitempty"`
}

// CreatePolicy persists the given Policy in Fleet.
func (c *Client) CreatePolicy(p Policy) (*Policy, error) {
	reqBody, err := json.Marshal(p)
	if err != nil {
		return nil, errors.Wrap(err, "could not convert policy (request) to JSON")
	}

	statusCode, respBody, err := c.post(fmt.Sprintf("%s/agent_policies", FleetAPI), reqBody)
	if err != nil {
		return nil, errors.Wrap(err, "could not create policy")
	}

	if statusCode != http.StatusOK {
		return nil, fmt.Errorf("could not create policy; API status code = %d; response body = %s", statusCode, respBody)
	}

	var resp struct {
		Item Policy `json:"item"`
	}

	if err := json.Unmarshal(respBody, &resp); err != nil {
		return nil, errors.Wrap(err, "could not convert policy (response) to JSON")
	}

	return &resp.Item, nil
}

// GetPolicy fetches the given Policy in Fleet.
func (c *Client) GetPolicy(policyID string) (*Policy, error) {
	statusCode, respBody, err := c.get(fmt.Sprintf("%s/agent_policies/%s", FleetAPI, policyID))
	if err != nil {
		return nil, errors.Wrap(err, "could not get policy")
	}

	if statusCode != http.StatusOK {
		return nil, fmt.Errorf("could not get policy; API status code = %d; response body = %s", statusCode, respBody)
	}

	var resp struct {
		Item Policy `json:"item"`
	}

	if err := json.Unmarshal(respBody, &resp); err != nil {
		return nil, errors.Wrap(err, "could not convert policy (response) to JSON")
	}

	return &resp.Item, nil
}

// GetRawPolicy fetches the given Policy with all the fields in Fleet.
func (c *Client) GetRawPolicy(policyID string) (json.RawMessage, error) {
	statusCode, respBody, err := c.get(fmt.Sprintf("%s/agent_policies/%s", FleetAPI, policyID))
	if err != nil {
		return nil, errors.Wrap(err, "could not get policy")
	}

	if statusCode != http.StatusOK {
		return nil, fmt.Errorf("could not get policy; API status code = %d; response body = %s", statusCode, respBody)
	}

	var resp struct {
		Item json.RawMessage `json:"item"`
	}

	if err := json.Unmarshal(respBody, &resp); err != nil {
		return nil, errors.Wrap(err, "could not convert policy (response) to JSON")
	}

	return resp.Item, nil
}

// ListRawPolicies fetches all the Policies in Fleet.
func (c *Client) ListRawPolicies() ([]json.RawMessage, error) {
	itemsRetrieved := 0
	currentPage := 1
	var items []json.RawMessage
	var resp struct {
		Items   []json.RawMessage `json:"items"`
		Total   int               `json:"total"`
		Page    int               `json:"page"`
		PerPage int               `json:"perPage"`
	}

	for finished := false; !finished; finished = itemsRetrieved == resp.Total {
		statusCode, respBody, err := c.get(fmt.Sprintf("%s/agent_policies?full=true&page=%d", FleetAPI, currentPage))
		if err != nil {
			return nil, errors.Wrap(err, "could not get policies")
		}

		if statusCode != http.StatusOK {
			return nil, fmt.Errorf("could not get policies; API status code = %d; response body = %s", statusCode, respBody)
		}

		if err := json.Unmarshal(respBody, &resp); err != nil {
			return nil, errors.Wrap(err, "could not convert policies (response) to JSON")
		}

		itemsRetrieved += len(resp.Items)
		currentPage += 1
		items = append(items, resp.Items...)
	}

	return items, nil
}

// DeletePolicy removes the given Policy from Fleet.
func (c *Client) DeletePolicy(p Policy) error {
	reqBody := `{ "agentPolicyId": "` + p.ID + `" }`

	statusCode, respBody, err := c.post(fmt.Sprintf("%s/agent_policies/delete", FleetAPI), []byte(reqBody))
	if err != nil {
		return errors.Wrap(err, "could not delete policy")
	}

	if statusCode != http.StatusOK {
		return fmt.Errorf("could not delete policy; API status code = %d; response body = %s", statusCode, respBody)
	}

	return nil
}

// Var represents a single variable at the package or
// data stream level, encapsulating the data type of the
// variable and it's value.
type Var struct {
	Value packages.VarValue `json:"value"`
	Type  string            `json:"type"`
}

// Vars is a collection of variables either at the package or
// data stream level.
type Vars map[string]Var

// DataStream represents a data stream within a package.
type DataStream struct {
	Type    string `json:"type"`
	Dataset string `json:"dataset"`
}

// Stream encapsulates a data stream and it's variables.
type Stream struct {
	ID         string     `json:"id"`
	Enabled    bool       `json:"enabled"`
	DataStream DataStream `json:"data_stream"`
	Vars       Vars       `json:"vars"`
}

// Input represents a package-level input.
type Input struct {
	PolicyTemplate string   `json:"policy_template,omitempty"` // Name of policy_template from the package manifest that contains this input. If not specified the Kibana uses the first policy_template.
	Type           string   `json:"type"`
	Enabled        bool     `json:"enabled"`
	Streams        []Stream `json:"streams"`
	Vars           Vars     `json:"vars"`
}

// PackageDataStream represents a request to add a single package's single data stream to a
// Policy in Fleet.
type PackageDataStream struct {
	Name        string  `json:"name"`
	Description string  `json:"description"`
	Namespace   string  `json:"namespace"`
	PolicyID    string  `json:"policy_id"`
	Enabled     bool    `json:"enabled"`
	OutputID    string  `json:"output_id"`
	Inputs      []Input `json:"inputs"`
	Package     struct {
		Name    string `json:"name"`
		Title   string `json:"title"`
		Version string `json:"version"`
	} `json:"package"`
}

// AddPackageDataStreamToPolicy adds a PackageDataStream to a Policy in Fleet.
func (c *Client) AddPackageDataStreamToPolicy(r PackageDataStream) error {
	reqBody, err := json.Marshal(r)
	if err != nil {
		return errors.Wrap(err, "could not convert policy-package (request) to JSON")
	}

	statusCode, respBody, err := c.post(fmt.Sprintf("%s/package_policies", FleetAPI), reqBody)
	if err != nil {
		return errors.Wrap(err, "could not add package to policy")
	}

	if statusCode != http.StatusOK {
		return fmt.Errorf("could not add package to policy; API status code = %d; response body = %s", statusCode, respBody)
	}

	return nil
}
