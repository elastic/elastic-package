// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package docs

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"text/template"

	"github.com/pmezard/go-difflib/difflib"

	"github.com/elastic/elastic-package/internal/builder"
	"github.com/elastic/elastic-package/internal/logger"
	"github.com/elastic/elastic-package/internal/packages"
)

// ReadmeFile contains file name and status of each readme file.
type ReadmeFile struct {
	FileName string
	UpToDate bool
	Diff     string
	Error    error
}

// AreReadmesUpToDate function checks if all the .md readme files are up-to-date.
func AreReadmesUpToDate() ([]ReadmeFile, error) {
	packageRoot, err := packages.MustFindPackageRoot()
	if err != nil {
		return nil, fmt.Errorf("package root not found: %w", err)
	}

	files, err := filepath.Glob(filepath.Join(packageRoot, "_dev", "build", "docs", "*.md"))
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("reading directory entries failed: %w", err)
	}

	var readmeFiles []ReadmeFile
	for _, filePath := range files {
		fileName := filepath.Base(filePath)
		ok, diff, err := isReadmeUpToDate(fileName, packageRoot)
		if !ok || err != nil {
			readmeFile := ReadmeFile{
				FileName: fileName,
				UpToDate: ok,
				Diff:     diff,
				Error:    err,
			}
			readmeFiles = append(readmeFiles, readmeFile)
		}
	}

	if readmeFiles != nil {
		return readmeFiles, fmt.Errorf("files do not match")
	}
	return readmeFiles, nil
}

func isReadmeUpToDate(fileName, packageRoot string) (bool, string, error) {
	logger.Debugf("Check if %s is up-to-date", fileName)

	rendered, shouldBeRendered, err := generateReadme(fileName, packageRoot)
	if err != nil {
		return false, "", fmt.Errorf("generating readme file failed: %w", err)
	}
	if !shouldBeRendered {
		return true, "", nil // README file is static and doesn't use template.
	}

	existing, found, err := readReadme(fileName, packageRoot)
	if err != nil {
		return false, "", fmt.Errorf("reading README file failed: %w", err)
	}
	if !found {
		return false, "", nil
	}
	if bytes.Equal(existing, rendered) {
		return true, "", nil
	}
	var buf bytes.Buffer
	err = difflib.WriteUnifiedDiff(&buf, difflib.UnifiedDiff{
		A:        difflib.SplitLines(string(existing)),
		B:        difflib.SplitLines(string(rendered)),
		FromFile: "want",
		ToFile:   "got",
		Context:  1,
	})
	return false, buf.String(), err
}

// UpdateReadmes function updates all .md readme files using a defined template
// files. The function doesn't perform any action if the template file is not present.
func UpdateReadmes(packageRoot string) ([]string, error) {
	readmeFiles, err := filepath.Glob(filepath.Join(packageRoot, "_dev", "build", "docs", "*.md"))
	if err != nil {
		return nil, fmt.Errorf("reading directory entries failed: %w", err)
	}

	var targets []string
	for _, filePath := range readmeFiles {
		fileName := filepath.Base(filePath)
		target, err := updateReadme(fileName, packageRoot)
		if err != nil {
			return nil, fmt.Errorf("updating readme file %s failed: %w", fileName, err)
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
		return "", fmt.Errorf("writing %s file failed: %w", fileName, err)
	}

	packageBuildRoot, err := builder.BuildPackagesDirectory(packageRoot)
	if err != nil {
		return "", fmt.Errorf("package build root not found: %w", err)
	}

	_, err = writeReadme(fileName, packageBuildRoot, rendered)
	if err != nil {
		return "", fmt.Errorf("writing %s file failed: %w", fileName, err)
	}
	return target, nil
}

func generateReadme(fileName, packageRoot string) ([]byte, bool, error) {
	logger.Debugf("Generate %s file (package: %s)", fileName, packageRoot)
	templatePath, found, err := findReadmeTemplatePath(fileName, packageRoot)
	if err != nil {
		return nil, false, fmt.Errorf("can't locate %s template file: %w", fileName, err)
	}
	if !found {
		logger.Debug("README file is static, can't be generated from the template file")
		return nil, false, nil
	}
	logger.Debugf("Template file for %s found: %s", fileName, templatePath)

	linksMap, err := readLinksMap()
	if err != nil {
		return nil, false, err
	}

	rendered, err := renderReadme(fileName, packageRoot, templatePath, linksMap)
	if err != nil {
		return nil, true, fmt.Errorf("rendering Readme failed: %w", err)
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
		return "", false, fmt.Errorf("can't stat the %s file: %w", fileName, err)
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
		"url": func(args ...string) (string, error) {
			options := linkOptions{}
			if len(args) > 1 {
				options.caption = args[1]
			}
			return linksMap.RenderLink(args[0], options)
		},
	}).ParseFiles(templatePath)
	if err != nil {
		return nil, fmt.Errorf("parsing README template failed (path: %s): %w", templatePath, err)
	}

	var rendered bytes.Buffer
	err = t.Execute(&rendered, nil)
	if err != nil {
		return nil, fmt.Errorf("executing template failed: %w", err)
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
		return nil, false, fmt.Errorf("readfile failed (path: %s): %w", readmePath, err)
	}
	return b, true, err
}

func writeReadme(fileName, packageRoot string, content []byte) (string, error) {
	logger.Debugf("Write %s file (package: %s)", fileName, packageRoot)

	docsPath := docsPath(packageRoot)
	logger.Debugf("Create directories: %s", docsPath)
	err := os.MkdirAll(docsPath, 0o755)
	if err != nil {
		return "", fmt.Errorf("mkdir failed (path: %s): %w", docsPath, err)
	}

	aReadmePath := readmePath(fileName, packageRoot)
	logger.Debugf("Write %s file to: %s", fileName, aReadmePath)

	err = os.WriteFile(aReadmePath, content, 0o644)
	if err != nil {
		return "", fmt.Errorf("writing file failed (path: %s): %w", aReadmePath, err)
	}
	return aReadmePath, nil
}

func readmePath(fileName, packageRoot string) string {
	return filepath.Join(docsPath(packageRoot), fileName)
}

func docsPath(packageRoot string) string {
	return filepath.Join(packageRoot, "docs")
}
