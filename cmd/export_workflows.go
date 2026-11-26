// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package cmd

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/elastic/elastic-package/internal/cobraext"
	"github.com/elastic/elastic-package/internal/common"
	"github.com/elastic/elastic-package/internal/export"
	"github.com/elastic/elastic-package/internal/kibana"
	"github.com/elastic/elastic-package/internal/packages"
	"github.com/elastic/elastic-package/internal/stack"
	"github.com/elastic/elastic-package/internal/tui"
)

const exportWorkflowsLongDescription = `Use this command to export workflows from the Kibana instance.

Use this command to download selected workflows from Kibana and store them as workflow assets of the package, in the kibana/workflow directory. Workflows are stored disabled, as required for package assets, in files named after the package and the workflow name. Workflows made editable with elastic-package edit workflows are stored in the file they were installed from.`

func exportWorkflowsCmd(cmd *cobra.Command, args []string) error {
	cmd.Println("Export Kibana workflows")

	workflowIDs, err := cmd.Flags().GetStringSlice(cobraext.WorkflowIDsFlagName)
	if err != nil {
		return cobraext.FlagParsingError(err, cobraext.WorkflowIDsFlagName)
	}
	common.TrimStringSlice(workflowIDs)

	var opts []kibana.ClientOption
	tlsSkipVerify, err := cmd.Flags().GetBool(cobraext.TLSSkipVerifyFlagName)
	if err != nil {
		return cobraext.FlagParsingError(err, cobraext.TLSSkipVerifyFlagName)
	}
	if tlsSkipVerify {
		opts = append(opts, kibana.TLSSkipVerify())
	}

	allowSnapshot, err := cmd.Flags().GetBool(cobraext.AllowSnapshotFlagName)
	if err != nil {
		return cobraext.FlagParsingError(err, cobraext.AllowSnapshotFlagName)
	}

	profile, err := cobraext.GetProfileFlag(cmd)
	if err != nil {
		return err
	}

	packageRoot, err := packages.MustFindPackageRoot()
	if err != nil {
		return fmt.Errorf("locating package root failed: %w", err)
	}
	manifest, err := packages.ReadPackageManifestFromPackageRoot(packageRoot)
	if err != nil {
		return fmt.Errorf("reading package manifest failed (path: %s): %w", packageRoot, err)
	}

	kibanaClient, err := stack.NewKibanaClientFromProfile(profile, opts...)
	if err != nil {
		return fmt.Errorf("can't create Kibana client: %w", err)
	}

	kibanaVersion, err := kibanaClient.Version()
	if err != nil {
		return fmt.Errorf("can't get Kibana status information: %w", err)
	}

	if kibanaVersion.IsSnapshot() {
		message := fmt.Sprintf("exporting workflows from a SNAPSHOT version of Kibana (%s) is discouraged. It could lead to invalid workflows (for example if they use features that are reverted or modified before the final release)", kibanaVersion.Version())
		if !allowSnapshot {
			return fmt.Errorf("%s. --%s flag can be used to ignore this error", message, cobraext.AllowSnapshotFlagName)
		}
		fmt.Printf("Warning: %s\n", message)
	}

	unmanaged, err := kibanaClient.ListWorkflows(cmd.Context(), kibana.ListWorkflowsOptions{Managed: kibana.WorkflowsUnmanagedOnly})
	if err != nil {
		return fmt.Errorf("listing workflows failed: %w", err)
	}

	if len(workflowIDs) == 0 {
		workflowIDs, err = promptWorkflowIDs(unmanaged)
		if err != nil {
			return fmt.Errorf("selecting workflow IDs failed: %w", err)
		}
		if len(workflowIDs) == 0 {
			fmt.Println("No workflows authored by users were found in Kibana.")
			return nil
		}
	} else if err := checkWorkflowsExportable(workflowIDs, unmanaged); err != nil {
		return err
	}

	exported, err := kibanaClient.ExportWorkflows(cmd.Context(), workflowIDs)
	if err != nil {
		return fmt.Errorf("exporting workflows failed: %w", err)
	}

	workflowsDir := filepath.Join(packageRoot, export.WorkflowsDir)
	existingFiles, err := listWorkflowFiles(workflowsDir)
	if err != nil {
		return err
	}

	files, err := transformExportedWorkflows(manifest.Name, exported, existingFiles)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(workflowsDir, 0755); err != nil {
		return fmt.Errorf("creating workflows directory failed: %w", err)
	}
	for _, file := range files {
		path := filepath.Join(workflowsDir, file.Name)
		if err := os.WriteFile(path, file.Content, 0644); err != nil {
			return fmt.Errorf("writing workflow file %s failed: %w", path, err)
		}
		cmd.Printf("Workflow exported to %s\n", path)
	}

	cmd.Println("Done")
	return nil
}

// checkWorkflowsExportable returns an error if any of the given workflow IDs is not an unmanaged
// workflow, authored by users. Managed workflows, such as Kibana's system workflows or the ones
// installed by Fleet, are read-only and not exported as package assets.
func checkWorkflowsExportable(ids []string, unmanaged []kibana.Workflow) error {
	unmanagedIDs := make(map[string]bool, len(unmanaged))
	for _, workflow := range unmanaged {
		unmanagedIDs[workflow.ID] = true
	}
	for _, id := range ids {
		if !unmanagedIDs[id] {
			return fmt.Errorf("workflow %s not found among the workflows authored by users", id)
		}
	}
	return nil
}

// listWorkflowFiles returns the names of the files in the workflows directory of the package.
// It returns an empty list if the directory doesn't exist.
func listWorkflowFiles(workflowsDir string) ([]string, error) {
	entries, err := os.ReadDir(workflowsDir)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("reading workflows directory failed: %w", err)
	}
	var files []string
	for _, entry := range entries {
		if entry.Type().IsRegular() {
			files = append(files, entry.Name())
		}
	}
	return files, nil
}

func transformExportedWorkflows(pkgName string, exported []kibana.ExportedWorkflow, existingFiles []string) ([]*export.WorkflowFile, error) {
	files := make([]*export.WorkflowFile, 0, len(exported))
	sources := make(map[string]string, len(exported))
	for _, workflow := range exported {
		file, err := export.TransformWorkflow(pkgName, workflow, existingFiles)
		if err != nil {
			return nil, fmt.Errorf("transforming workflow %s failed: %w", workflow.ID, err)
		}
		if other, found := sources[file.Name]; found {
			return nil, fmt.Errorf("workflows %s and %s would be exported to the same file %s, rename one of them", other, workflow.ID, file.Name)
		}
		sources[file.Name] = workflow.ID
		files = append(files, file)
	}
	return files, nil
}

// promptWorkflowIDs prompts the user to select workflows to export among the given ones.
func promptWorkflowIDs(workflows []kibana.Workflow) ([]string, error) {
	if len(workflows) == 0 {
		return nil, nil
	}

	options := make([]string, 0, len(workflows))
	optionIDs := make(map[string]string, len(workflows))
	for _, workflow := range workflows {
		option := workflow.String()
		options = append(options, option)
		optionIDs[option] = workflow.ID
	}

	workflowsPrompt := tui.NewMultiSelect("Which workflows would you like to export?", options, []string{})
	workflowsPrompt.SetPageSize(100)

	var selectedOptions []string
	if err := tui.AskOne(workflowsPrompt, &selectedOptions, tui.Required); err != nil {
		return nil, err
	}

	selected := make([]string, 0, len(selectedOptions))
	for _, option := range selectedOptions {
		selected = append(selected, optionIDs[option])
	}
	return selected, nil
}
