// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package status

import (
	"fmt"
	"io"
	"net/http"

	"github.com/elastic/elastic-package/internal/logger"
	"github.com/elastic/go-ucfg"
	"github.com/elastic/go-ucfg/yaml"
)

var serverlessProjectTypes = []serverlessProjectType{
	{
		Name:      "observability",
		ConfigURL: "https://raw.githubusercontent.com/elastic/kibana/main/config/serverless.oblt.yml",
		FallbackCapabilities: []string{
			"apm",
			"observability",
		},
	},
	{
		Name:      "security",
		ConfigURL: "https://raw.githubusercontent.com/elastic/kibana/main/config/serverless.security.yml",
		FallbackCapabilities: []string{
			"security",
		},
	},
	{
		Name:                  "elasticsearch",
		ConfigURL:             "https://raw.githubusercontent.com/elastic/kibana/main/config/serverless.es.yml",
		FallbackCapabilities:  []string{},
		FallbackFleetDisabled: true,
	},
}

type ServerlessProjectType struct {
	Name         string
	Capabilities []string
}

func GetServerlessProjectTypes(client *http.Client) []ServerlessProjectType {
	// TODO: Add local cache to avoid too many requests to Github.
	var projectTypes []ServerlessProjectType
	for _, projectType := range serverlessProjectTypes {
		config, err := requestServerlessKibanaConfig(client, projectType.ConfigURL)
		if err != nil {
			logger.Debugf("failed to get serverless project type configuration from %q: %v", projectType.ConfigURL, err)
			if !projectType.FallbackFleetDisabled {
				projectTypes = append(projectTypes, ServerlessProjectType{
					Name:         projectType.Name,
					Capabilities: projectType.FallbackCapabilities,
				})
			}
			continue
		}

		if enabled := config.XPack.Fleet.Enabled; enabled != nil && !*enabled {
			continue
		}

		projectTypes = append(projectTypes, ServerlessProjectType{
			Name:         projectType.Name,
			Capabilities: config.XPack.Fleet.Internal.Registry.Capabilities,
		})
	}
	return projectTypes
}

type serverlessProjectType struct {
	Name                  string
	ConfigURL             string
	FallbackCapabilities  []string
	FallbackFleetDisabled bool
}

type kibanaConfig struct {
	XPack struct {
		Fleet struct {
			Enabled  *bool `config:"enabled"`
			Internal struct {
				Registry struct {
					Capabilities []string `config:"capabilities"`
					Spec         struct {
						Max string `config:"max"`
						Min string `config:"min"`
					} `config:"spec"`
				} `config:"registry"`
			} `config:"internal"`
		} `config:"fleet"`
	} `config:"xpack"`
}

func parseServerlessKibanaConfig(r io.Reader) (*kibanaConfig, error) {
	d, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}

	config, err := yaml.NewConfig(d, ucfg.PathSep("."))
	if err != nil {
		return nil, fmt.Errorf("failed to parse kibana configuration: %w", err)
	}

	var kibanaConfig kibanaConfig
	err = config.Unpack(&kibanaConfig, ucfg.PathSep("."))
	if err != nil {
		return nil, fmt.Errorf("failed to unpack kibana configuration: %w", err)
	}

	return &kibanaConfig, nil
}

func requestServerlessKibanaConfig(client *http.Client, configURL string) (*kibanaConfig, error) {
	resp, err := client.Get(configURL)
	if err != nil {
		return nil, fmt.Errorf("failed to get kibana configuration: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("received status code %d when requesting kibana configuration", resp.StatusCode)
	}

	return parseServerlessKibanaConfig(resp.Body)
}
