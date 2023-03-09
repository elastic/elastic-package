// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package corpusgenerator

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"

	"github.com/elastic/elastic-integration-corpus-generator-tool/pkg/genlib"
	"github.com/elastic/elastic-integration-corpus-generator-tool/pkg/genlib/config"
	"github.com/elastic/elastic-integration-corpus-generator-tool/pkg/genlib/fields"
	"github.com/pkg/errors"

	"github.com/elastic/elastic-package/internal/builder"
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
		return nil, errors.Wrap(err, "could not get gotext template")
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

		return genlib.Config{}, errors.Wrap(err, "could not get config yaml")
	}

	if statusCode != http.StatusOK {
		return genlib.Config{}, fmt.Errorf("could not get config yaml; API status code = %d; response body = %s", statusCode, respBody)
	}

	cfg, err := config.LoadConfigFromYaml(respBody)
	if err != nil {
		return genlib.Config{}, errors.Wrap(err, "could not load config yaml")
	}

	return cfg, nil
}

// GetFields returns the genlib.Config of a package's data stream
func (c *Client) GetFields(packageName, dataStreamName string) (genlib.Fields, error) {
	assetsSubFolder := fmt.Sprintf("%s.%s", packageName, dataStreamName)

	statusCode, respBody, err := c.get(fmt.Sprintf("%s/%s", assetsSubFolder, fieldsYamlAssetName))
	if err != nil {
		return genlib.Fields{}, errors.Wrap(err, "could not get fields yaml")
	}

	if statusCode != http.StatusOK {
		return genlib.Fields{}, fmt.Errorf("could not get config yaml; API status code = %d; response body = %s", statusCode, respBody)
	}

	fieldsDefinitionPath, err := writeFieldsYamlFile(respBody, packageName, dataStreamName)
	if err != nil {
		return genlib.Fields{}, errors.Wrap(err, "could not load fields yaml")
	}

	ctx := context.Background()
	fields, err := fields.LoadFieldsWithTemplate(ctx, fieldsDefinitionPath)
	if err != nil {
		return genlib.Fields{}, errors.Wrap(err, "could not load fields yaml")
	}

	return fields, nil
}

func tmpGenlibDir() (string, error) {
	buildDir, err := builder.BuildDirectory()
	if err != nil {
		return "", errors.Wrap(err, "locating build directory failed")
	}
	return filepath.Join(buildDir, "genlib"), nil
}

func writeFieldsYamlFile(fieldsYamlContent []byte, packageName, dataStreamName string) (string, error) {
	dest, err := tmpGenlibDir()
	if err != nil {
		return "", errors.Wrap(err, "could not determine genlib temp folder")
	}

	// Create genlib temp folder folder if it doesn't exist
	_, err = os.Stat(dest)
	if err != nil && errors.Is(err, os.ErrNotExist) {
		if err := os.MkdirAll(dest, 0755); err != nil {
			return "", errors.Wrap(err, "could not create genlib temp folder")
		}
	}

	fileName := fmt.Sprintf("%s-%s", packageName, dataStreamName)
	filePath := filepath.Join(dest, fileName)

	if err := os.WriteFile(filePath, fieldsYamlContent, 0644); err != nil {
		return "", errors.Wrap(err, "could not write genlib field yaml temp file")
	}

	return filePath, nil
}
