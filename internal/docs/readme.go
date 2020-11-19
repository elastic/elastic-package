// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package docs

import (
	"bytes"
	"io/ioutil"
	"os"
	"path/filepath"
	"text/template"

	"github.com/pkg/errors"

	"github.com/elastic/elastic-package/internal/logger"
	"github.com/elastic/elastic-package/internal/packages"
)

// ReadmeFile for the Elastic package
const ReadmeFile = "README.md"

// IsReadmeUpToDate function checks if the README file is up-to-date.
func IsReadmeUpToDate() (bool, error) {
	logger.Debugf("Check if %s is up-to-date", ReadmeFile)

	packageRoot, err := packages.MustFindPackageRoot()
	if err != nil {
		return false, errors.Wrap(err, "package root not found")
	}

	rendered, shouldBeRendered, err := generateReadme(packageRoot)
	if err != nil {
		return false, err
	}
	if !shouldBeRendered {
		return true, nil // README file is static and doesn't use template.
	}

	existing, found, err := readReadme(packageRoot)
	if err != nil {
		return false, errors.Wrap(err, "reading README file failed")
	}
	if !found {
		return false, nil
	}
	return bytes.Equal(existing, rendered), nil
}

// UpdateReadme function updates the README file using ą defined template file. The function doesn't perform any action
// if the template file is not present.
func UpdateReadme() (string, error) {
	logger.Debugf("Update the %s file", ReadmeFile)

	packageRoot, err := packages.MustFindPackageRoot()
	if err != nil {
		return "", errors.Wrap(err, "package root not found")
	}

	rendered, shouldBeRendered, err := generateReadme(packageRoot)
	if err != nil {
		return "", err
	}
	if !shouldBeRendered {
		return "", nil
	}

	target, err := writeReadme(packageRoot, rendered)
	if err != nil {
		return "", errors.Wrapf(err, "writing %s file failed", ReadmeFile)
	}
	return target, nil
}

func generateReadme(packageRoot string) ([]byte, bool, error) {
	logger.Debugf("Generate %s file (package: %s)", ReadmeFile, packageRoot)
	templatePath, found, err := findReadmeTemplatePath(packageRoot)
	if err != nil {
		return nil, false, errors.Wrapf(err, "can't locate %s template file", ReadmeFile)
	}
	if !found {
		logger.Debug("README file is static, can't be generated from the template file")
		return nil, false, nil
	}
	logger.Debugf("Template file for %s found: %s", ReadmeFile, templatePath)

	rendered, err := renderReadme(packageRoot, templatePath)
	if err != nil {
		return nil, true, errors.Wrap(err, "rendering Readme failed")
	}
	return rendered, true, nil
}

func findReadmeTemplatePath(packageRoot string) (string, bool, error) {
	templatePath := filepath.Join(packageRoot, "_dev", "build", "docs", ReadmeFile)
	_, err := os.Stat(templatePath)
	if err != nil && os.IsNotExist(err) {
		return "", false, nil // README.md file not found
	}
	if err != nil {
		return "", false, errors.Wrapf(err, "can't located the %s file", ReadmeFile)
	}
	return templatePath, true, nil
}

func renderReadme(packageRoot, templatePath string) ([]byte, error) {
	logger.Debugf("Render %s file (package: %s, templatePath: %s)", ReadmeFile, packageRoot, templatePath)

	t := template.New(ReadmeFile)
	t, err := t.Funcs(template.FuncMap{
		"event": func(dataStreamName string) (string, error) {
			return renderSampleEvent(packageRoot, dataStreamName)
		},
		"fields": func(dataStreamName string) (string, error) {
			return renderExportedFields(packageRoot, dataStreamName)
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

func readReadme(packageRoot string) ([]byte, bool, error) {
	logger.Debugf("Read existing %s file (package: %s)", ReadmeFile, packageRoot)

	readmePath := filepath.Join(packageRoot, "docs", ReadmeFile)
	b, err := ioutil.ReadFile(readmePath)
	if err != nil && os.IsNotExist(err) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, errors.Wrapf(err, "readfile failed (path: %s)", readmePath)
	}
	return b, true, err
}

func writeReadme(packageRoot string, content []byte) (string, error) {
	logger.Debugf("Write %s file (package: %s)", ReadmeFile, packageRoot)

	docsPath := docsPath(packageRoot)
	logger.Debugf("Create directories: %s", docsPath)
	err := os.MkdirAll(docsPath, 0755)
	if err != nil {
		return "", errors.Wrapf(err, "mkdir failed (path: %s)", docsPath)
	}

	aReadmePath := readmePath(packageRoot)
	logger.Debugf("Write %s file to: %s", ReadmeFile, aReadmePath)

	err = ioutil.WriteFile(aReadmePath, content, 0644)
	if err != nil {
		return "", errors.Wrapf(err, "writing file failed (path: %s)", aReadmePath)
	}
	return aReadmePath, nil
}

func readmePath(packageRoot string) string {
	return filepath.Join(docsPath(packageRoot), ReadmeFile)
}

func docsPath(packageRoot string) string {
	return filepath.Join(packageRoot, "docs")
}
