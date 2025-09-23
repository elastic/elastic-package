// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/elastic/elastic-package/internal/licenses"
	"github.com/elastic/elastic-package/internal/packages"
	"github.com/elastic/elastic-package/internal/packages/archetype"
	"github.com/elastic/elastic-package/internal/tui"
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
	OwnerType           string `survey:"owner_type"`
	DataStreamType      string `survey:"datastream_type"`
	Subobjects          bool
}

func createPackageCommandAction(cmd *cobra.Command, args []string) error {
	cmd.Println("Create a new package")

	validator := tui.Validator{Cwd: "."}

	// Create license select with description
	licenseSelect := tui.NewSelect("License", []string{licenses.Elastic20, licenses.Apache20, noLicenseValue}, licenses.Elastic20)
	licenseSelect.SetDescription(func(value string, _ int) string {
		if value == noLicenseValue {
			return noLicenseOnCreationMessage
		}
		return ""
	})

	// Create categories multi-select
	categoriesMultiSelect := tui.NewMultiSelect("Categories", []string{
		"aws", "azure", "cloud", "config_management", "containers", "crm", "custom",
		"datastore", "elastic_stack", "google_cloud", "kubernetes", "languages", "message_queue",
		"monitoring", "network", "notification", "os_system", "productivity", "security", "support",
		"ticketing", "version_control", "web",
	}, []string{"custom"})
	categoriesMultiSelect.SetPageSize(50)

	// Create owner type select with description
	ownerTypeSelect := tui.NewSelect("Owner type", []string{"elastic", "partner", "community"}, "elastic")
	ownerTypeSelect.SetDescription(func(value string, _ int) string {
		switch value {
		case "elastic":
			return "Owned and supported by Elastic"
		case "partner":
			return "Vendor-owned with support from Elastic"
		case "community":
			return "Supported by the community"
		}
		return ""
	})

	// Create all questions including conditional ones
	qs := []*tui.Question{
		{
			Name:     "type",
			Prompt:   tui.NewSelect("Package type", []string{"input", "integration", "content"}, "integration"),
			Validate: tui.Required,
		},
		{
			Name:     "name",
			Prompt:   tui.NewInput("Package name", "new_package"),
			Validate: tui.ComposeValidators(tui.Required, validator.PackageDoesNotExist, validator.PackageName),
		},
		{
			Name:     "version",
			Prompt:   tui.NewInput("Version", "0.0.1"),
			Validate: tui.ComposeValidators(tui.Required, validator.Semver),
		},
		{
			Name:   "source_license",
			Prompt: licenseSelect,
		},
		{
			Name:     "title",
			Prompt:   tui.NewInput("Package title", "New Package"),
			Validate: tui.Required,
		},
		{
			Name:     "description",
			Prompt:   tui.NewInput("Description", "This is a new package."),
			Validate: tui.Required,
		},
		{
			Name:     "categories",
			Prompt:   categoriesMultiSelect,
			Validate: tui.Required,
		},
		{
			Name:     "kibana_version",
			Prompt:   tui.NewInput("Kibana version constraint", tui.DefaultKibanaVersionConditionValue()),
			Validate: tui.ComposeValidators(tui.Required, validator.Constraint),
		},
		{
			Name:     "elastic_subscription",
			Prompt:   tui.NewSelect("Required Elastic subscription", []string{"basic", "gold", "platinum", "enterprise"}, "basic"),
			Validate: tui.Required,
		},
		{
			Name:     "github_owner",
			Prompt:   tui.NewInput("Github owner", "elastic/integrations"),
			Validate: tui.ComposeValidators(tui.Required, validator.GithubOwner),
		},
		{
			Name:     "owner_type",
			Prompt:   ownerTypeSelect,
			Validate: tui.Required,
		},
	}

	var answers newPackageAnswers
	err := tui.Ask(qs, &answers)
	if err != nil {
		return fmt.Errorf("prompt failed: %w", err)
	}

	// If input type, ask additional questions
	if answers.Type == "input" {
		inputQs := []*tui.Question{
			{
				Name:     "datastream_type",
				Prompt:   tui.NewSelect("Input Data Stream type", []string{"logs", "metrics"}, "logs"),
				Validate: tui.Required,
			},
			{
				Name:     "subobjects",
				Prompt:   tui.NewConfirm("Enable creation of subobjects for fields with dots in their names?", false),
				Validate: tui.Required,
			},
		}

		err = tui.Ask(inputQs, &answers)
		if err != nil {
			return fmt.Errorf("prompt failed: %w", err)
		}
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

	var elasticsearch *packages.Elasticsearch
	inputDataStreamType := ""
	if answers.Type == "input" {
		inputDataStreamType = answers.DataStreamType
		if !answers.Subobjects {
			elasticsearch = &packages.Elasticsearch{
				IndexTemplate: &packages.ManifestIndexTemplate{
					Mappings: &packages.ManifestMappings{
						Subobjects: false,
					},
				},
			}
		}
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
				Type:   answers.OwnerType,
			},
			Description:   answers.Description,
			Categories:    answers.Categories,
			Elasticsearch: elasticsearch,
		},
		InputDataStreamType: inputDataStreamType,
	}
}
