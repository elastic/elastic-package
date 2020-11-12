// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package kibana

import (
	"os"

	"github.com/elastic/elastic-package/internal/stack"
)

// Client is responsible for exporting dashboards from Kibana.
type Client struct {
	host     string
	username string
	password string
}

// NewClient creates a new instance of the client.
func NewClient() (*Client, error) {
	host := os.Getenv(stack.KibanaHostEnv)
	if host == "" {
		return nil, stack.UndefinedEnvError(stack.KibanaHostEnv)
	}

	username := os.Getenv(stack.ElasticsearchUsernameEnv)
	password := os.Getenv(stack.ElasticsearchPasswordEnv)

	return &Client{
		host:     host,
		username: username,
		password: password,
	}, nil
}
