// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

// Package locations manages base file and directory locations from within the elastic-package config
package locations

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/elastic/elastic-package/internal/environment"
)

const (
	elasticPackageDir = ".elastic-package"
	stackDir          = "stack"
	packagesDir       = "development"
	profilesDir       = "profiles"

	temporaryDir = "tmp"
	deployerDir  = "deployer"

	cacheDir              = "cache"
	FieldsCacheName       = "fields"
	KibanaConfigCacheName = "kibana_config"

	llm     = "llm_config"
	mcpJson = "mcp.json"
)

var (
	// elasticPackageDataHome is the name of the environment variable used to override data folder for elastic-package
	elasticPackageDataHome = environment.WithElasticPackagePrefix("DATA_HOME")

	serviceLogsDir   = filepath.Join(temporaryDir, "service_logs")
	rallyCorpusDir   = filepath.Join(temporaryDir, "rally_corpus")
	serviceOutputDir = filepath.Join(temporaryDir, "output")
)

// LocationManager maintains an instance of a config path location
type LocationManager struct {
	stackPath string
}

// NewLocationManager returns a new manager to track the Configuration dir
func NewLocationManager() (*LocationManager, error) {
	cfg, err := configurationDir()
	if err != nil {
		return nil, fmt.Errorf("error getting config dir: %w", err)
	}

	return &LocationManager{stackPath: cfg}, nil
}

// RootDir returns the root elastic-package dir
func (loc LocationManager) RootDir() string {
	return loc.stackPath
}

// ProfileDir is the root profile management directory
func (loc LocationManager) ProfileDir() string {
	return filepath.Join(loc.stackPath, profilesDir)
}

// TempDir returns the temp directory location
func (loc LocationManager) TempDir() string {
	return filepath.Join(loc.stackPath, temporaryDir)
}

// DeployerDir returns the deployer directory location
func (loc LocationManager) DeployerDir() string {
	return filepath.Join(loc.stackPath, deployerDir)
}

// PackagesDir returns the packages directory location
func (loc LocationManager) PackagesDir() string {
	return filepath.Join(loc.stackPath, stackDir, packagesDir)
}

// RallyCorpusDir returns the rally coprus directory
func (loc LocationManager) RallyCorpusDir() string {
	return filepath.Join(loc.stackPath, rallyCorpusDir)
}

// ServiceLogDir returns the log directory
func (loc LocationManager) ServiceLogDir() string {
	return filepath.Join(loc.stackPath, serviceLogsDir)
}

// ServiceOutputDir returns the output directory
func (loc LocationManager) ServiceOutputDir() string {
	return filepath.Join(loc.stackPath, serviceOutputDir)
}

// CacheDir returns the directory with cached fields
func (loc LocationManager) CacheDir(name string) string {
	return filepath.Join(loc.stackPath, cacheDir, name)
}

// LLMDir returns the directory with the LLM configuration
func (loc LocationManager) LLMDir() string {
	return filepath.Join(loc.stackPath, llm)
}

// MCPJson returns the file location for the MCP server configuration
func (loc LocationManager) MCPJson() string {
	return filepath.Join(loc.LLMDir(), mcpJson)
}

// configurationDir returns the configuration directory location
// If a environment variable named as in elasticPackageDataHome is present,
// the value is used as is, overriding the value of this function.
func configurationDir() (string, error) {
	customHome := os.Getenv(elasticPackageDataHome)
	if customHome != "" {
		return customHome, nil
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("reading home dir failed: %w", err)
	}
	return filepath.Join(homeDir, elasticPackageDir), nil
}
