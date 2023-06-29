// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package corpusgenerator

import (
	"context"
	"fmt"
	"net/http"

	"github.com/elastic/elastic-integration-corpus-generator-tool/pkg/genlib"
	"github.com/elastic/elastic-integration-corpus-generator-tool/pkg/genlib/config"
	"github.com/elastic/elastic-integration-corpus-generator-tool/pkg/genlib/fields"
)

const (
	confYamlAssetName       = "schema-b/configs.yml"
	fieldsYamlAssetName     = "schema-b/fields.yml"
	gotextTemplateAssetName = "schema-b/gotext.tpl"
)

// GetGoTextTemplate returns the gotext template of a package's data stream
func (c *Client) GetGoTextTemplate(packageName, dataStreamName string) ([]byte, error) {
	assetsSubFolder := fmt.Sprintf("%s.%s", packageName, dataStreamName)
	statusCode, respBody, err := c.get(fmt.Sprintf("%s/%s", assetsSubFolder, gotextTemplateAssetName))
	if err != nil {
		return nil, fmt.Errorf("could not get gotext template: %w", err)
	}

	if statusCode != http.StatusOK {
		return nil, fmt.Errorf("could not get gotext template; API status code = %d; response body = %s", statusCode, respBody)
	}

	return respBody, nil
}

// GetConf returns the genlib.Config of a package's data stream
func (c *Client) GetConf(packageName, dataStreamName string) (genlib.Config, error) {
	assetsSubFolder := fmt.Sprintf("%s.%s", packageName, dataStreamName)

	statusCode, respBody, err := c.get(fmt.Sprintf("%s/%s", assetsSubFolder, confYamlAssetName))
	if err != nil {

		return genlib.Config{}, fmt.Errorf("could not get config yaml: %w", err)
	}

	if statusCode != http.StatusOK {
		return genlib.Config{}, fmt.Errorf("could not get config yaml; API status code = %d; response body = %s", statusCode, respBody)
	}

	cfg, err := config.LoadConfigFromYaml(respBody)
	if err != nil {
		return genlib.Config{}, fmt.Errorf("could not load config yaml: %w", err)
	}

	return cfg, nil
}

// GetFields returns the genlib.Config of a package's data stream
func (c *Client) GetFields(packageName, dataStreamName string) (genlib.Fields, error) {
	assetsSubFolder := fmt.Sprintf("%s.%s", packageName, dataStreamName)

	statusCode, respBody, err := c.get(fmt.Sprintf("%s/%s", assetsSubFolder, fieldsYamlAssetName))
	if err != nil {
		return genlib.Fields{}, fmt.Errorf("could not get fields yaml: %w", err)
	}

	if statusCode != http.StatusOK {
		return genlib.Fields{}, fmt.Errorf("could not get fields yaml; API status code = %d; response body = %s", statusCode, respBody)
	}

	ctx := context.Background()
	fields, err := fields.LoadFieldsWithTemplateFromString(ctx, string(respBody))
	if err != nil {
		return genlib.Fields{}, fmt.Errorf("could not load fields yaml: %w", err)
	}

	return fields, nil
}
