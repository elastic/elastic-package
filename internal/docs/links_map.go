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

const linksMapFileName = "links_table.csv"

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
	links := newLinkMap()
	linksMapPath, err := common.FindFileRootDirectory(linksMapFileName)
	if err != nil && errors.Is(err, os.ErrNotExist) {
		return links, nil
	}
	if err != nil {
		return nil, err
	}

	f, err := os.Open(linksMapPath)
	if err != nil {
		return nil, errors.Wrapf(err, "readfile failed (path: %s)", linksMapPath)
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

func (l linkMap) renderUrl(key string) (string, error) {
	url, err := l.Get(key)
	if err != nil {
		return "", err
	}
	return url, nil
}

func (l linkMap) renderLink(key, link string) (string, error) {
	url, err := l.Get(key)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("[%s](%s)", link, url), nil
}
