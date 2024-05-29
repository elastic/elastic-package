// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package docs

import (
	"bytes"
	"errors"
	"fmt"
	"log/slog"
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

type DocsRenderer struct {
	logger *slog.Logger
}

type DocsRenderedOption func(d *DocsRenderer)

func NewDocsRenderer(opts ...DocsRenderedOption) *DocsRenderer {
	d := DocsRenderer{
		logger: logger.Logger,
	}
	for _, opt := range opts {
		opt(&d)
	}
	return &d
}

func WithLogger(logger *slog.Logger) DocsRenderedOption {
	return func(d *DocsRenderer) {
		d.logger = logger
	}
}

// AreReadmesUpToDate function checks if all the .md readme files are up-to-date.
func (d *DocsRenderer) AreReadmesUpToDate() ([]ReadmeFile, error) {
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
		ok, diff, err := d.isReadmeUpToDate(fileName, packageRoot)
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

func (d *DocsRenderer) isReadmeUpToDate(fileName, packageRoot string) (bool, string, error) {
	d.logger.Debug("Check if file is up-to-date", slog.String("file", fileName))

	rendered, shouldBeRendered, err := d.generateReadme(fileName, packageRoot)
	if err != nil {
		return false, "", fmt.Errorf("generating readme file failed: %w", err)
	}
	if !shouldBeRendered {
		return true, "", nil // README file is static and doesn't use template.
	}

	existing, found, err := d.readReadme(fileName, packageRoot)
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
func (d *DocsRenderer) UpdateReadmes(packageRoot string) ([]string, error) {
	readmeFiles, err := filepath.Glob(filepath.Join(packageRoot, "_dev", "build", "docs", "*.md"))
	if err != nil {
		return nil, fmt.Errorf("reading directory entries failed: %w", err)
	}

	var targets []string
	for _, filePath := range readmeFiles {
		fileName := filepath.Base(filePath)
		target, err := d.updateReadme(fileName, packageRoot)
		if err != nil {
			return nil, fmt.Errorf("updating readme file %s failed: %w", fileName, err)
		}

		if target != "" {
			targets = append(targets, target)
		}
	}
	return targets, nil
}

func (d *DocsRenderer) updateReadme(fileName, packageRoot string) (string, error) {
	d.logger.Debug("Update file", slog.String("file", fileName))

	rendered, shouldBeRendered, err := d.generateReadme(fileName, packageRoot)
	if err != nil {
		return "", err
	}
	if !shouldBeRendered {
		return "", nil
	}

	target, err := d.writeReadme(fileName, packageRoot, rendered)
	if err != nil {
		return "", fmt.Errorf("writing %s file failed: %w", fileName, err)
	}

	packageBuildRoot, err := builder.BuildPackagesDirectory(packageRoot)
	if err != nil {
		return "", fmt.Errorf("package build root not found: %w", err)
	}

	_, err = d.writeReadme(fileName, packageBuildRoot, rendered)
	if err != nil {
		return "", fmt.Errorf("writing %s file failed: %w", fileName, err)
	}
	return target, nil
}

func (d *DocsRenderer) generateReadme(fileName, packageRoot string) ([]byte, bool, error) {
	d.logger.Debug("Generate file", slog.String("file", fileName), slog.String("package", packageRoot))
	templatePath, found, err := findReadmeTemplatePath(fileName, packageRoot)
	if err != nil {
		return nil, false, fmt.Errorf("can't locate %s template file: %w", fileName, err)
	}
	if !found {
		d.logger.Debug("README file is static, can't be generated from the template file")
		return nil, false, nil
	}
	d.logger.Debug("Template file for file found", slog.String("file", fileName), slog.String("template", templatePath))

	// TODO
	linksMap, err := d.readLinksMap()
	if err != nil {
		return nil, false, err
	}

	rendered, err := d.renderReadme(fileName, packageRoot, templatePath, linksMap)
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

func (d *DocsRenderer) renderReadme(fileName, packageRoot, templatePath string, linksMap linkMap) ([]byte, error) {
	d.logger.Debug("Render file", slog.String("file", fileName), slog.String("package", packageRoot), slog.String("template", templatePath))

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

func (d *DocsRenderer) readReadme(fileName, packageRoot string) ([]byte, bool, error) {
	d.logger.Debug("Read existing file", slog.String("file", fileName), slog.String("package", packageRoot))

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

func (d *DocsRenderer) writeReadme(fileName, packageRoot string, content []byte) (string, error) {
	d.logger.Debug("Write file", slog.String("file", fileName), slog.String("package", packageRoot))

	docsPath := docsPath(packageRoot)
	d.logger.Debug("Create directories", slog.String("directories", docsPath))
	err := os.MkdirAll(docsPath, 0o755)
	if err != nil {
		return "", fmt.Errorf("mkdir failed (path: %s): %w", docsPath, err)
	}

	aReadmePath := readmePath(fileName, packageRoot)
	d.logger.Debug("Write file", slog.String("source", fileName), slog.String("target", aReadmePath))

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
