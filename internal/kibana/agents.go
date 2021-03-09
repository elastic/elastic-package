// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package kibana

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/elastic/elastic-package/internal/logger"

	"github.com/pkg/errors"
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
	} `json:"local_metadata"`
}

// ListAgents returns the list of agents enrolled with Fleet.
func (c *Client) ListAgents() ([]Agent, error) {
	statusCode, respBody, err := c.get(fmt.Sprintf("%s/agents", FleetAPI))
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

	path := fmt.Sprintf("%s/agents/%s/reassign", FleetAPI, a.ID)
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
	for {
		agents, err := c.ListAgents()
		if err != nil {
			return errors.Wrap(err, "can't list available agents")
		}

		agentsWithPolicy := filterAgentsByPolicy(agents, p)
		agentsWithPolicyAndRevision := filterAgentsByPolicyAndRevision(agents, p)
		if len(agentsWithPolicy) != 0 && len(agentsWithPolicy) == len(agentsWithPolicyAndRevision) {
			logger.Debugf("Policy revision assigned to all agents")
			break
		}

		logger.Debugf("Wait until the policy (ID: %s, revision: %d) is assigned to all agents (%d/%d)...", p.ID, p.Revision,
			len(agentsWithPolicyAndRevision), len(agentsWithPolicy))
		time.Sleep(2 * time.Second)
	}
	return nil
}

func filterAgentsByPolicy(agents []Agent, policy Policy) []Agent {
	var filtered []Agent
	for _, agent := range agents {
		if agent.PolicyID == policy.ID {
			filtered = append(filtered, agent)
		}
	}
	return filtered
}

func filterAgentsByPolicyAndRevision(agents []Agent, policy Policy) []Agent {
	var filtered []Agent
	for _, agent := range agents {
		if agent.PolicyID == policy.ID && agent.PolicyRevision == policy.Revision {
			filtered = append(filtered, agent)
		}
	}
	return filtered
}
