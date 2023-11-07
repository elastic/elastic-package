// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package corpusgenerator

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"text/template"
)

const (
	rallyTrackTemplate = `{% import "rally.helpers" as rally with context %}
{
  "version": 2,
  "description": "Track for [[.DataStream]]",
  "datastream": [
    {
      "name": "[[.DataStream]]",
      "body": "[[.CorpusFilename]]"
    }
  ],
  "corpora": [
    {
      "name": "[[.CorpusFilename]]",
      "documents": [
        {
          "target-data-stream": "[[.DataStream]]",
          "source-file": "[[.CorpusFilename]]",
          "document-count": [[.CorpusDocsCount]],
          "uncompressed-bytes": [[.CorpusSizeInBytes]]
        }
      ]
    }
  ],
  "schedule": [
    {
      "operation": {
        "operation-type": "bulk",
        "bulk-size": {{bulk_size | default(5000)}},
        "ingest-percentage": {{ingest_percentage | default(100)}}
      },
      "clients": {{bulk_indexing_clients | default(8)}}
    }
  ]
}
`
)

func GenerateRallyTrack(dataStream string, corpusFile *os.File, corpusDocsCount uint64) ([]byte, error) {
	t := template.New("rallytrack")

	parsedTpl, err := t.Delims("[[", "]]").Parse(rallyTrackTemplate)
	if err != nil {
		return nil, fmt.Errorf("error while parsing rally track template: %w", err)
	}

	fi, err := corpusFile.Stat()
	if err != nil {
		return nil, fmt.Errorf("error with stat on rally corpus file: %w", err)
	}

	corpusSizeInBytes := fi.Size()

	buf := new(bytes.Buffer)
	templateData := map[string]any{
		"DataStream":        dataStream,
		"CorpusFilename":    filepath.Base(corpusFile.Name()),
		"CorpusDocsCount":   corpusDocsCount,
		"CorpusSizeInBytes": corpusSizeInBytes,
	}

	err = parsedTpl.Execute(buf, templateData)
	if err != nil {
		return nil, fmt.Errorf("error on parsin on rally track template: %w", err)
	}

	return buf.Bytes(), nil
}
