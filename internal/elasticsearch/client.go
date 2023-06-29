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
	"os"
	"strings"

	"github.com/elastic/go-elasticsearch/v7"
	"github.com/elastic/go-elasticsearch/v7/esapi"

	"github.com/elastic/elastic-package/internal/certs"
	"github.com/elastic/elastic-package/internal/stack"
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

// defaultOptionsFromEnv returns clientOptions initialized with values from environmet variables.
func defaultOptionsFromEnv() clientOptions {
	return clientOptions{
		address:              os.Getenv(stack.ElasticsearchHostEnv),
		username:             os.Getenv(stack.ElasticsearchUsernameEnv),
		password:             os.Getenv(stack.ElasticsearchPasswordEnv),
		certificateAuthority: os.Getenv(stack.CACertificateEnv),
	}
}

type ClientOption func(*clientOptions)

// OptionWithAddress sets the address to be used by the client.
func OptionWithAddress(address string) ClientOption {
	return func(opts *clientOptions) {
		opts.address = address
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
	options := defaultOptionsFromEnv()
	for _, option := range customOptions {
		option(&options)
	}

	if options.address == "" {
		return nil, stack.UndefinedEnvError(stack.ElasticsearchHostEnv)
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
			return nil, fmt.Errorf("reading CA certificate: %w", err)
		}
		config.Transport = &http.Transport{
			TLSClientConfig: &tls.Config{RootCAs: rootCAs},
		}
	}

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
