// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package docs

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"

	"github.com/elastic/elastic-package/internal/environment"
	"github.com/elastic/elastic-package/internal/logger"
)

const linksMapFileNameDefault = "links_table.yml"

var linksMapFilePathEnvVar = environment.WithElasticPackagePrefix("LINKS_FILE_PATH")

type linkMap struct {
	Links map[string]string `yaml:"links"`
}

type linkOptions struct {
	caption string
}

func newEmptyLinkMap() linkMap {
	return linkMap{
		Links: make(map[string]string),
	}
}

func (l linkMap) Get(key string) (string, error) {
	if url, ok := l.Links[key]; ok {
		return url, nil
	}
	return "", fmt.Errorf("link key not found: %s", key)
}

func (l linkMap) Add(key, value string) error {
	if _, ok := l.Links[key]; ok {
		return fmt.Errorf("link key already present: %s", key)
	}
	l.Links[key] = value
	return nil
}

// readLinksMap reads the links definitions file from the given repository root directory,
// parses its YAML contents, and returns a populated linkMap. If the links file does not exist,
// it returns an empty linkMap. Returns an error if locating, reading, or unmarshalling the file fails.
func readLinksMap(linksFilePath string) (linkMap, error) {
	// No links file, return empty map with Links initialized
	if linksFilePath == "" {
		return newEmptyLinkMap(), nil
	}

	logger.Debugf("Using links definitions file: %s", linksFilePath)
	contents, err := os.ReadFile(linksFilePath)
	if err != nil {
		return newEmptyLinkMap(), fmt.Errorf("readfile failed (path: %s): %w", linksFilePath, err)
	}

	var lmap linkMap
	err = yaml.Unmarshal(contents, &lmap)
	if err != nil {
		return newEmptyLinkMap(), err
	}

	return lmap, nil
}

func (l linkMap) RenderLink(key string, options linkOptions) (string, error) {
	url, err := l.Get(key)
	if err != nil {
		return "", err
	}
	if options.caption != "" {
		url = fmt.Sprintf("[%s](%s)", options.caption, url)
	}
	return url, nil
}

// linksDefinitionsFilePath returns the file path to the links definitions file.
// It first checks if the environment variable specified by linksMapFilePathEnvVar is set.
// If set, it verifies that the file exists and returns its path, or an error if not found.
// If the environment variable is not set, it falls back to the default file path
// constructed from repositoryRoot and linksMapFileNameDefault, returning the path if the file exists,
// or nil if it does not.
func linksDefinitionsFilePath(repositoryRoot *os.Root) (string, error) {
	linksFilePath := os.Getenv(linksMapFilePathEnvVar)
	if linksFilePath != "" {
		if _, err := os.Stat(linksFilePath); err != nil {
			// if env var is defined, file must exist
			return "", fmt.Errorf("links definitions file set with %s doesn't exist: %s", linksMapFilePathEnvVar, linksFilePath)
		}
		return linksFilePath, nil
	}

	if _, err := repositoryRoot.Stat(linksMapFileNameDefault); err != nil {
		logger.Debugf("links definitions default file doesn't exist: %s", linksFilePath)
		return "", nil
	}

	return filepath.Join(repositoryRoot.Name(), linksMapFileNameDefault), nil
}
