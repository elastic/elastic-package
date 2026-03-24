// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package cmd

import (
	"errors"
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/Masterminds/semver/v3"
	"github.com/spf13/cobra"

	"github.com/elastic/elastic-package/internal/cobraext"
	"github.com/elastic/elastic-package/internal/packages"
	"github.com/elastic/elastic-package/internal/packages/archetype"
	"github.com/elastic/elastic-package/internal/tui"
)

var semver3_2_0 = semver.MustParse("3.2.0")

const createDataStreamLongDescription = `Use this command to create a new data stream.

The command can extend the package with a new data stream using embedded data stream template and wizard.`

type newDataStreamAnswers struct {
	Name                   string
	Title                  string
	Type                   string
	Inputs                 []string
	Subobjects             bool
	SyntheticAndTimeSeries bool
	Synthetic              bool
}

func createDataStreamCommandAction(cmd *cobra.Command, args []string) error {
	cmd.Println("Create a new data stream")

	if flags := cmd.Flags(); flags.Changed(cobraext.CreateDataStreamNameFlagName) ||
		flags.Changed(cobraext.CreateDataStreamTypeFlagName) ||
		flags.Changed(cobraext.CreateDataStreamInputsFlagName) {
		return createDataStreamNonInteractive(cmd)
	}

	packageRoot, err := packages.FindPackageRoot()
	if err != nil {
		if errors.Is(err, packages.ErrPackageRootNotFound) {
			return errors.New("package root not found, you can only create new data stream in the package context")
		}
		return fmt.Errorf("locating package root failed: %w", err)
	}

	manifest, err := packages.ReadPackageManifestFromPackageRoot(packageRoot)
	if err != nil {
		return fmt.Errorf("reading package manifest failed (path: %s): %w", packageRoot, err)
	}

	if manifest.Type == "input" {
		return fmt.Errorf("data-streams are not supported in input packages")
	}

	sv, err := semver.NewVersion(manifest.SpecVersion)
	if err != nil {
		return fmt.Errorf("failed to obtain spec version from package manifest in \"%s\": %w", packageRoot, err)
	}

	qs := getInitialSurveyQuestionsForVersion(sv)

	var answers newDataStreamAnswers
	err = tui.Ask(qs, &answers)
	if err != nil {
		return fmt.Errorf("prompt failed: %w", err)
	}

	if answers.Type == "metrics" {
		qs := []*tui.Question{
			{
				Name:     "syntheticAndTimeSeries",
				Prompt:   tui.NewConfirm("Enable time series and synthetic source?", true),
				Validate: tui.Required,
			},
		}
		err = tui.Ask(qs, &answers)
		if err != nil {
			return fmt.Errorf("prompt failed: %w", err)
		}

		if !answers.SyntheticAndTimeSeries {
			qs := []*tui.Question{
				{
					Name:     "synthetic",
					Prompt:   tui.NewConfirm("Enable synthetic source?", true),
					Validate: tui.Required,
				},
			}
			err = tui.Ask(qs, &answers)
			if err != nil {
				return fmt.Errorf("prompt failed: %w", err)
			}
		}
	}

	if answers.Type == "logs" {
		multiSelect := tui.NewMultiSelect("Select input types which will be used in this data stream. See https://www.elastic.co/docs/reference/fleet/elastic-agent-inputs-list for description of the inputs", slices.Sorted(maps.Keys(packages.AllowedLogsInputTypes)), []string{})
		multiSelect.SetPageSize(50)
		multiSelect.SetDescription(func(value string, index int) string {
			if label, ok := packages.AllowedLogsInputTypes[value]; ok {
				return label
			}
			return ""
		})

		qs := []*tui.Question{
			{
				Name:   "inputs",
				Prompt: multiSelect,
			},
		}
		err = tui.Ask(qs, &answers)
		if err != nil {
			return fmt.Errorf("prompt failed: %w", err)
		}
	}

	descriptor := createDataStreamDescriptorFromAnswers(answers, packageRoot, sv)
	err = archetype.CreateDataStream(descriptor)
	if err != nil {
		return fmt.Errorf("can't create new data stream: %w", err)
	}

	cmd.Println("Done")
	return nil
}

