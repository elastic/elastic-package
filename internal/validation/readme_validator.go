// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package validation

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/text"
)

func ValidateReadmeStructure(packageRoot string, enforcedSections []string) (error) {
	docsFolderPath := filepath.Join(packageRoot, "docs")
	files, err := os.ReadDir(docsFolderPath)
	if err != nil && errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("Docs folder %s not found: %w", docsFolderPath, err)
	}

	var errs DocsValidationError

	for _, file := range files {
		if !file.IsDir() {
			fullPath := filepath.Join(docsFolderPath, file.Name())

			content, err := os.ReadFile(fullPath)
			if err != nil {
				fmt.Printf("Error opening file %s: %v\n", fullPath, err)
				continue
			}

			validationErrs := validateContent(file.Name(), content, enforcedSections)
			if validationErrs != nil {
				errs = append(errs, validationErrs)
			}
		}
	}

	if len(errs) == 0 {
		return nil
	}

	return errs
}

func validateContent(filename string, content []byte, enforcedSections []string) (error) {
	var errs DocsValidationError

	md := goldmark.New()

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
			errs = append(errs, fmt.Errorf("missing required section '%s' in file '%s'", header, filename))
		}
	}

	if len(errs) == 0 {
		return nil
	}

	return errs
}

