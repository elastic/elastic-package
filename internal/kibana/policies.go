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
	"path"

	"gopkg.in/yaml.v3"

	"github.com/elastic/elastic-package/internal/common"
	"github.com/elastic/elastic-package/internal/packages"
)

// Policy represents an Agent Policy in Fleet.
type Policy struct {
	ID                   string         `json:"id,omitempty"`
	Name                 string         `json:"name"`
	Description          string         `json:"description"`
	Namespace            string         `json:"namespace"`
	Revision             int            `json:"revision,omitempty"`
	MonitoringEnabled    []string       `json:"monitoring_enabled,omitempty"`
	MonitoringOutputID   string         `json:"monitoring_output_id,omitempty"`
	DataOutputID         string         `json:"data_output_id,omitempty"`
	IsDefaultFleetServer bool           `json:"is_default_fleet_server,omitempty"`
	Overrides            map[string]any `json:"overrides,omitempty"`
}

// DownloadedPolicy represents a policy as returned by the download policy API.
type DownloadedPolicy json.RawMessage

// CreatePolicy persists the given Policy in Fleet.
func (c *Client) CreatePolicy(ctx context.Context, p Policy) (*Policy, error) {
	reqBody, err := json.Marshal(p)
	if err != nil {
		return nil, fmt.Errorf("could not convert policy (request) to JSON: %w", err)
	}

	statusCode, respBody, err := c.post(ctx, fmt.Sprintf("%s/agent_policies", FleetAPI), reqBody)
	if err != nil {
		return nil, fmt.Errorf("could not create policy: %w", err)
	}

	if statusCode == http.StatusConflict {
		return nil, fmt.Errorf("could not create policy: %w", ErrConflict)
	}
	if statusCode != http.StatusOK {
		return nil, fmt.Errorf("could not create policy; API status code = %d; response body = %s", statusCode, respBody)
	}

	var resp struct {
		Item Policy `json:"item"`
	}

	if err := json.Unmarshal(respBody, &resp); err != nil {
		return nil, fmt.Errorf("could not convert policy (response) to JSON: %w", err)
	}

	return &resp.Item, nil
}

type ErrPolicyNotFound struct {
	id string
}

func (e *ErrPolicyNotFound) Error() string {
	return fmt.Sprintf("policy %s not found", e.id)
}

// GetPolicy fetches the given Policy in Fleet.
func (c *Client) GetPolicy(ctx context.Context, policyID string) (*Policy, error) {
	statusCode, respBody, err := c.get(ctx, fmt.Sprintf("%s/agent_policies/%s", FleetAPI, policyID))
	if err != nil {
		return nil, fmt.Errorf("could not get policy: %w", err)
	}
	if statusCode == http.StatusNotFound {
		return nil, &ErrPolicyNotFound{id: policyID}
	}
	if statusCode != http.StatusOK {
		return nil, fmt.Errorf("could not get policy; API status code = %d; response body = %s", statusCode, respBody)
	}

	var resp struct {
		Item Policy `json:"item"`
	}

	if err := json.Unmarshal(respBody, &resp); err != nil {
		return nil, fmt.Errorf("could not convert policy (response) to JSON: %w", err)
	}

	return &resp.Item, nil
}

// DownloadPolicy fetches the agent Policy as would be downloaded by an agent.
func (c *Client) DownloadPolicy(ctx context.Context, policyID string) (DownloadedPolicy, error) {
	statusCode, respBody, err := c.get(ctx, fmt.Sprintf("%s/agent_policies/%s/download", FleetAPI, policyID))
	if err != nil {
		return nil, fmt.Errorf("could not get policy: %w", err)
	}
	if statusCode == http.StatusNotFound {
		return nil, &ErrPolicyNotFound{id: policyID}
	}
	if statusCode != http.StatusOK {
		return nil, fmt.Errorf("could not get policy; API status code = %d; response body = %s", statusCode, respBody)
	}

	return respBody, nil
}

