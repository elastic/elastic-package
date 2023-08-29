// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package kibana

import (
	"bytes"
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
)

var ErrUndefinedHost = errors.New("missing kibana host")

// Client is responsible for exporting dashboards from Kibana.
type Client struct {
	host     string
	username string
	password string

	certificateAuthority string
	tlSkipVerify         bool

	versionInfo VersionInfo
	semver      *semver.Version
}

// ClientOption is functional option modifying Kibana client.
type ClientOption func(*Client)

// NewClient creates a new instance of the client.
func NewClient(opts ...ClientOption) (*Client, error) {
	c := &Client{}
	for _, opt := range opts {
		opt(c)
	}

	if c.host == "" {
		return nil, ErrUndefinedHost
	}

	v, err := c.requestVersion()
	if err != nil {
		return nil, fmt.Errorf("failed to get Kibana version: %w", err)
	}
	c.versionInfo = v

	c.semver, err = semver.NewVersion(c.versionInfo.Number)
	if err != nil {
		return nil, fmt.Errorf("failed to parse Kibana version (%s): %w", c.versionInfo.Number, err)
	}

	return c, nil
}

// Address option sets the host to use to connect to Kibana.
func Address(address string) ClientOption {
	return func(c *Client) {
		c.host = address
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

// CertificateAuthority sets the certificate authority to be used by the client.
func CertificateAuthority(certificateAuthority string) ClientOption {
	return func(c *Client) {
		c.certificateAuthority = certificateAuthority
	}
}

func (c *Client) get(resourcePath string) (int, []byte, error) {
	return c.sendRequest(http.MethodGet, resourcePath, nil)
}

func (c *Client) post(resourcePath string, body []byte) (int, []byte, error) {
	return c.sendRequest(http.MethodPost, resourcePath, body)
}

func (c *Client) put(resourcePath string, body []byte) (int, []byte, error) {
	return c.sendRequest(http.MethodPut, resourcePath, body)
}

func (c *Client) delete(resourcePath string) (int, []byte, error) {
	return c.sendRequest(http.MethodDelete, resourcePath, nil)
}

func (c *Client) sendRequest(method, resourcePath string, body []byte) (int, []byte, error) {
	request, err := c.newRequest(method, resourcePath, bytes.NewReader(body))
	if err != nil {
		return 0, nil, err
	}

	return c.doRequest(request)
}

func (c *Client) newRequest(method, resourcePath string, reqBody io.Reader) (*http.Request, error) {
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

	req, err := http.NewRequest(method, u.String(), reqBody)
	if err != nil {
		return nil, fmt.Errorf("could not create %v request to Kibana API resource: %s: %w", method, resourcePath, err)
	}

	req.SetBasicAuth(c.username, c.password)
	req.Header.Add("content-type", "application/json")
	req.Header.Add("kbn-xsrf", install.DefaultStackVersion)

	return req, nil
}

func (c *Client) doRequest(request *http.Request) (int, []byte, error) {
	client := http.Client{}
	if c.tlSkipVerify {
		client.Transport = &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		}
	} else if c.certificateAuthority != "" {
		rootCAs, err := certs.SystemPoolWithCACertificate(c.certificateAuthority)
		if err != nil {
			return 0, nil, fmt.Errorf("reading CA certificate: %w", err)
		}
		client.Transport = &http.Transport{
			TLSClientConfig: &tls.Config{RootCAs: rootCAs},
		}
	}

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
