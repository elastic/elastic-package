// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package docs

import (
	"encoding/csv"
	"fmt"
	"os"
	"path/filepath"

	"github.com/pkg/errors"

	"github.com/elastic/elastic-package/internal/common"
	"github.com/elastic/elastic-package/internal/logger"
)

const (
	linksMapFileNameDefault = "links_table.csv"
	linksMapFilePathEnvVar  = common.ElasticPackageEnvPrefix + "LINKS_FILE_PATH"
)

type linkMap map[string]string

func newLinkMap() linkMap {
	return make(linkMap)
}

func (l linkMap) Get(key string) (string, error) {
	if url, ok := l[key]; ok {
		return url, nil
	}
	return "", errors.Errorf("link key %s not found", key)
}

func (l linkMap) Add(key, value string) error {
	if _, ok := l[key]; ok {
		return errors.Errorf("link key %s already present", key)
	}
	l[key] = value
	return nil
}

func readLinksMap() (linkMap, error) {
	linksFilePath, err := linksDefinitionsFilePath()
	if err != nil {
		return nil, errors.Wrap(err, "locating links file failed")
	}

	links := newLinkMap()
	if linksFilePath == "" {
		return links, nil
	}

	logger.Debugf("Using links definitions file: %s", linksFilePath)
	f, err := os.Open(linksFilePath)
	if err != nil {
		return nil, errors.Wrapf(err, "readfile failed (path: %s)", linksFilePath)
	}
	lines, err := csv.NewReader(f).ReadAll()
	if err != nil {
		return links, err
	}

	for _, line := range lines {
		links.Add(line[0], line[1])
	}
	return links, nil
}

func (l linkMap) RenderUrl(key string) (string, error) {
	url, err := l.Get(key)
	if err != nil {
		return "", err
	}
	return url, nil
}

func (l linkMap) RenderLink(key, link string) (string, error) {
	url, err := l.Get(key)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("[%s](%s)", link, url), nil
}

// linksDefinitionsFilePath returns the path where links definitions are located or empty string if the file does not exist.
// If linksMapFilePathEnvVar is defined, it returns the value of that env. var.
func linksDefinitionsFilePath() (string, error) {
	var err error
	linksFilePath, ok := os.LookupEnv(linksMapFilePathEnvVar)
	if ok {
		_, err = os.Stat(linksFilePath)
		if err != nil {
			// if env var is defined, file must exist
			return "", fmt.Errorf("not found links definitions file (%s) defined by %s", linksFilePath, linksMapFilePathEnvVar)
		}
		return linksFilePath, nil
	}

	dir, err := common.FindRepositoryRootDirectory()
	if err != nil {
		return "", err
	}

	linksFilePath = filepath.Join(dir, linksMapFileNameDefault)
	_, err = os.Stat(linksFilePath)
	if err != nil {
		logger.Debugf("Not found links definitions file at the default location (%s), skipping", linksFilePath)
		return "", nil
	}

	return linksFilePath, nil

}
