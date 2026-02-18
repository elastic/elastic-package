// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package status

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/elastic/go-ucfg"
	"github.com/elastic/go-ucfg/yaml"

	"github.com/elastic/elastic-package/internal/configuration/locations"
	"github.com/elastic/elastic-package/internal/logger"
)

func getServerlessProjectTypes(kibanaRepoBaseURL string) []serverlessProjectType {
	return []serverlessProjectType{
		{
			Name:      "observability",
			ConfigURL: fmt.Sprintf("%s/main/config/serverless.oblt.yml", kibanaRepoBaseURL),
			Fallback: serverlessProjectTypeFallback{
				Capabilities: []string{
					"apm",
					"observability",
				},
				SpecMin: "3.0",
				SpecMax: "3.0",
			},
		},
		{
			Name:      "security",
			ConfigURL: fmt.Sprintf("%s/main/config/serverless.security.yml", kibanaRepoBaseURL),
			Fallback: serverlessProjectTypeFallback{
				Capabilities: []string{
					"security",
				},
				SpecMin: "3.0",
				SpecMax: "3.0",
			},
		},
		{
			Name:      "elasticsearch",
			ConfigURL: fmt.Sprintf("%s/main/config/serverless.es.yml", kibanaRepoBaseURL),
			Fallback: serverlessProjectTypeFallback{
				FleetDisabled: true,
				Capabilities:  []string{},
			},
		},
	}
}

type ServerlessProjectType struct {
	Name            string
	Capabilities    []string
	ExcludePackages []string
	SpecMax         string
	SpecMin         string
}

func GetServerlessProjectTypes(client *http.Client, kibanaRepoBaseURL string) []ServerlessProjectType {
	cache, valid, err := loadCachedServerlessProjectTypes()
	if err == nil && valid {
		return cache
	}
	if err != nil {
		logger.Debugf("failed to load serverless config cache: %v", err)
	}

	serverlessProjectTypes := getServerlessProjectTypes(kibanaRepoBaseURL)
	var projectTypes []ServerlessProjectType
	for _, projectType := range serverlessProjectTypes {
		config, err := requestServerlessKibanaConfig(client, projectType.ConfigURL)
		if err != nil {
			logger.Debugf("failed to get serverless project type configuration from %q: %v", projectType.ConfigURL, err)
			if !projectType.Fallback.FleetDisabled {
				projectTypes = append(projectTypes, ServerlessProjectType{
					Name:         projectType.Name,
					Capabilities: projectType.Fallback.Capabilities,
					SpecMax:      projectType.Fallback.SpecMax,
					SpecMin:      projectType.Fallback.SpecMin,
				})
			}
			continue
		}

		if enabled := config.XPack.Fleet.Enabled; enabled != nil && !*enabled {
			continue
		}

		projectTypes = append(projectTypes, ServerlessProjectType{
			Name:            projectType.Name,
			Capabilities:    config.XPack.Fleet.Internal.Registry.Capabilities,
			ExcludePackages: config.XPack.Fleet.Internal.Registry.ExcludePackages,
			SpecMax:         config.XPack.Fleet.Internal.Registry.Spec.Max,
			SpecMin:         config.XPack.Fleet.Internal.Registry.Spec.Min,
		})
	}

	if len(projectTypes) > 0 {
		err := saveCachedServerlessProjectTypes(projectTypes)
		if err != nil {
			logger.Debugf("failed to save serverless config cache: %v", err)
		}
	}

	return projectTypes
}

type serverlessProjectType struct {
	Name      string
	ConfigURL string
	Fallback  serverlessProjectTypeFallback
}

type serverlessProjectTypeFallback struct {
	FleetDisabled bool
	Capabilities  []string
	SpecMax       string
	SpecMin       string
}

type kibanaConfig struct {
	XPack struct {
		Fleet struct {
			Enabled  *bool `config:"enabled"`
			Internal struct {
				Registry struct {
					Capabilities    []string `config:"capabilities"`
					ExcludePackages []string `config:"excludePackages"`
					Spec            struct {
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
	logger.Tracef("Request Serverless Kibana configuration from %s", configURL)
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

const (
	cachedServerlessProjectTypesFileName        = "serverless.json"
	cachedServerlessProjectTypesCacheExpiration = 1 * time.Hour
)

type cachedServerlessProjectTypes struct {
	Timestamp    time.Time
	ProjectTypes []ServerlessProjectType
}

func loadCachedServerlessProjectTypes() ([]ServerlessProjectType, bool, error) {
	locationManager, err := locations.NewLocationManager()
	if err != nil {
		return nil, false, err
	}

	cacheDir := locationManager.CacheDir(locations.KibanaConfigCacheName)
	cacheFilePath := filepath.Join(cacheDir, cachedServerlessProjectTypesFileName)
	d, err := os.ReadFile(cacheFilePath)
	if errors.Is(err, os.ErrNotExist) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}

	var cache cachedServerlessProjectTypes
	err = json.Unmarshal(d, &cache)
	if err != nil {
		return nil, false, err
	}

	if time.Now().After(cache.Timestamp.Add(cachedServerlessProjectTypesCacheExpiration)) {
		return cache.ProjectTypes, false, nil
	}

	return cache.ProjectTypes, true, nil
}

func saveCachedServerlessProjectTypes(projectTypes []ServerlessProjectType) error {
	locationManager, err := locations.NewLocationManager()
	if err != nil {
		return err
	}

	cacheDir := locationManager.CacheDir(locations.KibanaConfigCacheName)
	err = os.MkdirAll(cacheDir, 0755)
	if err != nil {
		return err
	}

	cache := cachedServerlessProjectTypes{
		Timestamp:    time.Now(),
		ProjectTypes: projectTypes,
	}
	d, err := json.Marshal(cache)
	if err != nil {
		return err
	}

	cacheFilePath := filepath.Join(cacheDir, cachedServerlessProjectTypesFileName)
	err = os.WriteFile(cacheFilePath, d, 0644)
	if err != nil {
		return err
	}

	return nil
}
