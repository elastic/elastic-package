// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package cmd

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"strings"

	"github.com/spf13/cobra"

	"github.com/elastic/elastic-package/internal/cobraext"
	"github.com/elastic/elastic-package/internal/common"
	"github.com/elastic/elastic-package/internal/kibana"
	"github.com/elastic/elastic-package/internal/packages"
	"github.com/elastic/elastic-package/internal/stack"
	"github.com/elastic/elastic-package/internal/tui"
)

const editWorkflowsLongDescription = `Use this command to make workflows editable.

Pass a comma-separated list of workflow ids with -d or use the interactive prompt to make managed workflows editable in Kibana.

Workflows installed by Fleet are read-only, so this command creates an editable copy of each one, with the name of its file in the package as ID. Export the copy with elastic-package export workflows to update that file.`

// fleetWorkflowIDPattern matches the IDs of the workflows installed by Fleet, which have the form
// "system-fleet-package-workflow-{hash(space)}-{hash(package)}-{file base}". The file base is
// lowercased and has "_" replaced by "-". The hash length is fixed, so a file base starting with
// hexadecimal characters can't be mistaken for a hash.
var fleetWorkflowIDPattern = regexp.MustCompile(`^system-fleet-package-workflow-[0-9a-f]{16}-[0-9a-f]{16}-([a-z0-9][a-z0-9-]*)$`)

func editWorkflowsCmd(cmd *cobra.Command, args []string) error {
	cmd.Println("Make Kibana workflows editable")

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

	kibanaClient, err := stack.NewKibanaClientFromProfile(profile, opts...)
	if err != nil {
		return fmt.Errorf("can't create Kibana client: %w", err)
	}

	kibanaVersion, err := kibanaClient.Version()
	if err != nil {
		return fmt.Errorf("can't get Kibana status information: %w", err)
	}

	if kibanaVersion.IsSnapshot() {
		message := fmt.Sprintf("editing workflows from a SNAPSHOT version of Kibana (%s) is discouraged. It could lead to invalid workflows (for example if they use features that are reverted or modified before the final release)", kibanaVersion.Version())
		if !allowSnapshot {
			return fmt.Errorf("%s. --%s flag can be used to ignore this error", message, cobraext.AllowSnapshotFlagName)
		}
		fmt.Printf("Warning: %s\n", message)
	}

	if len(workflowIDs) == 0 {
		// Not mandatory to get the package name here, but it would be helpful for users
		// to select by default the package where they are located if any.
		defaultPackage := ""
		packageRoot, err := packages.MustFindPackageRoot()
		if err == nil {
			m, err := packages.ReadPackageManifestFromPackageRoot(packageRoot)
			if err != nil {
				return fmt.Errorf("reading package manifest failed (path: %s): %w", packageRoot, err)
			}
			defaultPackage = m.Name
		}
		workflowIDs, err = promptPackageWorkflowIDs(cmd.Context(), kibanaClient, defaultPackage)
		if err != nil {
			return fmt.Errorf("selecting workflow IDs failed: %w", err)
		}

		if len(workflowIDs) == 0 {
			fmt.Println("No workflows were found in Kibana.")
			return nil
		}
	}

	editableWorkflowIDs := make([]string, 0, len(workflowIDs))
	var failedWorkflowUpdates []error
	for _, workflowID := range workflowIDs {
		editableID, created, err := makeWorkflowEditable(cmd.Context(), kibanaClient, workflowID)
		if err != nil {
			failedWorkflowUpdates = append(failedWorkflowUpdates, err)
			continue
		}
		if !created {
			cmd.Printf("Workflow %s was already editable as %s, it was left unchanged.\n", workflowID, editableID)
		}
		editableWorkflowIDs = append(editableWorkflowIDs, editableID)
	}

	if len(editableWorkflowIDs) > 0 {
		urls, err := workflowURLs(kibanaClient.Address(), editableWorkflowIDs)
		if err != nil {
			cmd.Println(fmt.Sprintf("\nFailed to retrieve workflow URLS: %s", err.Error()))
			cmd.Println(fmt.Sprintf("The following workflows are now editable in Kibana:\n%s", strings.Join(editableWorkflowIDs, "\n")))
		} else {
			cmd.Println(fmt.Sprintf("\nThe following workflows are now editable in Kibana:%s\n\nRemember to export modified workflows with elastic-package export workflows", urls))
		}
	}

	if len(failedWorkflowUpdates) > 0 {
		fmt.Println("")
		return fmt.Errorf("failed to make one or more workflows editable: %w", errors.Join(failedWorkflowUpdates...))
	}

	fmt.Println("\nDone")
	return nil
}

