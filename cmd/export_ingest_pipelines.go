// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/elastic/elastic-package/internal/tui"

	"github.com/spf13/cobra"

	"github.com/elastic/elastic-package/internal/cobraext"
	"github.com/elastic/elastic-package/internal/common"
	"github.com/elastic/elastic-package/internal/elasticsearch"
	"github.com/elastic/elastic-package/internal/elasticsearch/ingest"
	"github.com/elastic/elastic-package/internal/export"
	"github.com/elastic/elastic-package/internal/packages"
	"github.com/elastic/elastic-package/internal/stack"
)

const exportIngestPipelinesLongDescription = `Use this command to export ingest pipelines with referenced pipelines from the Elasticsearch instance.

Use this command to download selected ingest pipelines and its referenced processor pipelines from Elasticsearch. Select data stream or the package root directories to download the pipelines. Pipelines are downloaded as is and will need adjustment to meet your package needs.`

func exportIngestPipelinesCmd(cmd *cobra.Command, args []string) error {
	cmd.Println("Export Elasticsearch ingest pipelines")

	pipelineIDs, err := cmd.Flags().GetStringSlice(cobraext.IngestPipelineIDsFlagName)
	if err != nil {
		return cobraext.FlagParsingError(err, cobraext.IngestPipelineIDsFlagName)
	}

	common.TrimStringSlice(pipelineIDs)

	var opts []elasticsearch.ClientOption
	tlsSkipVerify, _ := cmd.Flags().GetBool(cobraext.TLSSkipVerifyFlagName)
	if tlsSkipVerify {
		opts = append(opts, elasticsearch.OptionWithSkipTLSVerify())
	}

	profile, err := cobraext.GetProfileFlag(cmd)
	if err != nil {
		return err
	}

	esClient, err := stack.NewElasticsearchClientFromProfile(profile, opts...)
	if err != nil {
		return fmt.Errorf("can't create Elasticsearch client: %w", err)
	}

	if len(pipelineIDs) == 0 {
		pipelineIDs, err = promptIngestPipelineIDs(cmd.Context(), esClient.API)
		if err != nil {
			return fmt.Errorf("prompt for ingest pipeline selection failed: %w", err)
		}

		if len(pipelineIDs) == 0 {
			cmd.Println("No ingest pipelines were found in Elasticsearch.")
			return nil
		}
	}

	packageRoot, err := packages.MustFindPackageRoot()
	if err != nil {
		return fmt.Errorf("locating package root failed: %w", err)
	}

	dataStreamDirs, err := getDataStreamDirs(packageRoot)
	if err != nil {
		return fmt.Errorf("getting data stream directories failed: %w", err)
	}

	rootWriteLocation := export.PipelineWriteLocation{
		Type:       export.PipelineWriteLocationTypeRoot,
		Name:       "Package root",
		ParentPath: packageRoot,
	}

	pipelineWriteLocations := append(dataStreamDirs, rootWriteLocation)

	pipelineWriteAssignments, err := promptWriteLocations(pipelineIDs, pipelineWriteLocations)
	if err != nil {
		return fmt.Errorf("prompt for ingest pipeline export locations failed: %w", err)
	}

	err = export.IngestPipelines(cmd.Context(), esClient.API, pipelineWriteAssignments)
	if err != nil {
		return err
	}

	cmd.Println("Done")
	return nil
}

func getDataStreamDirs(packageRoot string) ([]export.PipelineWriteLocation, error) {
	dataStreamDir := filepath.Join(packageRoot, "data_stream")

	_, err := os.Stat(dataStreamDir)
	if err != nil {
		return nil, fmt.Errorf("data_stream directory does not exist: %w", err)
	}

	dataStreamEntries, err := os.ReadDir(dataStreamDir)
	if err != nil {
		return nil, fmt.Errorf("could not read data_stream directory: %w", err)
	}

	var dataStreamDirs []export.PipelineWriteLocation

	for _, dirEntry := range dataStreamEntries {
		if dirEntry.IsDir() {
			pipelineWriteLocation := export.PipelineWriteLocation{
				Type:       export.PipelineWriteLocationTypeDataStream,
				Name:       dirEntry.Name(),
				ParentPath: filepath.Join(dataStreamDir, dirEntry.Name()),
			}
			dataStreamDirs = append(dataStreamDirs, pipelineWriteLocation)
		}
	}

	return dataStreamDirs, nil
}

func promptIngestPipelineIDs(ctx context.Context, api *elasticsearch.API) ([]string, error) {
	ingestPipelineNames, err := ingest.GetRemotePipelineNames(ctx, api)
	if err != nil {
		return nil, fmt.Errorf("finding ingest pipelines failed: %w", err)
	}

	ingestPipelineNames = slices.DeleteFunc(ingestPipelineNames, func(name string) bool {
		// Filter out system pipelines that start with dot "." or global@
		return strings.HasPrefix(name, ".") || strings.HasPrefix(name, "global@")
	})

	ingestPipelinesPrompt := tui.NewMultiSelect("Which ingest pipelines would you like to export?", ingestPipelineNames, []string{})
	ingestPipelinesPrompt.SetPageSize(20)

	var selectedOptions []string
	err = tui.AskOne(ingestPipelinesPrompt, &selectedOptions, tui.Required)
	if err != nil {
		return nil, err
	}

	return selectedOptions, nil
}

func promptWriteLocations(pipelineNames []string, writeLocations []export.PipelineWriteLocation) (export.PipelineWriteAssignments, error) {
	var options []string

	for _, writeLocation := range writeLocations {
		options = append(options, writeLocation.Name)
	}

	answers := make(map[string]string)

	for _, pipelineName := range pipelineNames {
		selectPrompt := tui.NewSelect(fmt.Sprintf("Select a location to export ingest pipeline '%s'", pipelineName), options, "")
		selectPrompt.SetDescription(func(value string, index int) string {
			if index < len(writeLocations) && writeLocations[index].Type == export.PipelineWriteLocationTypeDataStream {
				return "data stream"
			}
			return ""
		})

		var selectedLocation string
		err := tui.AskOne(selectPrompt, &selectedLocation, tui.Required)
		if err != nil {
			return nil, err
		}

		answers[pipelineName] = selectedLocation
	}

	pipelinesToWriteLocations := make(export.PipelineWriteAssignments)

	for pipeline, writeLocationName := range answers {
		writeLocationIdx := slices.IndexFunc(writeLocations, func(p export.PipelineWriteLocation) bool { return p.Name == writeLocationName })

		pipelinesToWriteLocations[pipeline] = writeLocations[writeLocationIdx]
	}

	return pipelinesToWriteLocations, nil
}
