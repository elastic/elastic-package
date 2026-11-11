// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package export

import (
	"bytes"
	"fmt"
	"regexp"
	"strings"

	"github.com/elastic/elastic-package/internal/kibana"
	"github.com/elastic/elastic-package/internal/yamledit"
)

// WorkflowsDir is the directory, relative to the package root, where workflow assets are stored.
const WorkflowsDir = "kibana/workflow"

var workflowSlugInvalidChars = regexp.MustCompile(`[^a-z0-9]+`)

// WorkflowFile is a workflow ready to be stored in a package.
type WorkflowFile struct {
	// Name is the file name, relative to WorkflowsDir.
	Name    string
	Content []byte
}

type workflowDefinition struct {
	Name string `yaml:"name"`
}

// workflowFileName returns the name of the file that stores the workflow with the given name
// in the given package. It has the form "{pkgName}-{slug}.yml", where slug only contains [a-z0-9-].
func workflowFileName(pkgName, workflowName string) (string, error) {
	slug := workflowSlugInvalidChars.ReplaceAllString(strings.ToLower(workflowName), "-")
	slug = strings.Trim(slug, "-")
	if slug == "" {
		return "", fmt.Errorf("cannot build a file name from workflow name %q", workflowName)
	}
	return pkgName + "-" + slug + ".yml", nil
}

// fleetWorkflowFileName returns the name of the package file a Fleet-installed workflow was installed
// from. Fleet encodes the file base in the workflow ID, lowercased and with "_" replaced by "-". The
// package name prefix is restored with the actual package name.
func fleetWorkflowFileName(pkgName, fileBase string) (string, error) {
	normalizedPkgName := strings.ReplaceAll(strings.ToLower(pkgName), "_", "-")
	suffix, found := strings.CutPrefix(fileBase, normalizedPkgName+"-")
	if !found || suffix == "" {
		return "", fmt.Errorf("workflow file %q was not installed from package %q", fileBase, pkgName)
	}
	return pkgName + "-" + suffix + ".yml", nil
}

// TransformWorkflow prepares a workflow, as exported from Kibana, to be stored in the given package.
// It disables the workflow, preserving comments and key order.
// Workflows installed by Fleet keep the name of the package file they were installed from, other
// workflows are named after their name.
func TransformWorkflow(pkgName string, workflow kibana.ExportedWorkflow) (*WorkflowFile, error) {
	var definition workflowDefinition
	doc, err := yamledit.ParseDocumentBytes([]byte(workflow.YAML), &definition)
	if err != nil {
		return nil, fmt.Errorf("could not parse workflow: %w", err)
	}

	if err := disableWorkflow(doc); err != nil {
		return nil, fmt.Errorf("could not disable workflow %q: %w", definition.Name, err)
	}

	var fileName string
	if fileBase, ok := kibana.FleetPackageWorkflowFileBase(workflow.ID); ok {
		fileName, err = fleetWorkflowFileName(pkgName, fileBase)
	} else {
		fileName, err = workflowFileName(pkgName, definition.Name)
	}
	if err != nil {
		return nil, err
	}

	var buf bytes.Buffer
	if _, err := doc.Write(&buf); err != nil {
		return nil, fmt.Errorf("could not encode workflow %q: %w", definition.Name, err)
	}
	if !bytes.HasSuffix(buf.Bytes(), []byte("\n")) {
		buf.WriteByte('\n')
	}

	return &WorkflowFile{Name: fileName, Content: buf.Bytes()}, nil
}

// disableWorkflow sets "enabled: false" in the workflow document. Package workflows must be
// disabled, users enable them after installation.
func disableWorkflow(doc *yamledit.Document) error {
	_, err := doc.SetKeyValue("$", "enabled", false, yamledit.IndexAppend)
	return err
}
