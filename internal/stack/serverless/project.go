package serverless

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// Project represents a serverless project
type Project struct {
	url    string
	apiKey string

	Name   string `json:"name"`
	ID     string `json:"id"`
	Type   string `json:"type"`
	Region string `json:"region_id"`

	Credentials struct {
		Username string `json:"username"`
		Password string `json:"password"`
	} `json:"credentials"`

	Endpoints struct {
		Elasticsearch string `json:"elasticsearch"`
		Kibana        string `json:"kibana"`
		Fleet         string `json:"fleet,omitempty"`
		APM           string `json:"apm,omitempty"`
	} `json:"endpoints"`
}

// NewObservabilityProject creates a new observability type project
func NewObservabilityProject(ctx context.Context, url, name, apiKey, region string) (*Project, error) {
	return newProject(ctx, url, name, apiKey, region, "observability")
}

// newProject creates a new serverless project
// Note that the Project.Endpoints may not be populated and another call may be required.
func newProject(ctx context.Context, url, name, apiKey, region, projectType string) (*Project, error) {
	ReqBody := struct {
		Name     string `json:"name"`
		RegionID string `json:"region_id"`
	}{
		Name:     name,
		RegionID: region,
	}
	p, err := json.Marshal(ReqBody)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, "POST", url+"/api/v1/serverless/projects/"+projectType, bytes.NewReader(p))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "ApiKey "+apiKey)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		p, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("unexpected status code %d, body: %s", resp.StatusCode, string(p))
	}
	project := &Project{url: url, apiKey: apiKey}

	err = json.NewDecoder(resp.Body).Decode(project)
	return project, err
}
