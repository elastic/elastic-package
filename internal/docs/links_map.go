// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package docs

import (
	"encoding/csv"
	"fmt"
	"os"

	"github.com/pkg/errors"

	"github.com/elastic/elastic-package/internal/common"
)

const (
	linksMapFileNameDefault = "links_table.csv"
	envLinksMapFilePath     = common.ElasticPackageEnvPrefix + "LINKS_FILE_PATH"
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

func readLinksMap(linksFilePath string) (linkMap, error) {
	links := newLinkMap()
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

// LinksFilePath returns the path where links definitions are located.
// If ELASTIC_PACKAGE_LINKS_FILE_PATH env. variable is defined, it returns that value.
// If not defined, it returns the default location that is located at the root of the repository
func LinksFilePath() (string, error) {
	filepath, ok := os.LookupEnv(envLinksMapFilePath)
	if !ok {
		return common.FindFileRootDirectory(linksMapFileNameDefault)
	}

	if _, err := os.Stat(filepath); err != nil {
		return "", err
	}
	return filepath, nil
}
