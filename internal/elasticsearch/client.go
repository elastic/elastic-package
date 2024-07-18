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
	username string
	password string

	// certificateAuthority is the certificate to validate the server certificate.
	certificateAuthority string

	// skipTLSVerify disables TLS validation.
	skipTLSVerify bool
}

type ClientOption func(*clientOptions)

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

// CheckFailureStore checks if the failure store is available.
func (client *Client) CheckFailureStore(ctx context.Context) (bool, error) {
	// FIXME: Using the low-level transport till the API SDK supports the failure store.
	request, err := http.NewRequest(http.MethodGet, fmt.Sprintf("/_search?failure_store=only"), nil)
	if err != nil {
		return false, fmt.Errorf("failed to create search request: %w", err)
	}
	request.Header.Set("Content-Type", "application/json")

	resp, err := client.Transport.Perform(request)
	if err != nil {
		return false, fmt.Errorf("failed to perform search request: %w", err)
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusOK:
		return true, nil
	case http.StatusBadRequest:
		// Error expected when using an unrecognized parameter.
		return false, nil
	default:
		return false, fmt.Errorf("unexpected status code received: %d", resp.StatusCode)
	}
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
