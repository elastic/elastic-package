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
	"net/url"
	"time"

	"github.com/elastic/elastic-package/internal/logger"
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
	Status string `json:"status"`
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
func (c *Client) ListAgents(ctx context.Context) ([]Agent, error) {
	return c.QueryAgents(ctx, "")
}

// QueryAgents returns the list of agents enrolled with Fleet that satisfy a kibana query.
func (c *Client) QueryAgents(ctx context.Context, kuery string) ([]Agent, error) {
	resource := fmt.Sprintf("%s/agents", FleetAPI)
	if kuery != "" {
		values := make(url.Values)
		values.Set("kuery", kuery)
		resource += "?" + values.Encode()
	}
	statusCode, respBody, err := c.get(ctx, resource)
	if err != nil {
		return nil, fmt.Errorf("could not list agents: %w", err)
	}

	if statusCode != http.StatusOK {
		return nil, fmt.Errorf("could not list agents; API status code = %d; response body = %s", statusCode, respBody)
	}

	var resp struct {
		List  []Agent `json:"list"`
		Items []Agent `json:"items"`
	}

	if err := json.Unmarshal(respBody, &resp); err != nil {
		return nil, fmt.Errorf("could not convert list agents (response) to JSON: %w", err)
	}

	switch {
	case c.semver != nil && c.semver.Major() < 9:
		return resp.List, nil
	default:
		return resp.Items, nil
	}
}

// AssignPolicyToAgent assigns the given Policy to the given Agent.
func (c *Client) AssignPolicyToAgent(ctx context.Context, a Agent, p Policy) error {
	reqBody := `{ "policy_id": "` + p.ID + `" }`
	path := fmt.Sprintf("%s/agents/%s/reassign", FleetAPI, a.ID)

	var statusCode int
	var err error
	var respBody []byte
	switch {
	case c.semver != nil && c.semver.Major() < 9:
		statusCode, respBody, err = c.put(ctx, path, []byte(reqBody))
	default:
		statusCode, respBody, err = c.post(ctx, path, []byte(reqBody))
	}
	if err != nil {
		return fmt.Errorf("could not assign policy to agent: %w", err)
	}

	if statusCode != http.StatusOK {
		return fmt.Errorf("could not assign policy to agent; API status code = %d; response body = %s", statusCode, respBody)
	}

	err = c.waitUntilPolicyAssigned(ctx, a, p)
	if err != nil {
		return fmt.Errorf("error occurred while waiting for the policy to be assigned to all agents: %w", err)
	}
	return nil
}

// RemoveAgent unenrolls the given agent
func (c *Client) RemoveAgent(ctx context.Context, a Agent) error {
	reqBody := `{ "revoke": true, "force": true }`

	path := fmt.Sprintf("%s/agents/%s/unenroll", FleetAPI, a.ID)
	statusCode, respBody, err := c.post(ctx, path, []byte(reqBody))
	if err != nil {
		return fmt.Errorf("could not enroll agent: %w", err)
	}

	if statusCode != http.StatusOK {
		return fmt.Errorf("could not enroll agent; API status code = %d; response body = %s", statusCode, respBody)
	}

	return nil
}

func (c *Client) waitUntilPolicyAssigned(ctx context.Context, a Agent, p Policy) error {
	ctx, cancel := context.WithTimeout(ctx, waitForPolicyAssignedTimeout)
	defer cancel()
	ticker := time.NewTicker(waitForPolicyAssignedRetryPeriod)
	defer ticker.Stop()

	logger.Debugf("Wait until the policy (ID: %s, revision: %d) is assigned to the agent (ID: %s)...", p.ID, p.Revision, a.ID)
	for {
		agent, err := c.getAgent(ctx, a.ID)
		if err != nil {
			return fmt.Errorf("can't get the agent: %w", err)
		}
		logger.Debugf("Agent %s (Host: %s): Policy ID %s LogLevel: %s Status: %s",
			agent.ID, agent.LocalMetadata.Host.Name, agent.PolicyID, agent.LocalMetadata.Elastic.Agent.LogLevel, agent.Status)

		if agent.PolicyID == p.ID && agent.PolicyRevision >= p.Revision {
			logger.Debugf("Policy revision assigned to the agent (ID: %s)...", a.ID)
			break
		}

		select {
		case <-ctx.Done():
			if errors.Is(ctx.Err(), context.DeadlineExceeded) {
				return errors.New("timeout: policy hasn't been assigned in time")
			}
			return ctx.Err()
		case <-ticker.C:
			continue
		}

	}
	return nil
}

func (c *Client) getAgent(ctx context.Context, agentID string) (*Agent, error) {
	statusCode, respBody, err := c.get(ctx, fmt.Sprintf("%s/agents/%s", FleetAPI, agentID))
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
