// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package docs

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"

	"github.com/elastic/elastic-package/internal/environment"
	"github.com/elastic/elastic-package/internal/files"
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

func (d *DocsRenderer) readLinksMap() (linkMap, error) {
	linksFilePath, err := d.linksDefinitionsFilePath()
	if err != nil {
		return linkMap{}, fmt.Errorf("locating links file failed: %w", err)
	}

	links := newLinkMap()
	if linksFilePath == "" {
		return links, nil
	}

	d.logger.Debug("Using links definitions file", slog.String("file", linksFilePath))
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

// linksDefinitionsFilePath returns the path where links definitions are located or empty string if the file does not exist.
// If linksMapFilePathEnvVar is defined, it returns the value of that env var.
func (d *DocsRenderer) linksDefinitionsFilePath() (string, error) {
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
		d.logger.Debug("links definitions default file doesn't exist", slog.String("file", linksFilePath))
		return "", nil
	}

	return linksFilePath, nil

}
