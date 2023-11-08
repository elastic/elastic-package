// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package cmd

import (
	"errors"
	"fmt"

	"github.com/AlecAivazis/survey/v2"

	"github.com/spf13/cobra"

	"github.com/elastic/elastic-package/internal/surveyext"
	"github.com/elastic/elastic-package/pkg/packages"
	"github.com/elastic/elastic-package/pkg/packages/archetype"
)

const createDataStreamLongDescription = `Use this command to create a new data stream.

The command can extend the package with a new data stream using embedded data stream template and wizard.`

type newDataStreamAnswers struct {
	Name                   string
	Title                  string
	Type                   string
	SyntheticAndTimeSeries bool
	Synthetic              bool
}

func createDataStreamCommandAction(cmd *cobra.Command, args []string) error {
	cmd.Println("Create a new data stream")

	packageRoot, found, err := packages.FindPackageRoot()
	if err != nil {
		return fmt.Errorf("locating package root failed: %w", err)
	}
	if !found {
		return errors.New("package root not found, you can only create new data stream in the package context")
	}

	manifest, err := packages.ReadPackageManifestFromPackageRoot(packageRoot)
	if err != nil {
		return fmt.Errorf("reading package manifest failed (path: %s): %w", packageRoot, err)
	}

	if manifest.Type == "input" {
		return fmt.Errorf("data-streams are not supported in input packages")
	}

	qs := []*survey.Question{
		{
			Name: "name",
			Prompt: &survey.Input{
				Message: "Data stream name:",
				Default: "new_data_stream",
			},
			Validate: survey.ComposeValidators(survey.Required, surveyext.DataStreamDoesNotExistValidator, surveyext.DataStreamNameValidator),
		},
		{
			Name: "title",
			Prompt: &survey.Input{
				Message: "Data stream title:",
				Default: "New Data Stream",
			},
			Validate: survey.Required,
		},
		{
			Name: "type",
			Prompt: &survey.Select{
				Message: "Type:",
				Options: []string{"logs", "metrics"},
				Default: "logs",
			},
			Validate: survey.Required,
		},
	}
	var answers newDataStreamAnswers
	err = survey.Ask(qs, &answers)
	if err != nil {
		return fmt.Errorf("prompt failed: %w", err)
	}

	if answers.Type == "metrics" {
		qs := []*survey.Question{
			{
				Name: "syntheticAndTimeSeries",
				Prompt: &survey.Confirm{
					Message: "Enable time series and synthetic source?",
					Default: true,
				},
				Validate: survey.Required,
			},
		}
		err = survey.Ask(qs, &answers)
		if err != nil {
			return fmt.Errorf("prompt failed: %w", err)
		}

		if !answers.SyntheticAndTimeSeries {
			qs := []*survey.Question{
				{
					Name: "synthetic",
					Prompt: &survey.Confirm{
						Message: "Enable synthetic source?",
						Default: true,
					},
					Validate: survey.Required,
				},
			}
			err = survey.Ask(qs, &answers)
			if err != nil {
				return fmt.Errorf("prompt failed: %w", err)
			}
		}
	}

	descriptor := createDataStreamDescriptorFromAnswers(answers, packageRoot)
	err = archetype.CreateDataStream(descriptor)
	if err != nil {
		return fmt.Errorf("can't create new data stream: %w", err)
	}

	cmd.Println("Done")
	return nil
}

func createDataStreamDescriptorFromAnswers(answers newDataStreamAnswers, packageRoot string) archetype.DataStreamDescriptor {
	manifest := packages.DataStreamManifest{
		Name:  answers.Name,
		Title: answers.Title,
		Type:  answers.Type,
	}

	if !answers.SyntheticAndTimeSeries && !answers.Synthetic {
		return archetype.DataStreamDescriptor{
			Manifest:    manifest,
			PackageRoot: packageRoot,
		}
	}
	elasticsearch := packages.Elasticsearch{
		SourceMode: "synthetic",
	}
	if answers.SyntheticAndTimeSeries {
		elasticsearch.IndexMode = "time_series"
	}
	manifest.Elasticsearch = &elasticsearch
	return archetype.DataStreamDescriptor{
		Manifest:    manifest,
		PackageRoot: packageRoot,
	}
}
