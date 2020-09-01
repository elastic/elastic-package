package ingestmanager

import (
	"encoding/json"
	"fmt"

	"github.com/pkg/errors"

	"github.com/elastic/elastic-package/internal/packages"
)

type Policy struct {
	ID          string `json:"id,omitempty"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Namespace   string `json:"namespace"`
}

func (c *Client) CreatePolicy(p Policy) (*Policy, error) {
	reqBody, err := json.Marshal(p)
	if err != nil {
		return nil, errors.Wrap(err, "could not convert policy (request) to JSON")
	}

	statusCode, respBody, err := c.post("agent_policies", reqBody)
	if err != nil {
		return nil, errors.Wrap(err, "could not create policy")
	}

	if statusCode != 200 {
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

func (c *Client) DeletePolicy(p Policy) error {
	reqBody := `{ "agentPolicyId": "` + p.ID + `" }`

	statusCode, respBody, err := c.post("agent_policies/delete", []byte(reqBody))
	if err != nil {
		return errors.Wrap(err, "could not delete policy")
	}

	if statusCode != 200 {
		return fmt.Errorf("could not delete policy; API status code = %d; response body = %s", statusCode, respBody)
	}

	return nil
}

type VarType struct {
	Value packages.VarValue `json:"value"`
	Type  string            `json:"type"`
}

type Vars map[string]VarType

type Datastream struct {
	Type    string `json:"type"`
	Dataset string `json:"dataset"`
}

type Stream struct {
	ID         string     `json:"id"`
	Enabled    bool       `json:"enabled"`
	DataStream Datastream `json:"data_stream"`
	Vars       Vars       `json:"vars"`
}

type Input struct {
	Type    string   `json:"type"`
	Enabled bool     `json:"enabled"`
	Streams []Stream `json:"streams"`
	Vars    Vars     `json:"vars"`
}

type PackageDatastream struct {
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

func (c *Client) AddPackageDataStreamToPolicy(r PackageDatastream) error {
	reqBody, err := json.Marshal(r)
	if err != nil {
		return errors.Wrap(err, "could not convert policy-package (request) to JSON")
	}

	statusCode, respBody, err := c.post("package_policies", reqBody)
	if err != nil {
		return errors.Wrap(err, "could not add package to policy")
	}

	if statusCode != 200 {
		return fmt.Errorf("could not add package to policy; API status code = %d; response body = %s", statusCode, respBody)
	}

	return nil
}
