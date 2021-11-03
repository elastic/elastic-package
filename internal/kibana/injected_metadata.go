// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package kibana

import (
	"encoding/json"
	"fmt"
	"html"
	"net/http"
	"regexp"

	"github.com/pkg/errors"
)

var kbnInjectedMetadataRegexp = regexp.MustCompile(`<kbn-injected-metadata data="(.+)"></kbn-injected-metadata>`)

// injectedMetadata represents the Kibana metadata structure exposed in the web UI.
type injectedMetadata struct {
	// Stack version
	Version string `json:"version"`
}

// Version method returns the Kibana version.
func (c *Client) Version() (string, error) {
	statusCode, respBody, err := c.get("/login")
	if err != nil {
		return "", errors.Wrap(err, "could not reach login endpoint")
	}
	if statusCode != http.StatusOK {
		return "", fmt.Errorf("could not reach login endpoint; API status code = %d; response body = %s", statusCode, string(respBody))
	}

	im, err := extractInjectedMetadata(respBody)
	if err != nil {
		return "", errors.Wrap(err, "can't extract injected metadata")
	}
	return im.Version, nil
}

func extractInjectedMetadata(body []byte) (*injectedMetadata, error) {
	rawInjectedMetadata, err := extractRawInjectedMetadata(body)
	if err != nil {
		return nil, errors.Wrap(err, "can't extract raw metadata")
	}

	var im injectedMetadata
	err = json.Unmarshal(rawInjectedMetadata, &im)
	if err != nil {
		return nil, errors.Wrap(err, "can't unmarshal raw injected metadata")
	}
	return &im, nil
}

func extractRawInjectedMetadata(body []byte) ([]byte, error) {
	matches := kbnInjectedMetadataRegexp.FindSubmatch(body)
	if len(matches) < 2 { // index:0 - matched regexp, index:1 - matched data
		return nil, errors.New("expected to find at least one <kbn-injected-metadata> tag")
	}
	return []byte(html.UnescapeString(string(matches[1]))), nil
}
