// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package kibana

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/Masterminds/semver/v3"

	"github.com/elastic/elastic-package/internal/packages"
)

const (
	// WorkflowsManagedAll lists both managed and unmanaged workflows.
	WorkflowsManagedAll = "all"
	// WorkflowsUnmanagedOnly lists only unmanaged workflows, the ones authored by users.
	WorkflowsUnmanagedOnly = "unmanaged"

	// fleetPackageWorkflowIDPrefix is the prefix of the IDs of workflows installed by Fleet from packages.
	fleetPackageWorkflowIDPrefix = "system-fleet-package-workflow-"

	workflowAssetType packages.AssetType = "workflow"

	workflowsAPIVersion  = "2023-10-31"
	workflowsMaxPageSize = 100
)

var workflowsMinKibanaVersion = semver.MustParse("9.4.0")

// Workflow is a workflow as returned by the Kibana Workflows API.
type Workflow struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Managed bool   `json:"managed"`
}

// String method returns a string representation of the workflow, used in prompts.
func (w Workflow) String() string {
	return fmt.Sprintf("%s (ID: %s)", w.Name, w.ID)
}

// ExportedWorkflow is a workflow as returned by the Kibana Workflows export API.
type ExportedWorkflow struct {
	ID   string `json:"id"`
	YAML string `json:"yaml"`
}

// ListWorkflowsOptions are the options to filter the workflows listed by ListWorkflows.
type ListWorkflowsOptions struct {
	// Managed filters by managed status, one of WorkflowsManagedAll or WorkflowsUnmanagedOnly.
	// Kibana defaults to WorkflowsUnmanagedOnly when empty.
	Managed string
	// Query is a free-text search query.
	Query string
}

type listWorkflowsResponse struct {
	Page    int        `json:"page"`
	Size    int        `json:"size"`
	Total   int        `json:"total"`
	Results []Workflow `json:"results"`
}

type exportWorkflowsRequest struct {
	IDs []string `json:"ids"`
}

type exportWorkflowsResponse struct {
	Entries []ExportedWorkflow `json:"entries"`
}

// ListWorkflows lists all the workflows matching the given options, paging through all the results.
func (c *Client) ListWorkflows(ctx context.Context, opts ListWorkflowsOptions) ([]Workflow, error) {
	if err := c.checkWorkflowsSupport(); err != nil {
		return nil, err
	}

	var workflows []Workflow
	for page := 1; ; page++ {
		query := url.Values{}
		query.Set("size", strconv.Itoa(workflowsMaxPageSize))
		query.Set("page", strconv.Itoa(page))
		if opts.Managed != "" {
			query.Set("managed", opts.Managed)
		}
		if opts.Query != "" {
			query.Set("query", opts.Query)
		}

		statusCode, respBody, err := c.sendWorkflowsRequest(ctx, http.MethodGet, "/api/workflows?"+query.Encode(), nil)
		if err != nil {
			return nil, fmt.Errorf("could not list workflows: %w", err)
		}
		if statusCode != http.StatusOK {
			return nil, fmt.Errorf("could not list workflows; API status code = %d; response body = %s", statusCode, respBody)
		}

		var resp listWorkflowsResponse
		if err := json.Unmarshal(respBody, &resp); err != nil {
			return nil, fmt.Errorf("could not decode workflows list response: %w", err)
		}

		workflows = append(workflows, resp.Results...)
		if len(resp.Results) == 0 || len(workflows) >= resp.Total {
			return workflows, nil
		}
	}
}

// ExportWorkflows exports the workflows with the given IDs, including their YAML definitions.
// All IDs are sent in a single request, as with dashboards export; if Kibana rejects the
// request for exceeding its per-request limit, its error is returned.
func (c *Client) ExportWorkflows(ctx context.Context, ids []string) ([]ExportedWorkflow, error) {
	if err := c.checkWorkflowsSupport(); err != nil {
		return nil, err
	}

	body, err := json.Marshal(exportWorkflowsRequest{IDs: ids})
	if err != nil {
		return nil, fmt.Errorf("could not encode workflows export request: %w", err)
	}

	statusCode, respBody, err := c.sendWorkflowsRequest(ctx, http.MethodPost, "/api/workflows/export", body)
	if err != nil {
		return nil, fmt.Errorf("could not export workflows: %w", err)
	}
	if statusCode != http.StatusOK {
		return nil, fmt.Errorf("could not export workflows; API status code = %d; response body = %s", statusCode, respBody)
	}

	var resp exportWorkflowsResponse
	if err := json.Unmarshal(respBody, &resp); err != nil {
		return nil, fmt.Errorf("could not decode workflows export response: %w", err)
	}

	exported := make(map[string]bool, len(resp.Entries))
	for _, entry := range resp.Entries {
		exported[entry.ID] = true
	}
	for _, id := range ids {
		if !exported[id] {
			return nil, fmt.Errorf("workflow %q not found", id)
		}
	}

	return resp.Entries, nil
}

// InstalledPackageWorkflowIDs returns the IDs of the workflows installed by Fleet for the given package.
// It returns an empty set if the package is not installed.
func (c *Client) InstalledPackageWorkflowIDs(ctx context.Context, pkgName string) (map[string]bool, error) {
	pkg, err := c.GetPackage(ctx, pkgName)
	var notFound *ErrPackageNotFound
	if errors.As(err, &notFound) {
		return map[string]bool{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("could not get package %s: %w", pkgName, err)
	}
	if pkg.InstallationInfo == nil {
		return map[string]bool{}, nil
	}

	ids := make(map[string]bool)
	for _, asset := range pkg.InstallationInfo.InstalledKibanaAssets {
		if asset.Type == workflowAssetType {
			ids[asset.ID] = true
		}
	}
	return ids, nil
}

// FleetPackageWorkflowFileBase returns the file base of the package file a Fleet-installed workflow was
// installed from, as encoded in its ID: lowercased and with "_" replaced by "-". It returns false if the
// ID is not the ID of a Fleet-installed workflow.
func FleetPackageWorkflowFileBase(id string) (string, bool) {
	rest, found := strings.CutPrefix(id, fleetPackageWorkflowIDPrefix)
	if !found {
		return "", false
	}
	// The ID is "{prefix}{hash(space)}-{hash(package)}-{fileBase}".
	parts := strings.SplitN(rest, "-", 3)
	if len(parts) != 3 || parts[2] == "" {
		return "", false
	}
	return parts[2], true
}

func (c *Client) checkWorkflowsSupport() error {
	// Version info may not contain any version if this is a managed Kibana.
	if c.semver == nil {
		return nil
	}
	v, err := c.semver.SetPrerelease("")
	if err != nil {
		return fmt.Errorf("could not parse Kibana version %s: %w", c.semver, err)
	}
	if v.LessThan(workflowsMinKibanaVersion) {
		return fmt.Errorf("workflows are not supported in Kibana %s, the minimum version is %s", c.semver, workflowsMinKibanaVersion)
	}
	return nil
}

func (c *Client) sendWorkflowsRequest(ctx context.Context, method, resourcePath string, body []byte) (int, []byte, error) {
	req, err := c.newRequest(ctx, method, resourcePath, bytes.NewReader(body))
	if err != nil {
		return 0, nil, err
	}
	req.Header.Set("elastic-api-version", workflowsAPIVersion)

	return c.doRequest(req)
}
