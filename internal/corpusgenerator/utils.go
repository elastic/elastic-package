// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package corpusgenerator

import (
	"bytes"
	"io"
	"os"

	"github.com/elastic/elastic-integration-corpus-generator-tool/pkg/genlib"
)

func RunGenerator(generator genlib.Generator) error {
	state := genlib.NewGenState()

	f := os.Stdout
	buf := bytes.NewBufferString("")
	for {
		err := generator.Emit(state, buf)
		if err == nil {
			buf.WriteByte('\n')

			if _, err = f.Write(buf.Bytes()); err != nil {
				return err
			}

			buf.Reset()
		}

		if err == io.EOF {
			break
		}

		if err != nil {
			return err
		}
	}

	return generator.Close()
}

func GetGenerator(packageName, dataStreamName, commit string, totSizeInBytes uint64) (genlib.Generator, error) {

	genLibClient := NewClient(commit)

	config, err := genLibClient.GetConf(packageName, dataStreamName)
	if err != nil {
		return nil, err
	}
	fields, err := genLibClient.GetFields(packageName, dataStreamName)

	if err != nil {
		return nil, err
	}
	tpl, err := genLibClient.GetGoTextTemplate(packageName, dataStreamName)
	if err != nil {
		return nil, err
	}

	g, err := genlib.NewGeneratorWithTextTemplate(tpl, config, fields, totSizeInBytes)
	if err != nil {
		return nil, err
	}

	return g, nil
}
