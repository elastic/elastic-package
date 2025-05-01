// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/elastic/elastic-package/internal/cobraext"
	"github.com/elastic/elastic-package/internal/common"
	"github.com/elastic/elastic-package/internal/elasticsearch"
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

	elasticsearchClient, err := stack.NewElasticsearchClientFromProfile(profile, opts...)
	if err != nil {
		return fmt.Errorf("can't create Elasticsearch client: %w", err)
	}

	_, err = elasticsearchClient.Ping()
	if err != nil {
		return fmt.Errorf("can't ping Elasticsearch: %w", err)
	}

	cmd.Println("Pinged Elasticsearch successfully")

	return nil
}