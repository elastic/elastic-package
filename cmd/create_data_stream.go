// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package cmd

import (
	"github.com/AlecAivazis/survey/v2"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"

	"github.com/elastic/elastic-package/internal/packages"
	"github.com/elastic/elastic-package/internal/packages/archetype"
	"github.com/elastic/elastic-package/internal/surveyext"
)

const createDataStreamLongDescription = `Use this command to create a new data stream.

The command can extend the package with a new data stream using embedded data stream template and wizard.`

type newDataStreamAnswers struct {
	Name  string
	Title string
	Type  string
}

func createDataStreamCommandAction(cmd *cobra.Command, args []string) error {
	cmd.Println("Create a new data stream")

	packageRoot, found, err := packages.FindPackageRoot()
	if err != nil {
		return errors.Wrap(err, "locating package root failed")
	}
	if !found {
		return errors.New("package root not found, you can only create new data stream in the package context")
	}

	qs := []*survey.Question{
		{
			Name: "name",
			Prompt: &survey.Input{
				Message: "Data stream name:",
				Default: "new_data_stream",
			},
			Validate: survey.ComposeValidators(survey.Required, surveyext.DataStreamDoesNotExistValidator),
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
		return errors.Wrap(err, "prompt failed")
	}

	descriptor := createDataStreamDescriptorFromAnswers(answers, packageRoot)
	err = archetype.CreateDataStream(descriptor)
	if err != nil {
		return errors.Wrap(err, "can't create new data stream")
	}

	cmd.Println("Done")
	return nil
}

func createDataStreamDescriptorFromAnswers(answers newDataStreamAnswers, packageRoot string) archetype.DataStreamDescriptor {
	return archetype.DataStreamDescriptor{
		Manifest: packages.DataStreamManifest{
			Name:  answers.Name,
			Title: answers.Title,
			Type:  answers.Type,
		},
		PackageRoot: packageRoot,
	}
}