// GetRawPolicy fetches the given Policy with all the fields in Fleet.
func (c *Client) GetRawPolicy(ctx context.Context, policyID string) (json.RawMessage, error) {
	statusCode, respBody, err := c.get(ctx, fmt.Sprintf("%s/agent_policies/%s", FleetAPI, policyID))
	if err != nil {
		return nil, fmt.Errorf("could not get policy: %w", err)
	}
	if statusCode == http.StatusNotFound {
		return nil, &ErrPolicyNotFound{id: policyID}
	}
	if statusCode != http.StatusOK {
		return nil, fmt.Errorf("could not get policy; API status code = %d; response body = %s", statusCode, respBody)
	}

	var resp struct {
		Item json.RawMessage `json:"item"`
	}

	if err := json.Unmarshal(respBody, &resp); err != nil {
		return nil, fmt.Errorf("could not convert policy (response) to JSON: %w", err)
	}

	return resp.Item, nil
}

// ListRawPolicies fetches all the Policies in Fleet.
func (c *Client) ListRawPolicies(ctx context.Context) ([]json.RawMessage, error) {
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
		statusCode, respBody, err := c.get(ctx, fmt.Sprintf("%s/agent_policies?full=true&page=%d", FleetAPI, currentPage))
		if err != nil {
			return nil, fmt.Errorf("could not get policies: %w", err)
		}

		if statusCode != http.StatusOK {
			return nil, fmt.Errorf("could not get policies; API status code = %d; response body = %s", statusCode, respBody)
		}

		if err := json.Unmarshal(respBody, &resp); err != nil {
			return nil, fmt.Errorf("could not convert policies (response) to JSON: %w", err)
		}

		itemsRetrieved += len(resp.Items)
		currentPage += 1
		items = append(items, resp.Items...)
	}

	return items, nil
}

