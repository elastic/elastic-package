// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package corpusgenerator

import (
	"bytes"
	"io"
	"os"
	"path/filepath"

	"github.com/elastic/elastic-integration-corpus-generator-tool/pkg/genlib"
)

func RunGenerator(generator genlib.Generator, dataStream, rallyTrackOutputDir string) error {
	var f io.Writer
	if len(rallyTrackOutputDir) == 0 {
		f = os.Stdout
	} else {
		err := os.MkdirAll(rallyTrackOutputDir, os.ModePerm)
		if err != nil {
			return err
		}

		f, err = os.CreateTemp(rallyTrackOutputDir, "corpus-*")
		if err != nil {
			return err
		}
	}
	buf := bytes.NewBufferString("")
	var corpusDocsCount uint64
	for {
		err := generator.Emit(buf)
		if err == io.EOF {
			break
		}

		if err != nil {
			return err
		}

		// TODO: this should be taken care of by the corpus generator tool, once it will be done let's remove this
		event := bytes.ReplaceAll(buf.Bytes(), []byte("\n"), []byte(""))
		if _, err = f.Write(event); err != nil {
			return err
		}

		if _, err = f.Write([]byte("\n")); err != nil {
			return err
		}

		buf.Reset()
		corpusDocsCount += 1
	}

	if len(rallyTrackOutputDir) > 0 {
		corpusFile := f.(*os.File)
		rallyTrackContent, err := GenerateRallyTrack(dataStream, corpusFile, corpusDocsCount)
		if err != nil {
			return err
		}

		err = os.WriteFile(filepath.Join(rallyTrackOutputDir, "track.json"), rallyTrackContent, os.ModePerm)
		if err != nil {
			return err
		}

	}

	return generator.Close()
}

func NewGenerator(genLibClient GenLibClient, packageName, dataStreamName string, totEvents uint64) (genlib.Generator, error) {

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

	g, err := genlib.NewGeneratorWithTextTemplate(tpl, config, fields, totEvents)
	if err != nil {
		return nil, err
	}

	return g, nil
}
