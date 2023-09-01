// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package cmd

import (
	"fmt"

	"github.com/AlecAivazis/survey/v2"
	"github.com/spf13/cobra"

	"github.com/elastic/elastic-package/internal/licenses"
	"github.com/elastic/elastic-package/internal/packages"
	"github.com/elastic/elastic-package/internal/packages/archetype"
	"github.com/elastic/elastic-package/internal/surveyext"
)

const createPackageLongDescription = `Use this command to create a new package.

The command can bootstrap the first draft of a package using embedded package template and wizard.`

const (
	noLicenseValue             = "None"
	noLicenseOnCreationMessage = "I will add a license later."
)

type newPackageAnswers struct {
	Name                string
	Type                string
	Version             string
	SourceLicense       string `survey:"source_license"`
	Title               string
	Description         string
	Categories          []string
	KibanaVersion       string `survey:"kibana_version"`
	ElasticSubscription string `survey:"elastic_subscription"`
	GithubOwner         string `survey:"github_owner"`
	DataStreamType      string `survey:"datastream_type"`
}

func createPackageCommandAction(cmd *cobra.Command, args []string) error {
	cmd.Println("Create a new package")

	qs := []*survey.Question{
		{
			Name: "type",
			Prompt: &survey.Select{
				Message: "Package type:",
				Options: []string{"input", "integration"},
				Default: "integration",
			},
			Validate: survey.Required,
		},
	}

	var answers newPackageAnswers
	err := survey.Ask(qs, &answers)
	if err != nil {
		return fmt.Errorf("prompt failed: %w", err)
	}

	qs = []*survey.Question{
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
			Name: "source_license",
			Prompt: &survey.Select{
				Message: "License:",
				Options: []string{
					licenses.Elastic20,
					licenses.Apache20,
					noLicenseValue,
				},
				Description: func(value string, _ int) string {
					if value == noLicenseValue {
						return noLicenseOnCreationMessage
					}
					return ""
				},
				Default: licenses.Elastic20,
			},
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
				Default: surveyext.DefaultKibanaVersionConditionValue(),
			},
			Validate: survey.ComposeValidators(survey.Required, surveyext.ConstraintValidator),
		},
		{
			Name: "elastic_subscription",
			Prompt: &survey.Select{
				Message: "Required Elastic subscription:",
				Options: []string{"basic", "gold", "platinum", "enterprise"},
				Default: "basic",
			},
			Validate: survey.Required,
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

	if answers.Type == "input" {
		inputQs := []*survey.Question{
			{
				Name: "datastream_type",
				Prompt: &survey.Select{
					Message: "Input Data Stream type:",
					Options: []string{"logs", "metrics"},
					Default: "logs",
				},
				Validate: survey.Required,
			},
		}

		qs = append(qs, inputQs...)
	}

	err = survey.Ask(qs, &answers)
	if err != nil {
		return fmt.Errorf("prompt failed: %w", err)
	}

	descriptor := createPackageDescriptorFromAnswers(answers)
	specVersion, err := archetype.GetLatestStableSpecVersion()
	if err != nil {
		return fmt.Errorf("failed to get spec version: %w", err)
	}
	descriptor.Manifest.SpecVersion = specVersion.String()

	err = archetype.CreatePackage(descriptor)
	if err != nil {
		return fmt.Errorf("can't create new package: %w", err)
	}

	cmd.Println("Done")
	return nil
}

func createPackageDescriptorFromAnswers(answers newPackageAnswers) archetype.PackageDescriptor {
	sourceLicense := ""
	if answers.SourceLicense != noLicenseValue {
		sourceLicense = answers.SourceLicense
	}

	inputDataStreamType := ""
	if answers.Type == "input" {
		inputDataStreamType = answers.DataStreamType
	}
	return archetype.PackageDescriptor{
		Manifest: packages.PackageManifest{
			Name:    answers.Name,
			Title:   answers.Title,
			Type:    answers.Type,
			Version: answers.Version,
			Source: packages.Source{
				License: sourceLicense,
			},
			Conditions: packages.Conditions{
				Kibana: packages.KibanaConditions{
					Version: answers.KibanaVersion,
				},
				Elastic: packages.ElasticConditions{
					Subscription: answers.ElasticSubscription,
				},
			},
			Owner: packages.Owner{
				Github: answers.GithubOwner,
			},
			License:     answers.ElasticSubscription,
			Description: answers.Description,
			Categories:  answers.Categories,
		},
		InputDataStreamType: inputDataStreamType,
	}
}
