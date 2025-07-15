// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package validation

import (
	"archive/zip"
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/elastic/go-resource"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/text"

	"github.com/elastic/package-spec/v3/code/go/pkg/specerrors"
	"github.com/elastic/package-spec/v3/code/go/pkg/validator"
)

//go:embed _static
var static embed.FS

var staticSource = resource.NewSourceFS(static)

type DocsValidationError []error

func (dve DocsValidationError) Error() string {
	var out string
	for _, err := range dve {
		out += "\n" + err.Error()
	}
	return out
}

func ValidateFromPath(rootPath string) error {
	return validator.ValidateFromPath(rootPath)
}

func ValidateFromZip(packagePath string) error {
	return validator.ValidateFromZip(packagePath)
}

func ValidateAndFilterFromPath(rootPath string) (error, error) {
	allErrors := validator.ValidateFromPath(rootPath)
	if allErrors == nil {
		return nil, nil
	}

	fsys := os.DirFS(rootPath)
	result, err := filterErrors(allErrors, fsys)
	if err != nil {
		return err, nil
	}
	return result.Processed, result.Removed
}

func ValidateAndFilterFromZip(packagePath string) (error, error) {
	allErrors := validator.ValidateFromZip(packagePath)
	if allErrors == nil {
		return nil, nil
	}

	fsys, err := zip.OpenReader(packagePath)
	if err != nil {
		return fmt.Errorf("failed to open zip file (%s): %w", packagePath, err), nil
	}
	defer fsys.Close()

	fsZip, err := fsFromPackageZip(fsys)
	if err != nil {
		return fmt.Errorf("failed to extract filesystem from zip file (%s): %w", packagePath, err), nil
	}

	result, err := filterErrors(allErrors, fsZip)
	if err != nil {
		return err, nil
	}
	return result.Processed, result.Removed
}

func ValidateDocsStructureFromPath(rootPath string) error {
	fsys := os.DirFS(rootPath)

	enforcedSections, err := retrieveEnforcedDocsSections(fsys)
	if err != nil {
		return fmt.Errorf("failed to retrieve enforced documentation sections: %w", err)
	}

	if len(enforcedSections) == 0 {
		return nil
	}

	return validateReadmeStructure(rootPath, enforcedSections)
}

func ValidateDocsStructureFromZip(packagePath string) error {
	fsys, err := zip.OpenReader(packagePath)
	if err != nil {
		return fmt.Errorf("failed to open zip file (%s): %w", packagePath, err)
	}
	defer fsys.Close()

	fsZip, err := fsFromPackageZip(fsys)
	if err != nil {
		return fmt.Errorf("failed to extract filesystem from zip file (%s): %w", packagePath, err)
	}

	enforcedSections, err := retrieveEnforcedDocsSections(fsZip)
	if err != nil {
		return fmt.Errorf("failed to retrieve enforced documentation sections: %w", err)
	}
	if len(enforcedSections) == 0 {
		return nil
	}

	return validateReadmeStructure(packagePath, enforcedSections)
}

func extractHeadingText(n ast.Node, source []byte) string {
	var builder strings.Builder

	for child := n.FirstChild(); child != nil; child = child.NextSibling() {
		switch t := child.(type) {
		case *ast.Text:
			builder.Write(t.Segment.Value(source))
		}
	}

	return builder.String()
}

func validateReadmeStructure(packageRoot string, enforcedSections []string) error {
	docsFolderPath := filepath.Join(packageRoot, "docs")
	files, err := os.ReadDir(docsFolderPath)
	if err != nil && errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("Docs folder %s not found: %w", docsFolderPath, err)
	}

	md := goldmark.New()
	var errs DocsValidationError

	for _, file := range files {
		fmt.Println("Validating file:", file.Name())
		if !file.IsDir() {
			fullPath := filepath.Join(docsFolderPath, file.Name())

			content, err := os.ReadFile(fullPath)
			if err != nil {
				fmt.Printf("Error opening file %s: %v\n", fullPath, err)
				continue
			}

			reader := text.NewReader(content)
			doc := md.Parser().Parse(reader)

			found := map[string]bool{}
			ast.Walk(doc, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
				if heading, ok := n.(*ast.Heading); ok && entering {
					text := extractHeadingText(heading, content)
					for _, required := range enforcedSections {
						if strings.EqualFold(text, required) {
							found[required] = true
						}
					}
				}
				return ast.WalkContinue, nil
			})

			for _, header := range enforcedSections {
				if !found[header] {
					errs = append(errs, fmt.Errorf("missing required section '%s' in file '%s'", header, file.Name()))
				}
			}

		}
	}

	if len(errs) == 0 {
		return nil
	}

	return errs
}

