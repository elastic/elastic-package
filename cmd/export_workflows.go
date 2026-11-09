// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package cmd

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
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

Use this command to download selected workflows from Kibana and store them as workflow assets of the package, in the kibana/workflow directory. Workflows are stored disabled, as required for package assets, in files named after the package and the workflow name.`

func exportWorkflowsCmd(cmd *cobra.Command, args []string) error {
	cmd.Println("Export Kibana workflows")

	workflowIDs, err := cmd.Flags().GetStringSlice(cobraext.WorkflowIDsFlagName)
	if err != nil {
		return cobraext.FlagParsingError(err, cobraext.WorkflowIDsFlagName)
	}
	common.TrimStringSlice(workflowIDs)

	includeManaged, err := cmd.Flags().GetBool(cobraext.WorkflowsManagedFlagName)
	if err != nil {
		return cobraext.FlagParsingError(err, cobraext.WorkflowsManagedFlagName)
	}

	force, err := cmd.Flags().GetBool(cobraext.WorkflowsForceFlagName)
	if err != nil {
		return cobraext.FlagParsingError(err, cobraext.WorkflowsForceFlagName)
	}

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

	var installedIDs map[string]bool
	if len(workflowIDs) > 0 || includeManaged {
		installedIDs, err = kibanaClient.InstalledPackageWorkflowIDs(cmd.Context(), manifest.Name)
		if err != nil {
			return fmt.Errorf("getting workflows installed for package %s failed: %w", manifest.Name, err)
		}
	}

	if len(workflowIDs) == 0 {
		workflowIDs, err = promptWorkflowIDs(cmd.Context(), kibanaClient, includeManaged, installedIDs)
		if err != nil {
			return fmt.Errorf("selecting workflow IDs failed: %w", err)
		}
		if len(workflowIDs) == 0 {
			fmt.Println(noWorkflowsFoundMessage(includeManaged))
			return nil
		}
	} else {
		unmanaged, err := kibanaClient.ListWorkflows(cmd.Context(), kibana.ListWorkflowsOptions{Managed: kibana.WorkflowsUnmanagedOnly})
		if err != nil {
			return fmt.Errorf("listing workflows failed: %w", err)
		}
		if err := checkWorkflowsExportable(workflowIDs, unmanaged, installedIDs, manifest.Name); err != nil {
			return err
		}
	}

	exported, err := kibanaClient.ExportWorkflows(cmd.Context(), workflowIDs)
	if err != nil {
		return fmt.Errorf("exporting workflows failed: %w", err)
	}

	files, err := transformExportedWorkflows(manifest.Name, exported)
	if err != nil {
		return err
	}

	workflowsDir := filepath.Join(packageRoot, export.WorkflowsDir)
	if !force {
		if err := checkWorkflowFilesNotExist(workflowsDir, files); err != nil {
			return err
		}
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

// checkWorkflowsExportable returns an error if any of the given workflow IDs is neither an unmanaged
// workflow, authored by users, nor a workflow installed by Fleet for the current package. This applies
// the same rule as selectableWorkflows to explicitly requested IDs, so managed workflows such as
// Kibana's system workflows, or the ones installed for other packages, are not exported as assets of
// this package. Only unmanaged workflows are listed, as listing managed ones requires extra privileges.
func checkWorkflowsExportable(ids []string, unmanaged []kibana.Workflow, installedIDs map[string]bool, pkgName string) error {
	unmanagedIDs := make(map[string]bool, len(unmanaged))
	for _, workflow := range unmanaged {
		unmanagedIDs[workflow.ID] = true
	}
	for _, id := range ids {
		if !unmanagedIDs[id] && !installedIDs[id] {
			return fmt.Errorf("workflow %s not found among the workflows authored by users, nor among the ones installed by Fleet for package %s", id, pkgName)
		}
	}
	return nil
}

func transformExportedWorkflows(pkgName string, exported []kibana.ExportedWorkflow) ([]*export.WorkflowFile, error) {
	files := make([]*export.WorkflowFile, 0, len(exported))
	sources := make(map[string]string, len(exported))
	for _, workflow := range exported {
		file, err := export.TransformWorkflow(pkgName, workflow)
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

func checkWorkflowFilesNotExist(workflowsDir string, files []*export.WorkflowFile) error {
	for _, file := range files {
		path := filepath.Join(workflowsDir, file.Name)
		_, err := os.Stat(path)
		if err == nil {
			return fmt.Errorf("workflow file %s already exists, use --%s to overwrite it", path, cobraext.WorkflowsForceFlagName)
		}
		if !errors.Is(err, fs.ErrNotExist) {
			return fmt.Errorf("checking workflow file %s failed: %w", path, err)
		}
	}
	return nil
}

// selectableWorkflows returns the workflows that can be exported to the current package: the
// unmanaged ones, authored by users, and the ones installed by Fleet for this package. Other
// managed workflows, such as Kibana's own system workflows, are excluded.
func selectableWorkflows(workflows []kibana.Workflow, installedIDs map[string]bool) []kibana.Workflow {
	var selectable []kibana.Workflow
	for _, workflow := range workflows {
		if !workflow.Managed || installedIDs[workflow.ID] {
			selectable = append(selectable, workflow)
		}
	}
	return selectable
}

func noWorkflowsFoundMessage(includeManaged bool) string {
	if includeManaged {
		return "No workflows were found in Kibana."
	}
	return fmt.Sprintf("No workflows authored by users were found in Kibana. Use --%s to include the workflows installed by Fleet for this package.", cobraext.WorkflowsManagedFlagName)
}

// promptWorkflowIDs prompts the user to select workflows to export. By default it only lists
// unmanaged workflows, the ones authored by users. If includeManaged is set, it also lists the
// workflows installed by Fleet for the current package.
func promptWorkflowIDs(ctx context.Context, kibanaClient *kibana.Client, includeManaged bool, installedIDs map[string]bool) ([]string, error) {
	listOptions := kibana.ListWorkflowsOptions{Managed: kibana.WorkflowsUnmanagedOnly}
	if includeManaged {
		listOptions.Managed = kibana.WorkflowsManagedAll
	}

	workflows, err := kibanaClient.ListWorkflows(ctx, listOptions)
	if err != nil {
		return nil, fmt.Errorf("listing workflows failed: %w", err)
	}

	workflows = selectableWorkflows(workflows, installedIDs)
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
