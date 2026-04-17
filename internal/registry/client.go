// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package registry

import (
	"crypto/tls"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"

	"github.com/elastic/elastic-package/internal/certs"
	"github.com/elastic/elastic-package/internal/logger"
)

const (
	ProductionURL = "https://epr.elastic.co"
)

// ClientOption is a functional option for the registry client.
type ClientOption func(*Client)

// Client is responsible for communicating with the Package Registry API.
type Client struct {
	baseURL              string
	certificateAuthority string
	tlsSkipVerify        bool
	httpClient           *http.Client
}

// NewClient creates a new instance of the client.
func NewClient(baseURL string, opts ...ClientOption) (*Client, error) {
	c := &Client{baseURL: baseURL}
	for _, opt := range opts {
		opt(c)
	}
	httpClient, err := c.newHTTPClient()
	if err != nil {
		return nil, fmt.Errorf("creating registry HTTP client: %w", err)
	}
	c.httpClient = httpClient
	return c, nil
}

// CertificateAuthority sets the certificate authority to use for TLS verification.
func CertificateAuthority(path string) ClientOption {
	return func(c *Client) {
		c.certificateAuthority = path
	}
}

// TLSSkipVerify disables TLS certificate verification (e.g. for local HTTPS registries).
func TLSSkipVerify() ClientOption {
	return func(c *Client) {
		c.tlsSkipVerify = true
	}
}

func (c *Client) newHTTPClient() (*http.Client, error) {
	client := &http.Client{}
	if c.tlsSkipVerify {
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
	return client, nil
}

func (c *Client) get(resourcePath string) (int, []byte, error) {
	base, err := url.Parse(c.baseURL)
	if err != nil {
		return 0, nil, fmt.Errorf("could not parse base URL: %v: %w", c.baseURL, err)
	}

	rel, err := url.Parse(resourcePath)
	if err != nil {
		return 0, nil, fmt.Errorf("could not create relative URL from resource path: %v: %w", resourcePath, err)
	}

	u := base.JoinPath(rel.EscapedPath())
	u.RawQuery = rel.RawQuery

	logger.Tracef("Sending request to Package Registry API: %s", u.String())

	req, err := http.NewRequest(http.MethodGet, u.String(), nil)
	if err != nil {
		return 0, nil, fmt.Errorf("could not create request to Package Registry API resource: %s: %w", resourcePath, err)
	}

	client := c.httpClient
	if client == nil {
		client = &http.Client{}
	}
	resp, err := client.Do(req)
	if err != nil {
		return 0, nil, fmt.Errorf("could not send request to Package Registry API: %w", err)
	}

	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return resp.StatusCode, nil, fmt.Errorf("could not read response body: %w", err)
	}

	return resp.StatusCode, body, nil
}

// DownloadPackage downloads a package zip from the registry and writes it to destDir.
// It returns the path to the downloaded zip file.
func (c *Client) DownloadPackage(name, version, destDir string) (string, error) {
	resourcePath := fmt.Sprintf("/epr/%s/%s-%s.zip", name, name, version)
	statusCode, body, err := c.get(resourcePath)
	if err != nil {
		return "", fmt.Errorf("downloading package %s-%s: %w", name, version, err)
	}
	if statusCode != http.StatusOK {
		return "", fmt.Errorf("downloading package %s-%s: unexpected status code %d", name, version, statusCode)
	}

	zipPath := filepath.Join(destDir, fmt.Sprintf("%s-%s.zip", name, version))
	shouldRemove := false
	defer func() {
		if shouldRemove {
			_ = os.Remove(zipPath)
		}
	}()

	shouldRemove = true
	if err := os.WriteFile(zipPath, body, 0o644); err != nil {
		return "", fmt.Errorf("writing package zip to %s: %w", zipPath, err)
	}

	shouldRemove = false
	return zipPath, nil
}
