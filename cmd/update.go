// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/elastic/elastic-package/internal/cobraext"
	"github.com/elastic/elastic-package/internal/install"
)

const updateLongDescription = `Use this command to update package resources.

The command can help update existing resources in a package. Currently only documentation is supported.`

func setupUpdateCommand() *cobraext.Command {
	updateDocumentationCmd := &cobra.Command{
		Use:   "documentation",
		Short: "Update package documentation",
		Long:  updateDocumentationLongDescription,
		Args:  cobra.NoArgs,
		RunE:  updateDocumentationCommandAction,
	}
	updateDocumentationCmd.Flags().Bool("non-interactive", false, "run in non-interactive mode, accepting the first result from the LLM")
	updateDocumentationCmd.Flags().String("modify-prompt", "", "modification instructions for targeted documentation changes (skips full rewrite)")
	updateDocumentationCmd.Flags().String("doc-file", "", "specify which markdown file to update (e.g., README.md, vpc.md). Defaults to README.md")

	// Evaluation mode flags
	updateDocumentationCmd.Flags().Bool("evaluate", false, "run in evaluation mode - outputs to directory instead of package, computes quality metrics")
	updateDocumentationCmd.Flags().String("evaluate-output-dir", "./doc_eval_results", "directory for evaluation results (used with --evaluate)")
	updateDocumentationCmd.Flags().Uint("evaluate-max-iterations", 3, "maximum generation iterations per section (evaluation mode only)")

	cmd := &cobra.Command{
		Use:   "update",
		Short: "Update package resources",
		Long:  updateLongDescription,
	}
	cmd.AddCommand(updateDocumentationCmd)
	cmd.PersistentFlags().StringP(cobraext.ProfileFlagName, "p", "", fmt.Sprintf(cobraext.ProfileFlagDescription, install.ProfileNameEnvVar))

	return cobraext.NewCommand(cmd, cobraext.ContextGlobal)
}
