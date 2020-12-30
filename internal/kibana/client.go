// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package kibana

import (
	"bytes"
	"io/ioutil"
	"net/http"
	"os"

	"github.com/elastic/elastic-package/internal/logger"
	"github.com/elastic/elastic-package/internal/stack"
	"github.com/pkg/errors"
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

func (c *Client) get(resourcePath string) (int, []byte, error) {
	url := c.host + "/" + resourcePath
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
	url := c.host + "/" + resourcePath

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
