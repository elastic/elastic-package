package serverless

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"

	"github.com/elastic/elastic-package/internal/environment"
	"github.com/elastic/elastic-package/internal/logger"
)

type Client struct {
	host   string
	apiKey string
}

// ClientOption is functional option modifying Serverless API client.
type ClientOption func(*Client)

var (
	ServerlessApiKeyEnvironmentVariable = "SERVERLESS_API_KEY"
	ServerlessHostvironmentVariable     = "SERVERLESS_HOST"

	ErrProjectNotExist = errors.New("project does not exist")
)

func NewClient(opts ...ClientOption) (*Client, error) {
	hostEnvName := environment.WithElasticPackagePrefix(ServerlessHostvironmentVariable)
	host := os.Getenv(hostEnvName)
	if host == "" {
		return nil, fmt.Errorf("unable to obtain value from %s environment variable", hostEnvName)
	}
	apiKeyEnvName := environment.WithElasticPackagePrefix(ServerlessApiKeyEnvironmentVariable)
	apiKey := os.Getenv(apiKeyEnvName)
	if apiKey == "" {
		return nil, fmt.Errorf("unable to obtain value from %s environment variable", apiKeyEnvName)
	}
	c := &Client{
		host:   host,
		apiKey: apiKey,
	}
	for _, opt := range opts {
		opt(c)
	}

	return c, nil
}

// Address option sets the host to use to connect to Kibana.
func WithAddress(address string) ClientOption {
	return func(c *Client) {
		c.host = address
	}
}

// Address option sets the host to use to connect to Kibana.
func WithApiKey(apiKey string) ClientOption {
	return func(c *Client) {
		c.apiKey = apiKey
	}
}

func (c *Client) get(ctx context.Context, resourcePath string) (int, []byte, error) {
	return c.sendRequest(ctx, http.MethodGet, resourcePath, nil)
}

func (c *Client) post(ctx context.Context, resourcePath string, body []byte) (int, []byte, error) {
	return c.sendRequest(ctx, http.MethodPost, resourcePath, body)
}

func (c *Client) delete(ctx context.Context, resourcePath string) (int, []byte, error) {
	return c.sendRequest(ctx, http.MethodDelete, resourcePath, nil)
}

func (c *Client) sendRequest(ctx context.Context, method, resourcePath string, body []byte) (int, []byte, error) {
	request, err := c.newRequest(ctx, method, resourcePath, bytes.NewReader(body))
	if err != nil {
		return 0, nil, err
	}

	return c.doRequest(request)
}

func (c *Client) newRequest(ctx context.Context, method, resourcePath string, reqBody io.Reader) (*http.Request, error) {
	base, err := url.Parse(c.host)
	if err != nil {
		return nil, fmt.Errorf("could not create base URL from host: %v: %w", c.host, err)
	}

	rel, err := url.Parse(resourcePath)
	if err != nil {
		return nil, fmt.Errorf("could not create relative URL from resource path: %v: %w", resourcePath, err)
	}

	u := base.JoinPath(rel.EscapedPath())
	u.RawQuery = rel.RawQuery

	logger.Debugf("%s %s", method, u)

	req, err := http.NewRequestWithContext(ctx, method, u.String(), reqBody)
	if err != nil {
		return nil, fmt.Errorf("could not create %v request to Kibana API resource: %s: %w", method, resourcePath, err)
	}

	req.Header.Add("content-type", "application/json")
	req.Header.Add("Authorization", fmt.Sprintf("ApiKey %s", c.apiKey))

	return req, nil
}

func (c *Client) doRequest(request *http.Request) (int, []byte, error) {
	client := http.Client{}

	resp, err := client.Do(request)
	if err != nil {
		return 0, nil, fmt.Errorf("could not send request to Kibana API: %w", err)
	}

	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return resp.StatusCode, nil, fmt.Errorf("could not read response body: %w", err)
	}

	return resp.StatusCode, body, nil
}

func (c *Client) CreateProject(name, region, project string) (*Project, error) {
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
	ctx := context.Background()
	resourcePath := fmt.Sprintf("%s/api/v1/serverless/projects/%s", c.host, project)
	statusCode, respBody, err := c.post(ctx, resourcePath, p)

	if err != nil {
		return nil, fmt.Errorf("error creating project: %w", err)
	}

	if statusCode != http.StatusCreated {
		return nil, fmt.Errorf("unexpected status code %d, body: %s", statusCode, string(respBody))
	}

	serverlessProject := &Project{url: c.host, apiKey: c.apiKey}
	err = json.Unmarshal(respBody, &serverlessProject)

	bytes, _ := json.MarshalIndent(&serverlessProject, "", "  ")
	fmt.Printf("Project:\n%s", string(bytes))

	return serverlessProject, err
}

func (c *Client) DeleteProject(project *Project) error {
	ctx := context.Background()
	resourcePath := fmt.Sprintf("%s/api/v1/serverless/projects/%s/%s", c.host, project.Type, project.ID)
	statusCode, _, err := c.delete(ctx, resourcePath)
	if err != nil {
		return fmt.Errorf("error deleting project: %w", err)
	}

	if statusCode != http.StatusOK {
		return fmt.Errorf("unexpected status code %ds", statusCode)
	}

	return nil
}

func (c *Client) GetProject(projectType, projectID string) (*Project, error) {
	ctx := context.Background()
	resourcePath := fmt.Sprintf("%s/api/v1/serverless/projects/%s/%s", c.host, projectType, projectID)
	statusCode, respBody, err := c.get(ctx, resourcePath)
	if err != nil {
		return nil, fmt.Errorf("error deleting project: %w", err)
	}

	if statusCode == http.StatusNotFound {
		return nil, ErrProjectNotExist
	}

	if statusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code %d", statusCode)
	}

	project := &Project{url: c.host, apiKey: c.apiKey}
	err = json.Unmarshal(respBody, &project)
	return project, err
}
