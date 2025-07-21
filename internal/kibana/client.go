// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package kibana

import (
	"bytes"
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"

	"github.com/Masterminds/semver/v3"

	"github.com/elastic/elastic-package/internal/certs"
	"github.com/elastic/elastic-package/internal/install"
	"github.com/elastic/elastic-package/internal/logger"
	"github.com/elastic/elastic-package/internal/retry"
)

var (
	ErrUndefinedHost = errors.New("missing kibana host")
	ErrConflict      = errors.New("resource already exists")
)

// Client is responsible for exporting dashboards from Kibana.
type Client struct {
	host     string
	apiKey   string
	username string
	password string

	certificateAuthority string
	tlSkipVerify         bool

	versionInfo VersionInfo
	semver      *semver.Version

	retryMax        int
	http            *http.Client
	httpClientSetup func(*http.Client) *http.Client
}

// ClientOption is functional option modifying Kibana client.
type ClientOption func(*Client)

// NewClient creates a new instance of the client.
func NewClient(opts ...ClientOption) (*Client, error) {
	c := &Client{
		retryMax: 10,
	}
	for _, opt := range opts {
		opt(c)
	}

	if c.host == "" {
		return nil, ErrUndefinedHost
	}

	httpClient, err := c.newHttpClient()
	if err != nil {
		return nil, err
	}
	c.http = httpClient

	// Allow to initialize version from tests.
	var zeroVersion VersionInfo
	if c.semver == nil || c.versionInfo == zeroVersion {
		// Passing a nil context here because we are on initialization.
		v, err := c.requestStatus(context.Background())
		if err != nil {
			return nil, fmt.Errorf("failed to get Kibana version: %w", err)
		}
		c.versionInfo = v.Version

		// Version info may not contain any version if this is a managed Kibana.
		if c.versionInfo.Number != "" {
			c.semver, err = semver.NewVersion(c.versionInfo.Number)
			if err != nil {
				return nil, fmt.Errorf("failed to parse Kibana version (%s): %w", c.versionInfo.Number, err)
			}
		}
	}

	return c, nil
}

// Get client host
func (c *Client) Address() string {
	return c.host
}

// Address option sets the host to use to connect to Kibana.
func Address(address string) ClientOption {
	return func(c *Client) {
		c.host = address
	}
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

// Username option sets the username to be used by the client.
func Username(username string) ClientOption {
	return func(c *Client) {
		c.username = username
	}
}

// Password option sets the password to be used by the client.
func Password(password string) ClientOption {
	return func(c *Client) {
		c.password = password
	}
}

// RetryMax configures the number of retries before failing.
func RetryMax(retryMax int) ClientOption {
	return func(c *Client) {
		c.retryMax = retryMax
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

func (c *Client) get(ctx context.Context, resourcePath string) (int, []byte, error) {
	return c.SendRequest(ctx, http.MethodGet, resourcePath, nil)
}

func (c *Client) post(ctx context.Context, resourcePath string, body []byte) (int, []byte, error) {
	return c.SendRequest(ctx, http.MethodPost, resourcePath, body)
}

func (c *Client) put(ctx context.Context, resourcePath string, body []byte) (int, []byte, error) {
	return c.SendRequest(ctx, http.MethodPut, resourcePath, body)
}

func (c *Client) delete(ctx context.Context, resourcePath string) (int, []byte, error) {
	return c.SendRequest(ctx, http.MethodDelete, resourcePath, nil)
}

func (c *Client) SendRequest(ctx context.Context, method, resourcePath string, body []byte) (int, []byte, error) {
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

	logger.Tracef("%s %s", method, u)

	req, err := http.NewRequestWithContext(ctx, method, u.String(), reqBody)
	if err != nil {
		return nil, fmt.Errorf("could not create %v request to Kibana API resource: %s: %w", method, resourcePath, err)
	}

	if c.apiKey != "" {
		req.Header.Set("Authorization", "ApiKey "+c.apiKey)
	} else {
		req.SetBasicAuth(c.username, c.password)
	}
	req.Header.Add("content-type", "application/json")
	req.Header.Add("kbn-xsrf", install.DefaultStackVersion)

	return req, nil
}

func (c *Client) doRequest(request *http.Request) (int, []byte, error) {
	resp, err := c.http.Do(request)
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

func (c *Client) newHttpClient() (*http.Client, error) {
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

	if c.retryMax > 0 {
		opts := retry.HTTPOptions{
			RetryMax: c.retryMax,
		}
		client = retry.WrapHTTPClient(client, opts)
	}

	if c.httpClientSetup != nil {
		client = c.httpClientSetup(client)
	}

	return client, nil
}
