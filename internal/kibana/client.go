// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package kibana

import (
	"bytes"
	"crypto/tls"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"

	"github.com/pkg/errors"

	"github.com/elastic/elastic-package/internal/certs"
	"github.com/elastic/elastic-package/internal/install"
	"github.com/elastic/elastic-package/internal/logger"
	"github.com/elastic/elastic-package/internal/stack"
)

const DefaultContentType = "application/json"

// Client is responsible for exporting dashboards from Kibana.
type Client struct {
	host     string
	username string
	password string

	certificateAuthority string
	tlSkipVerify         bool
}

// ClientOption is functional option modifying Kibana client.
type ClientOption func(*Client)

// NewClient creates a new instance of the client.
func NewClient(opts ...ClientOption) (*Client, error) {
	host := os.Getenv(stack.KibanaHostEnv)
	username := os.Getenv(stack.ElasticsearchUsernameEnv)
	password := os.Getenv(stack.ElasticsearchPasswordEnv)
	certificateAuthority := os.Getenv(stack.CACertificateEnv)

	c := &Client{
		host:                 host,
		username:             username,
		password:             password,
		certificateAuthority: certificateAuthority,
	}

	for _, opt := range opts {
		opt(c)
	}

	if c.host == "" {
		return nil, stack.UndefinedEnvError(stack.KibanaHostEnv)
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

// CertificateAuthority sets the certificate authority to be used by the client.
func CertificateAuthority(certificateAuthority string) ClientOption {
	return func(c *Client) {
		c.certificateAuthority = certificateAuthority
	}
}

func (c *Client) get(resourcePath string) (int, []byte, error) {
	return c.sendRequest(http.MethodGet, resourcePath, nil, map[string]string{"content-type": DefaultContentType})
}

func (c *Client) post(resourcePath string, body []byte) (int, []byte, error) {
	return c.sendRequest(http.MethodPost, resourcePath, body, map[string]string{"content-type": DefaultContentType})
}

func (c *Client) postFile(resourcePath, contentTypeHeader string, body []byte) (int, []byte, error) {
	return c.sendRequest(http.MethodPost, resourcePath, body, map[string]string{"content-type": contentTypeHeader})
}

func (c *Client) put(resourcePath string, body []byte) (int, []byte, error) {
	return c.sendRequest(http.MethodPut, resourcePath, body, map[string]string{"content-type": DefaultContentType})
}

func (c *Client) delete(resourcePath string) (int, []byte, error) {
	return c.sendRequest(http.MethodDelete, resourcePath, nil, map[string]string{"content-type": DefaultContentType})
}

func (c *Client) sendRequest(method, resourcePath string, body []byte, extraHeaders map[string]string) (int, []byte, error) {
	reqBody := bytes.NewReader(body)
	base, err := url.Parse(c.host)
	if err != nil {
		return 0, nil, errors.Wrapf(err, "could not create base URL from host: %v", c.host)
	}

	rel, err := url.Parse(resourcePath)
	if err != nil {
		return 0, nil, errors.Wrapf(err, "could not create relative URL from resource path: %v", resourcePath)
	}

	u := base.JoinPath(rel.EscapedPath())
	u.RawQuery = rel.RawQuery

	logger.Debugf("%s %s", method, u)
	logger.Debugf("Request %s", u.String())

	req, err := http.NewRequest(method, u.String(), reqBody)
	if err != nil {
		return 0, nil, errors.Wrapf(err, "could not create %v request to Kibana API resource: %s", method, resourcePath)
	}

	req.SetBasicAuth(c.username, c.password)
	req.Header.Add("kbn-xsrf", install.DefaultStackVersion)

	for k, v := range extraHeaders {
		req.Header.Add(k, v)
	}

	logger.Debugf("Headers %s", req.Header)
	logger.Debugf("ContentLength %s", req.ContentLength)

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

	resp, err := client.Do(req)
	if err != nil {
		return 0, nil, errors.Wrap(err, "could not send request to Kibana API")
	}

	defer resp.Body.Close()
	body, err = io.ReadAll(resp.Body)
	if err != nil {
		return resp.StatusCode, nil, errors.Wrap(err, "could not read response body")
	}

	return resp.StatusCode, body, nil
}