// DeletePolicy removes the given Policy from Fleet.
func (c *Client) DeletePolicy(ctx context.Context, policyID string) error {
	reqBody := `{ "agentPolicyId": "` + policyID + `" }`

	statusCode, respBody, err := c.post(ctx, fmt.Sprintf("%s/agent_policies/delete", FleetAPI), []byte(reqBody))
	if err != nil {
		return fmt.Errorf("could not delete policy: %w", err)
	}

	if statusCode == http.StatusNotFound {
		return &ErrPolicyNotFound{id: policyID}
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

// ToMapStr converts Vars to the map format expected by PackagePolicyInput and PackagePolicyStream.
// The objects-based Fleet API expects raw values (not {value, type} wrappers).
// Variables of type "yaml" whose value is not already a string are marshaled to a
// YAML string, as Fleet does not accept raw objects/arrays for these.
func (v Vars) ToMapStr() common.MapStr {
	if len(v) == 0 {
		return nil
	}
	m := make(common.MapStr, len(v))
	for k, val := range v {
		raw := val.Value.Value()
		if val.Type == "yaml" && raw != nil {
			if _, isString := raw.(string); !isString {
				b, err := yaml.Marshal(raw)
				if err == nil {
					m[k] = string(b)
					continue
				}
			}
		}
		m[k] = raw
	}
	return m
}

// SetKibanaVariables builds a Vars map by combining variable definitions with
// supplied override values. Definition defaults are used when no override is provided.
func SetKibanaVariables(definitions []packages.Variable, values common.MapStr) Vars {
	vars := Vars{}
	for _, definition := range definitions {
		val := definition.Default

		value, err := values.GetValue(definition.Name)
		if err == nil {
			val = &packages.VarValue{}
			val.Unpack(value)
		} else if errors.Is(err, common.ErrKeyNotFound) && definition.Default == nil {
			// Do not include nulls for unset variables.
			continue
		}

		vars[definition.Name] = Var{
			Type:  definition.Type,
			Value: *val,
		}
	}
	return vars
}

// PackagePolicy represents an Package Policy in Fleet.
type PackagePolicy struct {
	ID          string `json:"id,omitempty"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Namespace   string `json:"namespace"`
	PolicyID    string `json:"policy_id"`
	Package     struct {
		Name    string `json:"name"`
		Version string `json:"version"`
	} `json:"package"`
	Inputs map[string]PackagePolicyInput `json:"inputs,omitempty"`
	Vars   map[string]any                `json:"vars,omitempty"`
	Force  bool                          `json:"force"`
}

type PackagePolicyInput struct {
	Enabled bool                           `json:"enabled"`
	Vars    map[string]interface{}         `json:"vars,omitempty"`
	Streams map[string]PackagePolicyStream `json:"streams,omitempty"`
}

type PackagePolicyStream struct {
	Enabled bool                   `json:"enabled"`
	Vars    map[string]interface{} `json:"vars,omitempty"`
}

// CreatePackagePolicy persists the given Package Policy in Fleet.
func (c *Client) CreatePackagePolicy(ctx context.Context, p PackagePolicy) (*PackagePolicy, error) {
	reqBody, err := json.Marshal(p)
	if err != nil {
		return nil, fmt.Errorf("could not convert package policy (request) to JSON: %w", err)
	}

	statusCode, respBody, err := c.post(ctx, path.Join(FleetAPI, "package_policies"), reqBody)
	if err != nil {
		return nil, fmt.Errorf("could not create package policy (req %s): %w", reqBody, err)
	}

	if statusCode != http.StatusOK {
		return nil, fmt.Errorf("could not create package policy (req %s); API status code = %d; response body = %s", string(reqBody), statusCode, respBody)
	}

	// Response format for the policy is inconsistent with the creation one,
	// we update the ID to avoid having a full type only to hold the response.
	var resp struct {
		Item struct {
			ID string `json:"id"`
		} `json:"item"`
	}

	if err := json.Unmarshal(respBody, &resp); err != nil {
		return nil, fmt.Errorf("could not convert package policy (response) to JSON: %w", err)
	}

	p.ID = resp.Item.ID

	return &p, nil
}

// ListRawPackagePolicies fetches all the Package Policies in Fleet.
func (c *Client) ListRawPackagePolicies(ctx context.Context) ([]json.RawMessage, error) {
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
		statusCode, respBody, err := c.get(ctx, fmt.Sprintf("%s?showUpgradeable=true&page=%d", path.Join(FleetAPI, "package_policies"), currentPage))
		if err != nil {
			return nil, fmt.Errorf("could not get policies: %w", err)
		}

		if statusCode != http.StatusOK {
			return nil, fmt.Errorf("could not get policies; API status code = %d; response body = %s", statusCode, respBody)
		}

		if err := json.Unmarshal(respBody, &resp); err != nil {
			return nil, fmt.Errorf("could not convert policies (response) to JSON: %w", err)
		}

		itemsRetrieved += len(resp.Items)
		currentPage += 1
		items = append(items, resp.Items...)
	}

	return items, nil
}

// UpgradePackagePolicyToLatest upgrades the given package in Fleet to the latest available version.
func (c *Client) UpgradePackagePolicyToLatest(ctx context.Context, policyIDs ...string) error {
	var req struct {
		PackagePolicyIds []string `json:"packagePolicyIds"`
	}
	req.PackagePolicyIds = policyIDs
	body, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("could not convert package policy (request) to JSON: %w", err)
	}
	statusCode, respBody, err := c.post(ctx, path.Join(FleetAPI, "package_policies/upgrade"), body)
	if err != nil {
		return fmt.Errorf("could not create package policy (req %s): %w", body, err)
	}
	if statusCode == http.StatusBadRequest {
		var resp struct {
			Message string `json:"message"`
		}
		err := json.Unmarshal(respBody, &resp)
		if err != nil {
			return fmt.Errorf("could not upgrade package: %q", respBody)
		}
		return fmt.Errorf("could not upgrade package: %s", resp.Message)
	}
	if statusCode != http.StatusOK {
		return fmt.Errorf("could not create package policy (req %s); API status code = %d; response body = %s", body, statusCode, respBody)
	}
	return nil
}

// DeletePackagePolicy removes the given Package Policy from Fleet.
func (c *Client) DeletePackagePolicy(ctx context.Context, p PackagePolicy) error {
	statusCode, respBody, err := c.delete(ctx, path.Join(FleetAPI, "package_policies", p.ID))
	if err != nil {
		return fmt.Errorf("could not delete package policy: %w", err)
	}

	if statusCode != http.StatusOK {
		return fmt.Errorf("could not delete package policy; API status code = %d; response body = %s", statusCode, respBody)
	}

	return nil
}
