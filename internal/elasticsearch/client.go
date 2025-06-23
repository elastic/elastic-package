// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package elasticsearch

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/elastic/go-elasticsearch/v7"
	"github.com/elastic/go-elasticsearch/v7/esapi"

	"github.com/elastic/elastic-package/internal/certs"
)

// API contains the elasticsearch APIs
type API = esapi.API

// IngestSimulateRequest configures the Ingest Simulate API request.
type IngestSimulateRequest = esapi.IngestSimulateRequest

// IngestGetPipelineRequest configures the Ingest Get Pipeline API request.
type IngestGetPipelineRequest = esapi.IngestGetPipelineRequest

// ClusterStateRequest configures the Cluster State API request.
type ClusterStateRequest = esapi.ClusterStateRequest

// clientOptions are used to configure a client.
type clientOptions struct {
	address  string
	apiKey   string
	username string
	password string

	// certificateAuthority is the certificate to validate the server certificate.
	certificateAuthority string

	// skipTLSVerify disables TLS validation.
	skipTLSVerify bool
}

type ClientOption func(*clientOptions)

// OptionWithAPIKey sets the API key to be used by the client for authentication.
func OptionWithAPIKey(apiKey string) ClientOption {
	return func(opts *clientOptions) {
		opts.apiKey = apiKey
	}
}

// OptionWithAddress sets the address to be used by the client.
func OptionWithAddress(address string) ClientOption {
	return func(opts *clientOptions) {
		opts.address = address
	}
}

// OptionWithUsername sets the username to be used by the client.
func OptionWithUsername(username string) ClientOption {
	return func(opts *clientOptions) {
		opts.username = username
	}
}

// OptionWithPassword sets the password to be used by the client.
func OptionWithPassword(password string) ClientOption {
	return func(opts *clientOptions) {
		opts.password = password
	}
}

// OptionWithCertificateAuthority sets the certificate authority to be used by the client.
func OptionWithCertificateAuthority(certificateAuthority string) ClientOption {
	return func(opts *clientOptions) {
		opts.certificateAuthority = certificateAuthority
	}
}

// OptionWithSkipTLSVerify disables TLS validation.
func OptionWithSkipTLSVerify() ClientOption {
	return func(opts *clientOptions) {
		opts.skipTLSVerify = true
	}
}

// Client is a wrapper over an Elasticsearch Client.
type Client struct {
	*elasticsearch.Client
}

// NewClient method creates new instance of the Elasticsearch client.
func NewClient(customOptions ...ClientOption) (*Client, error) {
	config, err := NewConfig(customOptions...)
	if err != nil {
		return nil, err
	}

	return NewClientWithConfig(config)
}

func NewConfig(customOptions ...ClientOption) (elasticsearch.Config, error) {
	options := clientOptions{}
	for _, option := range customOptions {
		option(&options)
	}

	if options.address == "" {
		return elasticsearch.Config{}, ErrUndefinedAddress
	}

	config := elasticsearch.Config{
		Addresses: []string{options.address},
		APIKey:    options.apiKey,
		Username:  options.username,
		Password:  options.password,
	}
	if options.skipTLSVerify {
		config.Transport = &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		}
	} else if options.certificateAuthority != "" {
		rootCAs, err := certs.SystemPoolWithCACertificate(options.certificateAuthority)
		if err != nil {
			return config, fmt.Errorf("reading CA certificate: %w", err)
		}
		config.Transport = &http.Transport{
			TLSClientConfig: &tls.Config{RootCAs: rootCAs},
		}
	}

	return config, nil
}

func NewClientWithConfig(config elasticsearch.Config) (*Client, error) {
	client, err := elasticsearch.NewClient(config)
	if err != nil {
		return nil, fmt.Errorf("can't create instance: %w", err)
	}
	return &Client{Client: client}, nil
}

