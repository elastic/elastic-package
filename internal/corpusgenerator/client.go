// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package corpusgenerator

import (
	"bytes"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/elastic/elastic-integration-corpus-generator-tool/pkg/genlib"
	"github.com/pkg/errors"

	"github.com/elastic/elastic-package/internal/logger"
)

// TODO: fetching artifacts from the corpus generator repo is a temporary solution in place
// before we will have the relevant assets content in package-spec.
// We still give the option to fetch from a specific commit
const integrationCorpusGeneratorAssetsBaseURL = "https://raw.githubusercontent.com/elastic/elastic-integration-corpus-generator-tool/%COMMIT%/assets/templates/"
const commitPlaceholder = "%COMMIT%"

// Client is responsible for exporting assets from elastic-integration-corpus-generator-tool repo.
type Client struct {
	commit string
}

// GenLibClient is an interface for the genlib client
type GenLibClient interface {
	GetGoTextTemplate(packageName, dataStreamName string) ([]byte, error)
	GetConf(packageName, dataStreamName string) (genlib.Config, error)
	GetFields(packageName, dataStreamName string) (genlib.Fields, error)
}

// NewClient creates a new instance of the client.
func NewClient(commit string) GenLibClient {
	return &Client{commit: commit}
}

func (c *Client) get(resourcePath string) (int, []byte, error) {
	return c.sendRequest(http.MethodGet, resourcePath, nil)
}

func (c *Client) sendRequest(method, resourcePath string, body []byte) (int, []byte, error) {
	reqBody := bytes.NewReader(body)
	commitAssetsBaseURL := strings.Replace(integrationCorpusGeneratorAssetsBaseURL, commitPlaceholder, c.commit, -1)
	base, err := url.Parse(commitAssetsBaseURL)
	if err != nil {
		return 0, nil, errors.Wrapf(err, "could not create base URL from commit: %v", c.commit)
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
