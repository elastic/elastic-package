// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package ingestmanager

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/elastic/elastic-package/internal/logger"

	"github.com/pkg/errors"
)

// Agent represents an Elastic Agent enrolled with fleet.
type Agent struct {
	ID       string `json:"id"`
	PolicyID string `json:"policy_id"`
}

// ListAgents returns the list of agents enrolled with Fleet.
func (c *Client) ListAgents() ([]Agent, error) {
	statusCode, respBody, err := c.get("fleet/agents")
	if err != nil {
		return nil, errors.Wrap(err, "could not list agents")
	}

	if statusCode != 200 {
		return nil, fmt.Errorf("could not list agents; API status code = %d", statusCode)
	}

	var resp struct {
		List []Agent `json:"list"`
	}

	if err := json.Unmarshal(respBody, &resp); err != nil {
		return nil, errors.Wrap(err, "could not convert list agents (response) to JSON")
	}

	return resp.List, nil
}

// AssignPolicyToAgent assigns the given Policy to the given Agent.
func (c *Client) AssignPolicyToAgent(a Agent, p Policy) error {
	reqBody := `{ "policy_id": "` + p.ID + `" }`

	path := fmt.Sprintf("fleet/agents/%s/reassign", a.ID)
	statusCode, respBody, err := c.put(path, []byte(reqBody))
	if err != nil {
		return errors.Wrap(err, "could not assign policy to agent")
	}

	if statusCode != 200 {
		return fmt.Errorf("could not assign policy to agent; API status code = %d; response body = %s", statusCode, string(respBody))
	}

	err = c.waitUntilPolicyAssigned(p)
	if err != nil {
		return errors.Wrap(err, "error occurred while waiting for the policy to be assigned to all agents")
	}
	return nil
}

func (c *Client) waitUntilPolicyAssigned(p Policy) error {
	path := fmt.Sprintf("fleet/agent-status?policyId=%s", p.ID)

	var assigned bool
	for !assigned {
		statusCode, respBody, err := c.get(path)
		if err != nil {
			return errors.Wrapf(err, "could not check agent status; API status code = %d; policy ID = %s; response body = %s", statusCode, p.ID, string(respBody))
		}

		var resp struct {
			Results struct {
				Online int `json:"online"`
				Total  int `json:"total"`
			} `json:"results"`
		}

		if err := json.Unmarshal(respBody, &resp); err != nil {
			return errors.Wrap(err, "could not convert agent status (response) to JSON")
		}

		if resp.Results.Total == 0 {
			return fmt.Errorf("no agent is available")
		}

		if resp.Results.Total == resp.Results.Online {
			assigned = true
		}

		logger.Debugf("Wait until the policy (ID: %s) is assigned to all agents...", p.ID)
		time.Sleep(2 * time.Second)
	}
	return nil
}
