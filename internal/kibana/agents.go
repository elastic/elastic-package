// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package kibana

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/elastic/elastic-package/internal/logger"
	"github.com/elastic/elastic-package/internal/signal"
)

var (
	waitForPolicyAssignedTimeout     = 10 * time.Minute
	waitForPolicyAssignedRetryPeriod = 2 * time.Second
)

// Agent represents an Elastic Agent enrolled with fleet.
type Agent struct {
	ID             string `json:"id"`
	PolicyID       string `json:"policy_id"`
	PolicyRevision int    `json:"policy_revision,omitempty"`
	LocalMetadata  struct {
		Host struct {
			Name string `json:"name"`
		} `json:"host"`
		Elastic struct {
			Agent struct {
				LogLevel string `json:"log_level"`
			} `json:"agent"`
		} `json:"elastic"`
	} `json:"local_metadata"`
}

// String method returns string representation of an agent.
func (a *Agent) String() string {
	b, err := json.Marshal(a)
	if err != nil {
		return err.Error()
	}
	return string(b)
}

// ListAgents returns the list of agents enrolled with Fleet.
func (c *Client) ListAgents() ([]Agent, error) {
	statusCode, respBody, err := c.get(fmt.Sprintf("%s/agents", FleetAPI))
	if err != nil {
		return nil, fmt.Errorf("could not list agents: %w", err)
	}

	if statusCode != http.StatusOK {
		return nil, fmt.Errorf("could not list agents; API status code = %d; response body = %s", statusCode, respBody)
	}

	var resp struct {
		List []Agent `json:"list"`
	}

	if err := json.Unmarshal(respBody, &resp); err != nil {
		return nil, fmt.Errorf("could not convert list agents (response) to JSON: %w", err)
	}

	return resp.List, nil
}

// AssignPolicyToAgent assigns the given Policy to the given Agent.
func (c *Client) AssignPolicyToAgent(a Agent, p Policy) error {
	reqBody := `{ "policy_id": "` + p.ID + `" }`

	path := fmt.Sprintf("%s/agents/%s/reassign", FleetAPI, a.ID)
	statusCode, respBody, err := c.put(path, []byte(reqBody))
	if err != nil {
		return fmt.Errorf("could not assign policy to agent: %w", err)
	}

	if statusCode != http.StatusOK {
		return fmt.Errorf("could not assign policy to agent; API status code = %d; response body = %s", statusCode, respBody)
	}

	err = c.waitUntilPolicyAssigned(a, p)
	if err != nil {
		return fmt.Errorf("error occurred while waiting for the policy to be assigned to all agents: %w", err)
	}
	return nil
}

func (c *Client) waitUntilPolicyAssigned(a Agent, p Policy) error {
	timeout := time.NewTimer(waitForPolicyAssignedTimeout)
	defer timeout.Stop()
	ticker := time.NewTicker(waitForPolicyAssignedRetryPeriod)
	defer ticker.Stop()

	for {
		if signal.SIGINT() {
			return errors.New("SIGINT: cancel waiting for policy assigned")
		}

		agent, err := c.getAgent(a.ID)
		if err != nil {
			return fmt.Errorf("can't get the agent: %w", err)
		}
		logger.Debugf("Agent data: %s", agent.String())

		if agent.PolicyID == p.ID && agent.PolicyRevision >= p.Revision {
			logger.Debugf("Policy revision assigned to the agent (ID: %s)...", a.ID)
			break
		}

		logger.Debugf("Wait until the policy (ID: %s, revision: %d) is assigned to the agent (ID: %s)...", p.ID, p.Revision, a.ID)
		select {
		case <-timeout.C:
			return errors.New("timeout: policy hasn't been assigned in time")
		case <-ticker.C:
			continue
		}

	}
	return nil
}

func (c *Client) getAgent(agentID string) (*Agent, error) {
	statusCode, respBody, err := c.get(fmt.Sprintf("%s/agents/%s", FleetAPI, agentID))
	if err != nil {
		return nil, fmt.Errorf("could not list agents: %w", err)
	}

	if statusCode != http.StatusOK {
		return nil, fmt.Errorf("could not list agents; API status code = %d; response body = %s", statusCode, respBody)
	}

	var resp struct {
		Item Agent `json:"item"`
	}
	if err := json.Unmarshal(respBody, &resp); err != nil {
		return nil, fmt.Errorf("could not convert list agents (response) to JSON: %w", err)
	}
	return &resp.Item, nil
}
