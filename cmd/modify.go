// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package cmd

import (
	"fmt"
	"io"
	"sort"
	"text/tabwriter"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/elastic/elastic-package/internal/cobraext"
	"github.com/elastic/elastic-package/internal/fleetpkg"
	"github.com/elastic/elastic-package/internal/modify"
	"github.com/elastic/elastic-package/internal/modify/pipelinetag"
	"github.com/elastic/elastic-package/internal/packages"
)

const modifyLongDescription = `Use this command to apply modifications to a package.

These modifications can range from applying best practices, generating ingest pipeline tags, and more. Run this command without any arguments to see a list of modifiers.

Use --modifiers to specify which modifiers to run, separated by commas.
`

func setupModifyCommand() *cobraext.Command {
	modifiers := []*modify.Modifier{
		pipelinetag.Modifier,
	}
	sort.Slice(modifiers, func(i, j int) bool {
		return modifiers[i].Name < modifiers[j].Name
	})

	validModifier := func(name string) bool {
		for _, modifier := range modifiers {
			if modifier.Name == name {
				return true
			}
		}

		return false
	}

	listModifiers := func(w io.Writer) {
		tw := tabwriter.NewWriter(w, 0, 2, 3, ' ', 0)
		for _, a := range modifiers {
			_, _ = fmt.Fprintf(tw, "%s\t%s\n", a.Name, a.Doc)
		}
		_ = tw.Flush()
		_, _ = fmt.Fprintln(w, "")
	}

	cmd := &cobra.Command{
		Use:   "modify",
		Short: "Modify package assets",
		Long:  modifyLongDescription,
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.Println("Modify package assets")

			selectedModifiers, err := cmd.Flags().GetStringSlice("modifiers")
			if err != nil {
				return cobraext.FlagParsingError(err, "modifiers")
			}
			if len(selectedModifiers) == 0 {
				_, _ = fmt.Fprint(cmd.OutOrStderr(), "Please provide at least one modifier:\n\n")
				listModifiers(cmd.OutOrStderr())
				return nil
			}
			for _, selected := range selectedModifiers {
				if !validModifier(selected) {
					_, _ = fmt.Fprint(cmd.OutOrStderr(), "Please provide at a valid modifier:\n\n")
					listModifiers(cmd.OutOrStderr())
					return cobraext.FlagParsingError(fmt.Errorf("invalid modifier: %q", selected), "modifiers")
				}
			}

			pkgRootPath, err := packages.FindPackageRoot()
			if err != nil {
				return fmt.Errorf("locating package root failed: %w", err)
			}

			for _, modifier := range modifiers {
				pkg, err := fleetpkg.Load(pkgRootPath)
				if err != nil {
					return fmt.Errorf("failed to load package from %q: %w", pkgRootPath, err)
				}
				if err = modifier.Run(pkg); err != nil {
					return fmt.Errorf("failed to apply modifier %q: %w", modifier.Name, err)
				}
			}

			return nil
		},
	}

	cmd.PersistentFlags().StringSliceP("modifiers", "m", nil, "List of modifiers to run, separated by commas")

	for _, m := range modifiers {
		prefix := m.Name + "."

		m.Flags.VisitAll(func(f *pflag.Flag) {
			name := prefix + f.Name
			cmd.Flags().Var(f.Value, name, f.Usage)
		})
	}

	return cobraext.NewCommand(cmd, cobraext.ContextPackage)
}
