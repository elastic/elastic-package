// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package fleetserver

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net/http"
	"net/url"

	"github.com/elastic/elastic-package/internal/certs"
	"github.com/elastic/elastic-package/internal/logger"
)

// Client is a client for Fleet Server API. This API only supports authentication with API
// keys, though some endpoints are also available without any authentication.
type Client struct {
	address string
	apiKey  string

	certificateAuthority string
	tlSkipVerify         bool

	http            *http.Client
	httpClientSetup func(*http.Client) *http.Client
}

type ClientOption func(*Client)

func NewClient(address string, opts ...ClientOption) (*Client, error) {
	client := Client{
		address: address,
	}

	for _, opt := range opts {
		opt(&client)
	}

	httpClient, err := client.httpClient()
	if err != nil {
		return nil, fmt.Errorf("cannot create HTTP client: %w", err)
	}
	client.http = httpClient
	return &client, nil
}

// APIKey option sets the API key to be used by the client for authentication.
func APIKey(apiKey string) ClientOption {
	return func(c *Client) {
		c.apiKey = apiKey
	}
}

// TLSSkipVerify option disables TLS verification.
func TLSSkipVerify() ClientOption {
	return func(c *Client) {
		c.tlSkipVerify = true
	}
}

// CertificateAuthority sets the certificate authority to be used by the client.
func CertificateAuthority(certificateAuthority string) ClientOption {
	return func(c *Client) {
		c.certificateAuthority = certificateAuthority
	}
}

// HTTPClientSetup adds an initializing function for the http client.
func HTTPClientSetup(setup func(*http.Client) *http.Client) ClientOption {
	return func(c *Client) {
		c.httpClientSetup = setup
	}
}

func (c *Client) httpClient() (*http.Client, error) {
	client := &http.Client{}
	if c.tlSkipVerify {
		client.Transport = &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		}
	} else if c.certificateAuthority != "" {
		rootCAs, err := certs.SystemPoolWithCACertificate(c.certificateAuthority)
		if err != nil {
			return nil, fmt.Errorf("reading CA certificate: %w", err)
		}
		client.Transport = &http.Transport{
			TLSClientConfig: &tls.Config{RootCAs: rootCAs},
		}
	}

	if c.httpClientSetup != nil {
		client = c.httpClientSetup(client)
	}

	return client, nil
}

func (c *Client) httpRequest(ctx context.Context, method, resourcePath string, reqBody io.Reader) (*http.Request, error) {
	base, err := url.Parse(c.address)
	if err != nil {
		return nil, fmt.Errorf("could not create base URL from host: %v: %w", c.address, err)
	}

	rel, err := url.Parse(resourcePath)
	if err != nil {
		return nil, fmt.Errorf("could not create relative URL from resource path: %v: %w", resourcePath, err)
	}

	u := base.JoinPath(rel.EscapedPath())
	u.RawQuery = rel.RawQuery

	logger.Tracef("%s %s", method, u)

	req, err := http.NewRequestWithContext(ctx, method, u.String(), reqBody)
	if err != nil {
		return nil, fmt.Errorf("could not create %v request to Fleet Server API resource: %s: %w", method, resourcePath, err)
	}

	if c.apiKey != "" {
		req.Header.Set("Authorization", "ApiKey "+c.apiKey)
	}

	return req, nil
}
