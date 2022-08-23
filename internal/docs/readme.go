// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package docs

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"os"
	"path/filepath"
	"text/template"

	"github.com/pkg/errors"

	"github.com/elastic/elastic-package/internal/builder"
	"github.com/elastic/elastic-package/internal/common"
	"github.com/elastic/elastic-package/internal/logger"
	"github.com/elastic/elastic-package/internal/packages"
)

const linksMapFileName = "links_table.csv"

// ReadmeFile contains file name and status of each readme file.
type ReadmeFile struct {
	FileName string
	UpToDate bool
	Error    error
}

type linkMap map[string]string

func NewLinkMap() linkMap {
	return make(linkMap)
}

func (l linkMap) Get(key string) (string, error) {
	if url, ok := l[key]; ok {
		return url, nil
	}
	return "", errors.Errorf("Link key %s not found", key)
}

func (l linkMap) Add(key, value string) error {
	if _, ok := l[key]; ok {
		return errors.Errorf("Link key %s already present", key)
	}
	l[key] = value
	return nil
}

// AreReadmesUpToDate function checks if all the .md readme files are up-to-date.
func AreReadmesUpToDate() ([]ReadmeFile, error) {
	packageRoot, err := packages.MustFindPackageRoot()
	if err != nil {
		return nil, errors.Wrap(err, "package root not found")
	}

	linksMap, err := readLinksMap()
	if err != nil {
		return nil, err
	}

	files, err := filepath.Glob(filepath.Join(packageRoot, "_dev", "build", "docs", "*.md"))
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, errors.Wrap(err, "reading directory entries failed")
	}

	var readmeFiles []ReadmeFile
	for _, filePath := range files {
		fileName := filepath.Base(filePath)
		ok, err := isReadmeUpToDate(fileName, packageRoot, linksMap)
		if !ok || err != nil {
			readmeFile := ReadmeFile{
				FileName: fileName,
				UpToDate: ok,
				Error:    err,
			}
			readmeFiles = append(readmeFiles, readmeFile)
		}
	}

	if readmeFiles != nil {
		return readmeFiles, fmt.Errorf("checking readme files are up-to-date failed")
	}
	return readmeFiles, nil
}

