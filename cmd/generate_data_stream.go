// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package cmd

import (
	"bytes"
	"github.com/dustin/go-humanize"
	"github.com/elastic/elastic-integration-corpus-generator-tool/pkg/genlib"
	"github.com/elastic/elastic-package/internal/cobraext"
	integration_corpus_generator "github.com/elastic/elastic-package/internal/integration-corpus-generator"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"os"
)

func generateDataStreamCommandAction(cmd *cobra.Command, _ []string) error {
	cmd.Println("Generate benchmarks data for a data stream")

	packageName, err := cmd.Flags().GetString(cobraext.PackageFlagName)
	if err != nil {
		return cobraext.FlagParsingError(err, cobraext.PackageRootFlagName)
	}

	dataStreamName, err := cmd.Flags().GetString(cobraext.GenerateDataStreamFlagName)
	if err != nil {
		return cobraext.FlagParsingError(err, cobraext.GenerateDataStreamFlagName)
	}

	totSize, err := cmd.Flags().GetString(cobraext.GenerateSizeFlagName)
	if err != nil {
		return cobraext.FlagParsingError(err, cobraext.GenerateSizeFlagName)
	}

	totSizeInBytes, err := humanize.ParseBytes(totSize)
	if err != nil {
		return cobraext.FlagParsingError(err, cobraext.GenerateSizeFlagName)
	}

	generator, err := getGenerator(packageName, dataStreamName)
	if err != nil {
		return errors.Wrap(err, "can't generate benchmarks data for data stream")
	}

	state := genlib.NewGenState()

	f := os.Stdout
	buf := bytes.NewBufferString("")
	var currentSize uint64
	for currentSize < totSizeInBytes {
		if err := generator.Emit(state, buf); err != nil {
			return err
		}

		buf.WriteByte('\n')

		if _, err = f.Write(buf.Bytes()); err != nil {
			return err
		}

		currentSize += uint64(buf.Len())
	}

	return generator.Close()
}

func getGenerator(packageName, dataStreamName string) (genlib.Generator, error) {

	genLibClient := integration_corpus_generator.NewClient()

	cfg, err := genLibClient.GetGenlibConf(packageName, dataStreamName)
	flds, err := genLibClient.GetGenlibFields(packageName, dataStreamName)
	tpl, err := genLibClient.GetGoTextTemplate(packageName, dataStreamName)

	g, err := genlib.NewGeneratorWithTextTemplate(tpl, cfg, flds)
	if err != nil {
		return nil, err
	}

	return g, nil
}
