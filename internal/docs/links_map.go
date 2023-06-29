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
	"github.com/elastic/elastic-package/internal/files"
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

func newLinkMap() linkMap {
	var links linkMap
	links.Links = make(map[string]string)
	return links
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

func readLinksMap() (linkMap, error) {
	linksFilePath, err := linksDefinitionsFilePath()
	if err != nil {
		return linkMap{}, fmt.Errorf("locating links file failed: %w", err)
	}

	links := newLinkMap()
	if linksFilePath == "" {
		return links, nil
	}

	logger.Debugf("Using links definitions file: %s", linksFilePath)
	contents, err := os.ReadFile(linksFilePath)
	if err != nil {
		return linkMap{}, fmt.Errorf("readfile failed (path: %s): %w", linksFilePath, err)
	}

	err = yaml.Unmarshal(contents, &links)
	if err != nil {
		return linkMap{}, err
	}

	return links, nil
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

// linksDefinitionsFilePath returns the path where links definitions are located or empty string if the file does not exist.
// If linksMapFilePathEnvVar is defined, it returns the value of that env var.
func linksDefinitionsFilePath() (string, error) {
	var err error
	linksFilePath, ok := os.LookupEnv(linksMapFilePathEnvVar)
	if ok {
		_, err = os.Stat(linksFilePath)
		if err != nil {
			// if env var is defined, file must exist
			return "", fmt.Errorf("links definitions file set with %s doesn't exist: %s", linksMapFilePathEnvVar, linksFilePath)
		}
		return linksFilePath, nil
	}

	dir, err := files.FindRepositoryRootDirectory()
	if err != nil {
		return "", err
	}

	linksFilePath = filepath.Join(dir, linksMapFileNameDefault)
	_, err = os.Stat(linksFilePath)
	if err != nil {
		logger.Debugf("links definitions default file doesn't exist: %s", linksFilePath)
		return "", nil
	}

	return linksFilePath, nil

}