func readLinksMap() (linkMap, error) {
	links := NewLinkMap()
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

func isReadmeUpToDate(fileName, packageRoot string, linksMap linkMap) (bool, error) {
	logger.Debugf("Check if %s is up-to-date", fileName)

	rendered, shouldBeRendered, err := generateReadme(fileName, packageRoot, linksMap)
	if err != nil {
		return false, errors.Wrap(err, "generating readme file failed")
	}
	if !shouldBeRendered {
		return true, nil // README file is static and doesn't use template.
	}

	existing, found, err := readReadme(fileName, packageRoot)
	if err != nil {
		return false, errors.Wrap(err, "reading README file failed")
	}
	if !found {
		return false, nil
	}
	return bytes.Equal(existing, rendered), nil
}

// UpdateReadmes function updates all .md readme files using a defined template
// files. The function doesn't perform any action if the template file is not present.
func UpdateReadmes(packageRoot string) ([]string, error) {
	readmeFiles, err := filepath.Glob(filepath.Join(packageRoot, "_dev", "build", "docs", "*.md"))
	if err != nil {
		return nil, errors.Wrap(err, "reading directory entries failed")
	}

	linksMap, err := readLinksMap()
	if err != nil {
		return nil, err
	}

	var targets []string
	for _, filePath := range readmeFiles {
		fileName := filepath.Base(filePath)
		target, err := updateReadme(fileName, packageRoot, linksMap)
		if err != nil {
			return nil, errors.Wrapf(err, "updating readme file %s failed", fileName)
		}

		if target != "" {
			targets = append(targets, target)
		}
	}
	return targets, nil
}

func updateReadme(fileName, packageRoot string, linksMap linkMap) (string, error) {
	logger.Debugf("Update the %s file", fileName)

	rendered, shouldBeRendered, err := generateReadme(fileName, packageRoot, linksMap)
	if err != nil {
		return "", err
	}
	if !shouldBeRendered {
		return "", nil
	}

	target, err := writeReadme(fileName, packageRoot, rendered)
	if err != nil {
		return "", errors.Wrapf(err, "writing %s file failed", fileName)
	}

	packageBuildRoot, err := builder.BuildPackagesDirectory(packageRoot)
	if err != nil {
		return "", errors.Wrap(err, "package build root not found")
	}

	_, err = writeReadme(fileName, packageBuildRoot, rendered)
	if err != nil {
		return "", errors.Wrapf(err, "writing %s file failed", fileName)
	}
	return target, nil
}

func generateReadme(fileName, packageRoot string, linksMap linkMap) ([]byte, bool, error) {
	logger.Debugf("Generate %s file (package: %s)", fileName, packageRoot)
	templatePath, found, err := findReadmeTemplatePath(fileName, packageRoot)
	if err != nil {
		return nil, false, errors.Wrapf(err, "can't locate %s template file", fileName)
	}
	if !found {
		logger.Debug("README file is static, can't be generated from the template file")
		return nil, false, nil
	}
	logger.Debugf("Template file for %s found: %s", fileName, templatePath)

	rendered, err := renderReadme(fileName, packageRoot, templatePath, linksMap)
	if err != nil {
		return nil, true, errors.Wrap(err, "rendering Readme failed")
	}
	return rendered, true, nil
}

func findReadmeTemplatePath(fileName, packageRoot string) (string, bool, error) {
	templatePath := filepath.Join(packageRoot, "_dev", "build", "docs", fileName)
	_, err := os.Stat(templatePath)
	if err != nil && errors.Is(err, os.ErrNotExist) {
		return "", false, nil // README.md file not found
	}
	if err != nil {
		return "", false, errors.Wrapf(err, "can't stat the %s file", fileName)
	}
	return templatePath, true, nil
}

func renderReadme(fileName, packageRoot, templatePath string, linksMap linkMap) ([]byte, error) {
	logger.Debugf("Render %s file (package: %s, templatePath: %s)", fileName, packageRoot, templatePath)

	t := template.New(fileName)
	t, err := t.Funcs(template.FuncMap{
		"event": func(dataStreamName string) (string, error) {
			return renderSampleEvent(packageRoot, dataStreamName)
		},
		"fields": func(args ...string) (string, error) {
			if len(args) > 0 {
				dataStreamPath := filepath.Join(packageRoot, "data_stream", args[0])
				return renderExportedFields(dataStreamPath)
			}
			return renderExportedFields(packageRoot)
		},
		"url": func(key string) (string, error) {
			return linksMap.Get(key)
		},
	}).ParseFiles(templatePath)
	if err != nil {
		return nil, errors.Wrapf(err, "parsing README template failed (path: %s)", templatePath)
	}

	var rendered bytes.Buffer
	err = t.Execute(&rendered, nil)
	if err != nil {
		return nil, errors.Wrap(err, "executing template failed")
	}
	return rendered.Bytes(), nil
}

func readReadme(fileName, packageRoot string) ([]byte, bool, error) {
	logger.Debugf("Read existing %s file (package: %s)", fileName, packageRoot)

	readmePath := filepath.Join(packageRoot, "docs", fileName)
	b, err := os.ReadFile(readmePath)
	if err != nil && errors.Is(err, os.ErrNotExist) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, errors.Wrapf(err, "readfile failed (path: %s)", readmePath)
	}
	return b, true, err
}

func writeReadme(fileName, packageRoot string, content []byte) (string, error) {
	logger.Debugf("Write %s file (package: %s)", fileName, packageRoot)

	docsPath := docsPath(packageRoot)
	logger.Debugf("Create directories: %s", docsPath)
	err := os.MkdirAll(docsPath, 0755)
	if err != nil {
		return "", errors.Wrapf(err, "mkdir failed (path: %s)", docsPath)
	}

	aReadmePath := readmePath(fileName, packageRoot)
	logger.Debugf("Write %s file to: %s", fileName, aReadmePath)

	err = os.WriteFile(aReadmePath, content, 0644)
	if err != nil {
		return "", errors.Wrapf(err, "writing file failed (path: %s)", aReadmePath)
	}
	return aReadmePath, nil
}

func readmePath(fileName, packageRoot string) string {
	return filepath.Join(docsPath(packageRoot), fileName)
}

func docsPath(packageRoot string) string {
	return filepath.Join(packageRoot, "docs")
}
