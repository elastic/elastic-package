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

const createPackageLongDescription = `Use this command to create a new package.

The command can bootstrap the first draft of a package using embedded package template and wizard.`

type newPackageAnswers struct {
	Name          string
	Version       string
	Title         string
	Description   string
	Categories    []string
	KibanaVersion string `survey:"kibana_version"`
	GithubOwner   string `survey:"github_owner"`
}

func createPackageCommandAction(cmd *cobra.Command, args []string) error {
	cmd.Println("Create a new package")

	qs := []*survey.Question{
		{
			Name: "name",
			Prompt: &survey.Input{
				Message: "Package name:",
				Default: "new_package",
			},
			Validate: survey.ComposeValidators(survey.Required, surveyext.PackageDoesNotExistValidator),
		},
		{
			Name: "version",
			Prompt: &survey.Input{
				Message: "Version:",
				Default: "0.0.1",
			},
			Validate: survey.ComposeValidators(survey.Required, surveyext.SemverValidator),
		},
		{
			Name: "title",
			Prompt: &survey.Input{
				Message: "Package title:",
				Default: "New Package",
			},
			Validate: survey.Required,
		},
		{
			Name: "description",
			Prompt: &survey.Input{
				Message: "Description:",
				Default: "This is a new package.",
			},
			Validate: survey.Required,
		},
		{
			Name: "categories",
			Prompt: &survey.MultiSelect{
				Message: "Categories:",
				Options: []string{"aws", "azure", "cloud", "config_management", "containers", "crm", "custom",
					"datastore", "elastic_stack", "google_cloud", "kubernetes", "languages", "message_queue",
					"monitoring", "network", "notification", "os_system", "productivity", "security", "support",
					"ticketing", "version_control", "web"},
				Default:  []string{"custom"},
				PageSize: 50,
			},
			Validate: survey.Required,
		},
		{
			Name: "kibana_version",
			Prompt: &survey.Input{
				Message: "Kibana version constraint:",
				Default: surveyext.DefaultConstraintValue(),
			},
			Validate: survey.ComposeValidators(survey.Required, surveyext.ConstraintValidator),
		},
		{
			Name: "github_owner",
			Prompt: &survey.Input{
				Message: "Github owner:",
				Default: "elastic/integrations",
			},
			Validate: survey.ComposeValidators(survey.Required, surveyext.GithubOwnerValidator),
		},
	}

	var answers newPackageAnswers
	err := survey.Ask(qs, &answers)
	if err != nil {
		return errors.Wrap(err, "prompt failed")
	}

	descriptor := createPackageDescriptorFromAnswers(answers)
	err = archetype.CreatePackage(descriptor)
	if err != nil {
		return errors.Wrap(err, "can't create new package")
	}

	cmd.Println("Done")
	return nil
}

func createPackageDescriptorFromAnswers(answers newPackageAnswers) archetype.PackageDescriptor {
	return archetype.PackageDescriptor{
		Manifest: packages.PackageManifest{
			Name:    answers.Name,
			Title:   answers.Title,
			Type:    "integration",
			Version: answers.Version,
			Conditions: packages.Conditions{
				Kibana: packages.KibanaConditions{
					Version: answers.KibanaVersion,
				},
			},
			Owner: packages.Owner{
				Github: answers.GithubOwner,
			},
			Description: answers.Description,
			License:     "basic",
			Categories:  answers.Categories,
		},
	}
}
