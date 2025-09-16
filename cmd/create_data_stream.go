// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package cmd

import (
	"errors"
	"fmt"
	"maps"
	"slices"

	"github.com/AlecAivazis/survey/v2"
	"github.com/Masterminds/semver/v3"

	"github.com/spf13/cobra"

	"github.com/elastic/elastic-package/internal/packages"
	"github.com/elastic/elastic-package/internal/packages/archetype"
	"github.com/elastic/elastic-package/internal/surveyext"
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

	validator := surveyext.Validator{Cwd: "."}
	qs := []*survey.Question{
		{
			Name: "name",
			Prompt: &survey.Input{
				Message: "Data stream name:",
				Default: "new_data_stream",
			},
			Validate: survey.ComposeValidators(survey.Required, validator.DataStreamDoesNotExist, validator.DataStreamName),
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

	sv, err := semver.NewVersion(manifest.SpecVersion)
	if err != nil {
		return fmt.Errorf("failed to obtain spec version from package manifest in \"%s\": %w", packageRoot, err)
	}
	if !sv.LessThan(semver3_2_0) {
		qs = append(qs, &survey.Question{
			Name: "subobjects",
			Prompt: &survey.Confirm{
				Message: "Enable creation of subobjects for fields with dots in their names?",
				Default: false,
			},
			Validate: survey.Required,
		})
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

	if answers.Type == "logs" {
		// Map of possible inputs that can be used in the wizard, and their description.
		inputsMap := map[string]string{
			"aws-cloudwatch":     "AWS Cloudwatch",
			"aws-s3":             "AWS S3",
			"azure-blob-storage": "Azure Blob Storage",
			"azure-eventhub":     "Azure Eventhub",
			"cel":                "Common Expression Language (CEL)",
			"entity-analytics":   "Entity Analytics",
			"etw":                "Event Tracing for Windows (ETW)",
			"filestream":         "Filestream",
			"gcp-pubsub":         "GCP PubSub",
			"gcs":                "Google Cloud Storage (GCS)",
			"http_endpoint":      "HTTP Endpoint",
			"journald":           "Journald",
			"netflow":            "Netflow",
			"redis":              "Redis",
			"tcp":                "TCP",
			"udp":                "UDP",
			"winlog":             "WinLogBeat",
		}
		qs := []*survey.Question{
			{
				Name: "inputs",
				Prompt: &survey.MultiSelect{
					Message:  "Select input types which will be used in this data stream. See https://www.elastic.co/docs/reference/fleet/elastic-agent-inputs-list for description of the inputs",
					Options:  slices.Sorted(maps.Keys(inputsMap)),
					PageSize: 50,
					Description: func(value string, index int) string {
						val, ok := inputsMap[value]
						if ok {
							return val
						}
						return ""
					},
				},
			},
		}
		err = survey.Ask(qs, &answers)
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