// editableWorkflowID returns the ID of the editable copy of a workflow installed by Fleet. It is
// the file base of the workflow in the package, as included in the installed ID.
func editableWorkflowID(installedID string) (string, error) {
	matches := fleetWorkflowIDPattern.FindStringSubmatch(installedID)
	if matches == nil {
		return "", fmt.Errorf("workflow %s was not installed by Fleet from a package", installedID)
	}
	return matches[1], nil
}

// makeWorkflowEditable creates an editable copy of the given workflow installed by Fleet, and
// returns its ID. If the copy already exists, it is left unchanged, as it may contain edits that
// haven't been exported yet, and created is false.
func makeWorkflowEditable(ctx context.Context, kibanaClient *kibana.Client, installedID string) (editableID string, created bool, err error) {
	editableID, err = editableWorkflowID(installedID)
	if err != nil {
		return "", false, err
	}

	_, found, err := kibanaClient.GetWorkflow(ctx, editableID)
	if err != nil {
		return "", false, err
	}
	if found {
		return editableID, false, nil
	}

	exported, err := kibanaClient.ExportWorkflows(ctx, []string{installedID})
	if err != nil {
		return "", false, fmt.Errorf("exporting workflow %s failed: %w", installedID, err)
	}

	if _, err := kibanaClient.CreateWorkflow(ctx, editableID, exported[0].YAML); err != nil {
		return "", false, err
	}
	return editableID, true, nil
}

// promptPackageWorkflowIDs prompts the user to select an installed package, and then the workflows
// installed by Fleet for that package. It returns the IDs of the selected workflows.
func promptPackageWorkflowIDs(ctx context.Context, kibanaClient *kibana.Client, defaultPackage string) ([]string, error) {
	installedPackage, err := promptPackagesInstalled(ctx, kibanaClient, defaultPackage, "Which package would you like to edit workflows from?")
	if err != nil {
		return nil, fmt.Errorf("prompt for package selection failed: %w", err)
	}
	if installedPackage == "" {
		fmt.Println("No installed packages were found in Kibana.")
		return nil, nil
	}

	// As it can be installed just one version of a package in Elastic, we can split by '-' to get the name.
	packageName, _, found := strings.Cut(installedPackage, "-")
	if !found {
		return nil, fmt.Errorf("invalid package name: %s", installedPackage)
	}

	pkg, err := kibanaClient.GetPackage(ctx, packageName)
	if err != nil {
		return nil, fmt.Errorf("failed to get package status: %w", err)
	}
	if pkg.Status == "not_installed" || pkg.InstallationInfo == nil {
		return nil, fmt.Errorf("package %s is not installed", packageName)
	}

	var workflows []kibana.Workflow
	for _, asset := range pkg.InstallationInfo.InstalledKibanaAssets {
		if asset.Type != "workflow" {
			continue
		}
		workflow, found, err := kibanaClient.GetWorkflow(ctx, asset.ID)
		if err != nil {
			return nil, err
		}
		if !found {
			// Fleet reports workflows it skipped as deferred, but doesn't install them.
			fmt.Printf("Warning: workflow %s of package %s is not installed in Kibana, check that Fleet workflow assets are enabled (stack.fleet_workflow_assets_enabled) and that the license includes Workflows\n", asset.ID, packageName)
			continue
		}
		workflows = append(workflows, workflow)
	}
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

	workflowsPrompt := tui.NewMultiSelect("Which workflows would you like to make editable?", options, []string{})
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

func workflowURLs(kibanaHost string, workflowIDs []string) (string, error) {
	kibanaURL, err := url.Parse(kibanaHost)
	if err != nil {
		return "", fmt.Errorf("failed to retrieve Kibana URL: %w", err)
	}
	var urls strings.Builder
	for _, workflowID := range workflowIDs {
		fmt.Fprintf(&urls, "\n%s", kibanaURL.JoinPath("app", "workflows", workflowID).String())
	}
	return urls.String(), nil
}
