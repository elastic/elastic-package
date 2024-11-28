// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package cmd

import (
	"errors"
	"fmt"

	"github.com/spf13/cobra"
	"go.opentelemetry.io/otel/trace"

	"github.com/elastic/elastic-package/internal/cobraext"
	"github.com/elastic/elastic-package/internal/install"
	"github.com/elastic/elastic-package/internal/kibana"
	"github.com/elastic/elastic-package/internal/packages"
	"github.com/elastic/elastic-package/internal/packages/installer"
	"github.com/elastic/elastic-package/internal/stack"
	"github.com/elastic/elastic-package/internal/telemetry"
)

const installLongDescription = `Use this command to install the package in Kibana.

The command uses Kibana API to install the package in Kibana. The package must be exposed via the Package Registry or built locally in zip format so they can be installed using --zip parameter. Zip packages can be installed directly in Kibana >= 8.7.0. More details in this [HOWTO guide](https://github.com/elastic/elastic-package/blob/main/docs/howto/install_package.md).`

func setupInstallCommand() *cobraext.Command {
	cmd := &cobra.Command{
		Use:   "install",
		Short: "Install the package",
		Long:  installLongDescription,
		Args:  cobra.NoArgs,
		RunE:  installCommandAction,
	}
	cmd.Flags().StringSliceP(cobraext.CheckConditionFlagName, "c", nil, cobraext.CheckConditionFlagDescription)
	cmd.Flags().StringP(cobraext.PackageRootFlagName, cobraext.PackageRootFlagShorthand, "", cobraext.PackageRootFlagDescription)
	cmd.Flags().StringP(cobraext.ZipPackageFilePathFlagName, cobraext.ZipPackageFilePathFlagShorthand, "", cobraext.ZipPackageFilePathFlagDescription)
	cmd.Flags().Bool(cobraext.BuildSkipValidationFlagName, false, cobraext.BuildSkipValidationFlagDescription)
	cmd.Flags().StringP(cobraext.ProfileFlagName, "p", "", fmt.Sprintf(cobraext.ProfileFlagDescription, install.ProfileNameEnvVar))
	cmd.Flags().Bool(cobraext.TLSSkipVerifyFlagName, false, cobraext.TLSSkipVerifyFlagDescription)

	return cobraext.NewCommand(cmd, cobraext.ContextPackage)
}

func installCommandAction(cmd *cobra.Command, _ []string) error {
	globalCtx, span := telemetry.StartSpanForCommand(telemetry.CmdTracer, cmd)
	defer span.End()

	zipPathFile, err := cmd.Flags().GetString(cobraext.ZipPackageFilePathFlagName)
	if err != nil {
		return cobraext.FlagParsingError(err, cobraext.ZipPackageFilePathFlagName)
	}
	packageRootPath, err := cmd.Flags().GetString(cobraext.PackageRootFlagName)
	if err != nil {
		return cobraext.FlagParsingError(err, cobraext.PackageRootFlagName)
	}
	skipValidation, err := cmd.Flags().GetBool(cobraext.BuildSkipValidationFlagName)
	if err != nil {
		return cobraext.FlagParsingError(err, cobraext.BuildSkipValidationFlagName)
	}

	profile, err := cobraext.GetProfileFlag(cmd)
	if err != nil {
		return err
	}

	var opts []kibana.ClientOption
	tlsSkipVerify, _ := cmd.Flags().GetBool(cobraext.TLSSkipVerifyFlagName)
	if tlsSkipVerify {
		opts = append(opts, kibana.TLSSkipVerify())
	}

	kibanaClient, err := stack.NewKibanaClientFromProfile(profile, opts...)
	if err != nil {
		return fmt.Errorf("could not create kibana client: %w", err)
	}

	if zipPathFile == "" && packageRootPath == "" {
		var found bool
		var err error
		packageRootPath, found, err = packages.FindPackageRoot()
		if !found {
			return errors.New("package root not found")
		}
		if err != nil {
			return fmt.Errorf("locating package root failed: %w", err)
		}
	}

	installer, err := installer.NewForPackage(installer.Options{
		Kibana:         kibanaClient,
		RootPath:       packageRootPath,
		SkipValidation: skipValidation,
		ZipPath:        zipPathFile,
	})
	if err != nil {
		return fmt.Errorf("package installation failed: %w", err)
	}

	// Check conditions
	keyValuePairs, err := cmd.Flags().GetStringSlice(cobraext.CheckConditionFlagName)
	if err != nil {
		return fmt.Errorf("can't process check-condition flag: %w", err)
	}
	manifest, err := installer.Manifest(globalCtx)
	if err != nil {
		return err
	}
	if len(keyValuePairs) > 0 {
		// Not used context returned by Start
		_, checkSpan := telemetry.CmdTracer.Start(globalCtx, "Check conditions",
			trace.WithAttributes(
				telemetry.AttributeKeyPackageName.String(manifest.Name),
				telemetry.AttributeKeyPackageVersion.String(manifest.Version),
				telemetry.AttributeKeyPackageVersion.String(manifest.SpecVersion),
			),
		)

		cmd.Println("Check conditions for package")
		err = packages.CheckConditions(*manifest, keyValuePairs)
		if err != nil {
			return fmt.Errorf("checking conditions failed: %w", err)
		}
		checkSpan.End()
		cmd.Println("Requirements satisfied - the package can be installed.")
		cmd.Println("Done")
		return nil
	}

	ctx, installSpan := telemetry.CmdTracer.Start(globalCtx, "Install package",
		trace.WithAttributes(
			telemetry.AttributeKeyPackageName.String(manifest.Name),
			telemetry.AttributeKeyPackageVersion.String(manifest.Version),
			telemetry.AttributeKeyPackageVersion.String(manifest.SpecVersion),
		),
	)
	_, err = installer.Install(ctx)
	installSpan.End()
	return err
}