func fsFromPackageZip(fsys fs.FS) (fs.FS, error) {
	dirs, err := fs.ReadDir(fsys, ".")
	if err != nil {
		return nil, fmt.Errorf("failed to read root directory in zip file fs: %w", err)
	}
	if len(dirs) != 1 {
		return nil, fmt.Errorf("a single directory is expected in zip file, %d found", len(dirs))
	}

	subDir, err := fs.Sub(fsys, dirs[0].Name())
	if err != nil {
		return nil, err
	}
	return subDir, nil
}

func filterErrors(allErrors error, fsys fs.FS) (specerrors.FilterResult, error) {
	errs, ok := allErrors.(specerrors.ValidationErrors)
	if !ok {
		return specerrors.FilterResult{Processed: allErrors, Removed: nil}, nil
	}

	config, err := specerrors.LoadConfigFilter(fsys)
	if err != nil && errors.Is(err, fs.ErrNotExist) {
		return specerrors.FilterResult{Processed: allErrors, Removed: nil}, nil
	}
	if err != nil {
		return specerrors.FilterResult{Processed: allErrors, Removed: nil},
			fmt.Errorf("failed to read config filter: %w", err)
	}
	if config == nil {
		return specerrors.FilterResult{Processed: allErrors, Removed: nil}, nil
	}

	filter := specerrors.NewFilter(config)

	result, err := filter.Run(errs)
	if err != nil {
		return specerrors.FilterResult{Processed: allErrors, Removed: nil},
			fmt.Errorf("failed to filter errors: %w", err)
	}
	return result, nil
}

func retrieveEnforcedDocsSections(fsys fs.FS) ([]string, error) {
	sections := []string{}

	config, err := specerrors.LoadConfigFilter(fsys)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return sections, fmt.Errorf("failed to read config filter: %w", err)
	}

	if config == nil || !config.DocsStructureEnforced.Enabled {
		return sections, nil
	}

	defaultSections, err := loadSectionsFromConfig(fmt.Sprintf("%d", config.DocsStructureEnforced.Version))
	if err != nil {
		fmt.Printf("Failed to load enforced sections from config: %v\n", err)
	}

	for _, section := range defaultSections {
		if contains(config.DocsStructureEnforced.Skip, section) {
			continue
		}
		sections = append(sections, section)
	}

	return sections, nil
}

// contains checks if a string is present in a slice of strings.
func contains(slice []specerrors.Skip, item string) bool {
	for _, s := range slice {
		if s.Title == item {
			return true
		}
	}
	return false
}

type EnforcedSections struct {
	Version  string    `yaml:"version"`
	Sections []Section `yaml:"enforced_sections"`
}

type Section struct {
	Name        string `yaml:"name"`
	Description string `yaml:"description"`
}

func loadSectionsFromConfig(version string) ([]string, error) {
	var schemaPath string
	switch version {
	case "1":
		schemaPath = "_static/docsValidationSchema/enforced_sections_v1.yml"
	default:
		return nil, fmt.Errorf("unsupported format_version: %s", version)
	}

	data, err := fs.ReadFile(staticSource.FS, schemaPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read schema file: %w", err)
	}

	var spec EnforcedSections
	if err := yaml.Unmarshal(data, &spec); err != nil {
		return nil, fmt.Errorf("invalid schema YAML: %w", err)
	}

	if spec.Version != version {
		return nil, fmt.Errorf("schema version mismatch: got %s, expected %s", spec.Version, version)
	}

	sections := make([]string, 0, len(spec.Sections))
	for _, section := range spec.Sections {
		if section.Name != "" {
			sections = append(sections, section.Name)
		}
	}

	return sections, nil
}
