// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package elasticsearch

import (
	"crypto/tls"
	"net/http"
	"os"

	"github.com/pkg/errors"

	"github.com/elastic/go-elasticsearch/v7"
	"github.com/elastic/go-elasticsearch/v7/esapi"

	"github.com/elastic/elastic-package/internal/stack"
)

// API contains the elasticsearch APIs
type API = esapi.API

// IngestSimulateRequest configures the Ingest Simulate API request.
type IngestSimulateRequest = esapi.IngestSimulateRequest

// IngestGetPipelineRequest configures the Ingest Get Pipeline API request.
type IngestGetPipelineRequest = esapi.IngestGetPipelineRequest

// clientOptions are used to configure a client.
type clientOptions struct {
	address  string
	username string
	password string

	// skipTLSVerify disables TLS validation.
	skipTLSVerify bool
}

// defaultOptionsFromEnv returns clientOptions initialized with values from environmet variables.
func defaultOptionsFromEnv() clientOptions {
	return clientOptions{
		address:  os.Getenv(stack.ElasticsearchHostEnv),
		username: os.Getenv(stack.ElasticsearchUsernameEnv),
		password: os.Getenv(stack.ElasticsearchPasswordEnv),
	}
}

type ClientOption func(*clientOptions)

// OptionWithAddress sets the address to be used by the client.
func OptionWithAddress(address string) ClientOption {
	return func(opts *clientOptions) {
		opts.address = address
	}
}

// OptionWithSkipTLSVerify disables TLS validation.
func OptionWithSkipTLSVerify() ClientOption {
	return func(opts *clientOptions) {
		opts.skipTLSVerify = true
	}
}

// Client method creates new instance of the Elasticsearch client.
func Client(customOptions ...ClientOption) (*elasticsearch.Client, error) {
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
	}

	client, err := elasticsearch.NewClient(config)
	if err != nil {
		return nil, errors.Wrap(err, "can't create instance")
	}
	return client, nil
}
