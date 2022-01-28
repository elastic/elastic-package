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

// ClientOptions are used to configure a client.
type ClientOptions struct {
	// SkipTLSVerify disables TLS validation.
	SkipTLSVerify bool
}

// Client method creates new instance of the Elasticsearch client.
func Client() (*elasticsearch.Client, error) {
	return ClientWithOptions(ClientOptions{})
}

// ClientWithOptions creates new instance of the Elasticsearch client with custom options.
func ClientWithOptions(options ClientOptions) (*elasticsearch.Client, error) {
	host := os.Getenv(stack.ElasticsearchHostEnv)
	if host == "" {
		return nil, stack.UndefinedEnvError(stack.ElasticsearchHostEnv)
	}

	username := os.Getenv(stack.ElasticsearchUsernameEnv)
	password := os.Getenv(stack.ElasticsearchPasswordEnv)

	config := elasticsearch.Config{
		Addresses: []string{host},
		Username:  username,
		Password:  password,
	}
	if options.SkipTLSVerify {
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
