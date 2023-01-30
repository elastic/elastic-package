// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package integration_corpus_generator

import (
	"bytes"
	"io"
	"net/http"
	"net/url"

	"github.com/pkg/errors"

	"github.com/elastic/elastic-package/internal/logger"
)

const integrationCorpusGeneratorAssetsBaseURL = "https://raw.githubusercontent.com/elastic/elastic-integration-corpus-generator-tool/main/assets/templates/"

// Client is responsible for exporting assets from elastic-integration-corpus-generator-tool repo.
type Client struct {
}

// NewClient creates a new instance of the client.
func NewClient() *Client {
	return &Client{}
}

func (c *Client) get(resourcePath string) (int, []byte, error) {
	return c.sendRequest(http.MethodGet, resourcePath, nil)
}

func (c *Client) sendRequest(method, resourcePath string, body []byte) (int, []byte, error) {
	reqBody := bytes.NewReader(body)
	base, err := url.Parse(integrationCorpusGeneratorAssetsBaseURL)
	if err != nil {
		return 0, nil, errors.Wrapf(err, "could not create base URL from host: %v", integrationCorpusGeneratorAssetsBaseURL)
	}

	rel, err := url.Parse(resourcePath)
	if err != nil {
		return 0, nil, errors.Wrapf(err, "could not create relative URL from resource path: %v", resourcePath)
	}

	u := base.JoinPath(rel.EscapedPath())

	logger.Debugf("%s %s", method, u)

	req, err := http.NewRequest(method, u.String(), reqBody)
	if err != nil {
		return 0, nil, errors.Wrapf(err, "could not create %v request to elastic-integration-corpus-generator-tool repo: %s", method, resourcePath)
	}

	client := http.Client{}

	resp, err := client.Do(req)
	if err != nil {
		return 0, nil, errors.Wrap(err, "could not send request to elastic-integration-corpus-generator-tool repo")
	}

	if resp.Body == nil {
		return 0, nil, errors.Wrap(err, "could not get response from elastic-integration-corpus-generator-tool repo")
	}

	defer func() {
		_ = resp.Body.Close()
	}()

	body, err = io.ReadAll(resp.Body)
	if err != nil {
		return resp.StatusCode, nil, errors.Wrap(err, "could not read response body")
	}

	return resp.StatusCode, body, nil
}
