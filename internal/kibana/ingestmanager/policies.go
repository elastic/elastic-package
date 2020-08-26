package ingestmanager

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/pkg/errors"

	"github.com/elastic/elastic-package/internal/logger"
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

	statusCode, respBody, err := c.post("agent_policies", bytes.NewReader(reqBody))
	if err != nil {
		return nil, errors.Wrap(err, "could not create policy")
	}

	if statusCode != 200 {
		return nil, fmt.Errorf("could not create policy; API status code = %d", statusCode)
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

	statusCode, _, err := c.post("agent_policies/delete", strings.NewReader(reqBody))
	if err != nil {
		return errors.Wrap(err, "could not delete policy")
	}

	if statusCode != 200 {
		return fmt.Errorf("could not delete policy; API status code = %d", statusCode)
	}

	return nil
}

type varType struct {
	Value packages.VarValue `json:"value"`
	Type  string            `json:"type"`
}

type vars map[string]varType

type datastream struct {
	Type    string `json:"type"`
	Dataset string `json:"dataset"`
}

type stream struct {
	ID         string     `json:"id"`
	Enabled    bool       `json:"enabled"`
	DataStream datastream `json:"data_stream"`
	Vars       vars       `json:"vars"`
}

type input struct {
	Type    string   `json:"type"`
	Enabled bool     `json:"enabled"`
	Streams []stream `json:"streams"`
	Vars    vars     `json:"vars"`
}

func (c *Client) AddPackageDataStreamToPolicy(p Policy, pkg packages.PackageManifest, ds packages.DatasetManifest) error {
	logger.Info("adding package datastream to policy")
	streamInput := ds.Streams[0].Input
	r := struct {
		Name        string  `json:"name"`
		Description string  `json:"description"`
		Namespace   string  `json:"namespace"`
		PolicyID    string  `json:"policy_id"`
		Enabled     bool    `json:"enabled"`
		OutputID    string  `json:"output_id"`
		Inputs      []input `json:"inputs"`
		Package     struct {
			Name    string `json:"name"`
			Title   string `json:"title"`
			Version string `json:"version"`
		} `json:"package"`
	}{
		Name:      fmt.Sprintf("%s-%s", pkg.Name, ds.Name),
		Namespace: "ep",
		PolicyID:  p.ID,
		Enabled:   true,
	}

	r.Package.Name = pkg.Name
	r.Package.Title = pkg.Title
	r.Package.Version = pkg.Version

	r.Inputs = []input{
		{
			Type:    streamInput,
			Enabled: true,
		},
	}

	streams := []stream{
		{
			ID:      fmt.Sprintf("%s-%s.%s", streamInput, pkg.Name, ds.Name),
			Enabled: true,
			DataStream: datastream{
				Type:    ds.Type,
				Dataset: fmt.Sprintf("%s.%s", pkg.Name, ds.Name),
			},
		},
	}

	// Add dataset-level vars
	dsVars := vars{}
	for _, dsVar := range ds.Streams[0].Vars {
		dsVars[dsVar.Name] = varType{
			Type:  dsVar.Type,
			Value: dsVar.Default,
		}
		// TODO: overlay var values from test configuration
	}
	streams[0].Vars = dsVars
	r.Inputs[0].Streams = streams

	// Add package-level vars
	pkgVars := vars{}
	input := pkg.ConfigTemplates[0].FindInputByType(streamInput)
	if input != nil {
		for _, pkgVar := range input.Vars {
			pkgVars[pkgVar.Name] = varType{
				Type:  pkgVar.Type,
				Value: pkgVar.Default,
			}
			// TODO: overlay var values from test configuration
		}
	}
	r.Inputs[0].Vars = pkgVars

	reqBody, err := json.Marshal(r)
	if err != nil {
		return errors.Wrap(err, "could not convert policy-package (request) to JSON")
	}

	fmt.Println("reqBody:", string(reqBody))

	statusCode, respBody, err := c.post("package_policies", bytes.NewReader(reqBody))
	if err != nil {
		return errors.Wrap(err, "could not add package to policy")
	}

	if statusCode != 200 {
		fmt.Println(string(respBody))
		return fmt.Errorf("could not add package to policy; API status code = %d", statusCode)
	}

	return nil
}
