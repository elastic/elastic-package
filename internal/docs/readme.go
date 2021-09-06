// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package docs

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"text/template"

	"github.com/pkg/errors"

	"github.com/elastic/elastic-package/internal/builder"
	"github.com/elastic/elastic-package/internal/logger"
	"github.com/elastic/elastic-package/internal/packages"
)

// ReadmeFile contains file name and status of each readme file.
type ReadmeFile struct {
	FileName string
	UpToDate bool
	Error    error
}

// AreReadmesUpToDate function checks if all the .md readme files are up-to-date.
func AreReadmesUpToDate() ([]ReadmeFile, error) {
	packageRoot, err := packages.MustFindPackageRoot()
	if err != nil {
		return nil, errors.Wrap(err, "package root not found")
	}

	files, err := filepath.Glob(filepath.Join(packageRoot, "_dev", "build", "docs", "*.md"))
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, errors.Wrap(err, "reading directory entries failed")
	}

	var readmeFiles []ReadmeFile
	for _, filePath := range files {
		fileName := filepath.Base(filePath)
		ok, err := isReadmeUpToDate(fileName, packageRoot)
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

func isReadmeUpToDate(fileName, packageRoot string) (bool, error) {
	logger.Debugf("Check if %s is up-to-date", fileName)

	rendered, shouldBeRendered, err := generateReadme(fileName, packageRoot)
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

	var targets []string
	for _, filePath := range readmeFiles {
		fileName := filepath.Base(filePath)
		target, err := updateReadme(fileName, packageRoot)
		if err != nil {
			return nil, errors.Wrapf(err, "updating readme file %s failed", fileName)
		}

		if target != "" {
			targets = append(targets, target)
		}
	}
	return targets, nil
}

func updateReadme(fileName, packageRoot string) (string, error) {
	logger.Debugf("Update the %s file", fileName)

	rendered, shouldBeRendered, err := generateReadme(fileName, packageRoot)
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

func generateReadme(fileName, packageRoot string) ([]byte, bool, error) {
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

	rendered, err := renderReadme(fileName, packageRoot, templatePath)
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

func renderReadme(fileName, packageRoot, templatePath string) ([]byte, error) {
	logger.Debugf("Render %s file (package: %s, templatePath: %s)", fileName, packageRoot, templatePath)

	t := template.New(fileName)
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
