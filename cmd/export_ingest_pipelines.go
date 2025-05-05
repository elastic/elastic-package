// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package cmd

import (
	"context"
	"fmt"

	"github.com/AlecAivazis/survey/v2"

	"github.com/spf13/cobra"

	"github.com/elastic/elastic-package/internal/cobraext"
	"github.com/elastic/elastic-package/internal/common"
	"github.com/elastic/elastic-package/internal/elasticsearch"
	"github.com/elastic/elastic-package/internal/elasticsearch/ingest"
	"github.com/elastic/elastic-package/internal/export"
	"github.com/elastic/elastic-package/internal/stack"
)


const exportIngestPipelinesLongDescription = `Use this command to export ingest pipelines with referenced objects from the Elasticsearch instance.

Use this command to download selected ingest pipelines and other associated saved objects from Elasticsearch. This command adjusts the downloaded saved objects according to package naming conventions (prefixes, unique IDs) and writes them locally into folders corresponding to saved object types (pipelines, etc.).`

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

	err = export.IngestPipelines(cmd.Context(), esClient.API, pipelineIDs...)

	if err != nil {
		return err
	}

	cmd.Println("Done")
	return nil
}

func promptIngestPipelineIDs(ctx context.Context, api *elasticsearch.API) ([]string, error) {
	ingestPipelineNames, err := ingest.GetRemotePipelineNames(ctx, api)
	if err != nil {
		return nil, fmt.Errorf("finding ingest pipelines failed: %w", err)
	}

	ingestPipelinesPrompt := &survey.MultiSelect{
		Message:  "Which ingest pipelines would you like to export?",
		Options:  ingestPipelineNames,
		PageSize: 20,
	}

	var selectedOptions []string
	err = survey.AskOne(ingestPipelinesPrompt, &selectedOptions, survey.WithValidator(survey.Required))
	if err != nil {
		return nil, err
	}

	return selectedOptions, nil
}