// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package ingestmanager

import (
	"bytes"
	"io/ioutil"
	"net/http"

	"github.com/pkg/errors"

	"github.com/elastic/elastic-package/internal/logger"
	"github.com/elastic/elastic-package/internal/stack"
)

// Client represents an Ingest Manager API client.
type Client struct {
	apiBaseURL string

	username string
	password string
}

// NewClient returns a new Ingest Manager API client.
func NewClient(baseURL, username, password string) (*Client, error) {
	return &Client{
		baseURL + "/api/fleet",
		username,
		password,
	}, nil
}

func (c *Client) get(resourcePath string) (int, []byte, error) {
	url := c.apiBaseURL + "/" + resourcePath
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return 0, nil, errors.Wrapf(err, "could not create GET request to Ingest Manager resource: %s", resourcePath)
	}

	req.SetBasicAuth(c.username, c.password)

	_, statusCode, respBody, err := sendRequest(req)
	if err != nil {
		return statusCode, respBody, errors.Wrapf(err, "error sending POST request to Ingest Manager resource: %s", resourcePath)
	}

	return statusCode, respBody, nil
}

func (c *Client) post(resourcePath string, body []byte) (int, []byte, error) {
	return c.putOrPost(http.MethodPost, resourcePath, body)
}

func (c *Client) put(resourcePath string, body []byte) (int, []byte, error) {
	return c.putOrPost(http.MethodPut, resourcePath, body)
}

func (c *Client) putOrPost(method, resourcePath string, body []byte) (int, []byte, error) {
	reqBody := bytes.NewReader(body)
	url := c.apiBaseURL + "/" + resourcePath

	logger.Debugf("%s %s", method, url)
	logger.Debugf("%s", body)

	req, err := http.NewRequest(method, url, reqBody)
	if err != nil {
		return 0, nil, errors.Wrapf(err, "could not create POST request to Ingest Manager resource: %s", resourcePath)
	}

	req.SetBasicAuth(c.username, c.password)
	req.Header.Add("content-type", "application/json")
	req.Header.Add("kbn-xsrf", stack.DefaultVersion)

	_, statusCode, respBody, err := sendRequest(req)
	if err != nil {
		return statusCode, respBody, errors.Wrapf(err, "error sending POST request to Ingest Manager resource: %s", resourcePath)
	}

	return statusCode, respBody, nil
}

func sendRequest(req *http.Request) (*http.Response, int, []byte, error) {
	client := http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, 0, nil, errors.Wrap(err, "could not send request to Kibana API")
	}

	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return resp, resp.StatusCode, nil, errors.Wrap(err, "could not read response body")
	}

	return resp, resp.StatusCode, body, nil
}
