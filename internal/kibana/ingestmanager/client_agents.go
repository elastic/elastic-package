package ingestmanager

import (
	"encoding/json"
	"fmt"

	"github.com/pkg/errors"
)

type Agent struct {
	ID       string `json:"id"`
	PolicyID string `json:"policy_id"`
}

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

func (c *Client) AssignPolicyToAgent(a Agent, p Policy) error {
	reqBody := `{ "policy_id": "` + p.ID + `" }`

	path := fmt.Sprintf("fleet/agents/%s/reassign", a.ID)
	statusCode, respBody, err := c.put(path, []byte(reqBody))
	if err != nil {
		return errors.Wrap(err, "could not assign policy to agent")
	}

	if statusCode != 200 {
		return fmt.Errorf("could not assign policy to agent; API status code = %d; response body = %s", statusCode, respBody)
	}

	return nil

}