func createDataStreamNonInteractive(cmd *cobra.Command) error {
	packageRoot, err := packages.FindPackageRoot()
	if err != nil {
		if errors.Is(err, packages.ErrPackageRootNotFound) {
			return errors.New("package root not found, you can only create new data stream in the package context")
		}
		return fmt.Errorf("locating package root failed: %w", err)
	}

	manifest, err := packages.ReadPackageManifestFromPackageRoot(packageRoot)
	if err != nil {
		return fmt.Errorf("reading package manifest failed (path: %s): %w", packageRoot, err)
	}

	if manifest.Type == "input" {
		return fmt.Errorf("data-streams are not supported in input packages")
	}

	sv, err := semver.NewVersion(manifest.SpecVersion)
	if err != nil {
		return fmt.Errorf("failed to obtain spec version from package manifest in \"%s\": %w", packageRoot, err)
	}

	dsName, _ := cmd.Flags().GetString(cobraext.CreateDataStreamNameFlagName)
	dsType, _ := cmd.Flags().GetString(cobraext.CreateDataStreamTypeFlagName)
	inputs, _ := cmd.Flags().GetStringSlice(cobraext.CreateDataStreamInputsFlagName)

	if dsName == "" {
		return fmt.Errorf("--%s is required", cobraext.CreateDataStreamNameFlagName)
	}
	if dsType == "" {
		return fmt.Errorf("--%s is required", cobraext.CreateDataStreamTypeFlagName)
	}
	if !slices.Contains(packages.AllowedDataStreamTypes, dsType) {
		return fmt.Errorf("--%s must be one of: %s", cobraext.CreateDataStreamTypeFlagName, strings.Join(packages.AllowedDataStreamTypes, ", "))
	}

	validator := tui.Validator{Cwd: packageRoot}
	if err := validator.DataStreamDoesNotExist(dsName); err != nil {
		return err
	}
	if err := validator.DataStreamName(dsName); err != nil {
		return err
	}

	if dsType == "logs" {
		if len(inputs) == 0 {
			return fmt.Errorf("--%s is required when type is logs", cobraext.CreateDataStreamInputsFlagName)
		}
		for _, input := range inputs {
			if _, ok := packages.AllowedLogsInputTypes[input]; !ok {
				return fmt.Errorf("invalid input type %q; allowed values: %v", input, slices.Sorted(maps.Keys(packages.AllowedLogsInputTypes)))
			}
		}
	}

	answers := newDataStreamAnswers{
		Name:       dsName,
		Title:      dsName,
		Type:       dsType,
		Inputs:     inputs,
		Subobjects: false,
	}
	if dsType == "metrics" {
		answers.SyntheticAndTimeSeries = true
	}

	descriptor := createDataStreamDescriptorFromAnswers(answers, packageRoot, sv)
	err = archetype.CreateDataStream(descriptor)
	if err != nil {
		return fmt.Errorf("can't create new data stream: %w", err)
	}

	cmd.Println("Done")
	return nil
}

func createDataStreamDescriptorFromAnswers(answers newDataStreamAnswers, packageRoot string, specVersion *semver.Version) archetype.DataStreamDescriptor {
	manifest := packages.DataStreamManifest{
		Name:  answers.Name,
		Title: answers.Title,
		Type:  answers.Type,
	}

	if !specVersion.LessThan(semver3_2_0) && !answers.Subobjects {
		manifest.Elasticsearch = &packages.Elasticsearch{
			IndexTemplate: &packages.ManifestIndexTemplate{
				Mappings: &packages.ManifestMappings{
					Subobjects: false,
				},
			},
		}
	}

	if answers.Synthetic || answers.SyntheticAndTimeSeries {
		if manifest.Elasticsearch == nil {
			manifest.Elasticsearch = &packages.Elasticsearch{}
		}
		manifest.Elasticsearch.SourceMode = "synthetic"
		if answers.SyntheticAndTimeSeries {
			manifest.Elasticsearch.IndexMode = "time_series"
		}
	}

	// If no inputs were selected, insert one so the datastream shows an example of an input.
	if answers.Type == "logs" && len(answers.Inputs) == 0 {
		answers.Inputs = []string{"filestream"}
	}

	if len(answers.Inputs) > 0 {
		var streams []packages.Stream
		for _, input := range answers.Inputs {
			streams = append(streams, packages.Stream{
				Input: input,
				Vars:  []packages.Variable{},
			})
		}
		manifest.Streams = streams
	}

	return archetype.DataStreamDescriptor{
		Manifest:    manifest,
		PackageRoot: packageRoot,
	}
}

func getInitialSurveyQuestionsForVersion(specVersion *semver.Version) []*tui.Question {
	validator := tui.Validator{Cwd: "."}
	qs := []*tui.Question{
		{
			Name:     "name",
			Prompt:   tui.NewInput("Data stream name", "new_data_stream"),
			Validate: tui.ComposeValidators(tui.Required, validator.DataStreamDoesNotExist, validator.DataStreamName),
		},
		{
			Name:     "title",
			Prompt:   tui.NewInput("Data stream title", "New Data Stream"),
			Validate: tui.Required,
		},
		{
			Name:     "type",
			Prompt:   tui.NewSelect("Type", packages.AllowedDataStreamTypes, "logs"),
			Validate: tui.Required,
		},
	}

	if !specVersion.LessThan(semver3_2_0) {
		qs = append(qs, &tui.Question{
			Name:     "subobjects",
			Prompt:   tui.NewConfirm("Enable creation of subobjects for fields with dots in their names?", false),
			Validate: tui.Required,
		})
	}

	return qs
}