// CheckHealth checks the health of the cluster.
func (client *Client) CheckHealth(ctx context.Context) error {
	resp, err := client.Cluster.Health(client.Cluster.Health.WithContext(ctx))
	if err != nil {
		return fmt.Errorf("error checking cluster health: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusGone {
		// We are in a managed deployment, API not available, assume healthy.
		return nil
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to check cluster health: %s", resp.String())
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("error reading cluster health response: %w", err)
	}

	var clusterHealth struct {
		Status string `json:"status"`
	}
	err = json.Unmarshal(body, &clusterHealth)
	if err != nil {
		return fmt.Errorf("error decoding cluster health response: %w", err)
	}

	if status := clusterHealth.Status; status != "green" && status != "yellow" {
		if status != "red" {
			return fmt.Errorf("cluster in unhealthy state: %q", status)
		}
		cause, err := client.redHealthCause(ctx)
		if err != nil {
			return fmt.Errorf("cluster in unhealthy state, failed to identify cause: %w", err)
		}
		return fmt.Errorf("cluster in unhealthy state: %s", cause)
	}

	return nil
}

type Info struct {
	Name        string `json:"name"`
	ClusterName string `json:"cluster_name"`
	ClusterUUID string `json:"cluster_uuid"`
	Version     struct {
		Number      string `json:"number"`
		BuildFlavor string `json:"build_flavor"`
	} `json:"version"`
}

// Info gets cluster information and metadata.
func (client *Client) Info(ctx context.Context) (*Info, error) {
	resp, err := client.Client.Info(client.Client.Info.WithContext(ctx))
	if err != nil {
		return nil, fmt.Errorf("error getting cluster info: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to get cluster info: %s", resp.String())
	}

	var info Info
	err = json.NewDecoder(resp.Body).Decode(&info)
	if err != nil {
		return nil, fmt.Errorf("error decoding cluster info: %w", err)
	}

	return &info, nil
}

// redHealthCause tries to identify the cause of a cluster in red state. This could be
// also used as a replacement of CheckHealth, but keeping them separated because it uses
// internal undocumented APIs that might change.
func (client *Client) redHealthCause(ctx context.Context) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "/_internal/_health", nil)
	if err != nil {
		return "", fmt.Errorf("error creating internal health request: %w", err)
	}
	resp, err := client.Transport.Perform(req)
	if err != nil {
		return "", fmt.Errorf("error performing internal health request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("error reading internal health response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("failed to get cause of red health; API status code = %d; response body = %s", resp.StatusCode, string(body))
	}

	var internalHealth struct {
		Status     string `json:"status"`
		Indicators map[string]struct {
			Status  string `json:"status"`
			Impacts []struct {
				Severity int `json:"severity"`
			} `json:"impacts"`
			Diagnosis []struct {
				Cause string `json:"cause"`
			} `json:"diagnosis"`
		} `json:"indicators"`
	}
	err = json.Unmarshal(body, &internalHealth)
	if err != nil {
		return "", fmt.Errorf("error decoding internal health response: %w", err)
	}
	if internalHealth.Status != "red" {
		return "", errors.New("cluster state is not red?")
	}

	// Only diagnostics with the highest severity impacts are returned.
	var highestSeverity int
	var causes []string
	for _, indicator := range internalHealth.Indicators {
		if indicator.Status != "red" {
			continue
		}

		var severity int
		for _, impact := range indicator.Impacts {
			if impact.Severity > severity {
				severity = impact.Severity
			}
		}

		switch {
		case severity < highestSeverity:
			continue
		case severity > highestSeverity:
			highestSeverity = severity
			causes = nil
		case severity == highestSeverity:
			// Continue appending for current severity.
		}

		for _, diagnosis := range indicator.Diagnosis {
			causes = append(causes, diagnosis.Cause)
		}
	}
	if len(causes) == 0 {
		return "", errors.New("no causes found")
	}
	return strings.Join(causes, ", "), nil
}

type Mappings struct {
	Properties       json.RawMessage `json:"properties"`
	DynamicTemplates json.RawMessage `json:"dynamic_templates"`
}

func (c *Client) SimulateIndexTemplate(ctx context.Context, indexTemplateName string) (*Mappings, error) {
	resp, err := c.Indices.SimulateTemplate(
		c.Indices.SimulateTemplate.WithContext(ctx),
		c.Indices.SimulateTemplate.WithName(indexTemplateName),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get field mapping for data stream %q: %w", indexTemplateName, err)
	}
	defer resp.Body.Close()
	if resp.IsError() {
		return nil, fmt.Errorf("error getting mapping: %s", resp)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading mapping body: %w", err)
	}

	type indexTemplateSimulated struct {
		// Settings json.RawMessage       `json:"settings"`
		Mappings Mappings `json:"mappings"`
	}

	type previewTemplate struct {
		Template indexTemplateSimulated `json:"template"`
	}

	var preview previewTemplate

	if err := json.Unmarshal(body, &preview); err != nil {
		return nil, fmt.Errorf("error unmarshaling mappings: %w", err)
	}

	// In case there are no dynamic templates, set an empty array
	if string(preview.Template.Mappings.DynamicTemplates) == "" {
		preview.Template.Mappings.DynamicTemplates = []byte("[]")
	}

	// In case there are no mappings defined, set an empty map
	if string(preview.Template.Mappings.Properties) == "" {
		preview.Template.Mappings.Properties = []byte("{}")
	}

	return &preview.Template.Mappings, nil
}

func (c *Client) DataStreamMappings(ctx context.Context, dataStreamName string) (*Mappings, error) {
	mappingResp, err := c.Indices.GetMapping(
		c.Indices.GetMapping.WithContext(ctx),
		c.Indices.GetMapping.WithIndex(dataStreamName),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get field mapping for data stream %q: %w", dataStreamName, err)
	}
	defer mappingResp.Body.Close()
	if mappingResp.IsError() {
		return nil, fmt.Errorf("error getting mapping: %s", mappingResp)
	}
	body, err := io.ReadAll(mappingResp.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading mapping body: %w", err)
	}

	mappingsRaw := map[string]struct {
		Mappings Mappings `json:"mappings"`
	}{}

	if err := json.Unmarshal(body, &mappingsRaw); err != nil {
		return nil, fmt.Errorf("error unmarshaling mappings: %w", err)
	}

	if len(mappingsRaw) != 1 {
		return nil, fmt.Errorf("exactly 1 mapping was expected, got %d", len(mappingsRaw))
	}

	var mappingsDefinition Mappings
	for _, v := range mappingsRaw {
		mappingsDefinition = v.Mappings
	}

	// In case there are no dynamic templates, set an empty array
	if string(mappingsDefinition.DynamicTemplates) == "" {
		mappingsDefinition.DynamicTemplates = []byte("[]")
	}

	// In case there are no mappings defined, set an empty map
	if string(mappingsDefinition.Properties) == "" {
		mappingsDefinition.Properties = []byte("{}")
	}

	return &mappingsDefinition, nil
}
