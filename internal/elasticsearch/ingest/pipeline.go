// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package ingest

import (
	"encoding/json"
	"strings"

	"github.com/pkg/errors"
	"gopkg.in/yaml.v3"
)

// Pipeline represents a pipeline resource loaded from a file
type Pipeline struct {
	Name    string // Name of the pipeline
	Format  string // Format (extension) of the pipeline
	Content []byte // Content is the original file contents.
}

// Filename returns the original filename associated with the pipeline.
func (p *Pipeline) Filename() string {
	pos := strings.LastIndexByte(p.Name, '-')
	if pos == -1 {
		pos = len(p.Name)
	}
	return p.Name[:pos] + "." + p.Format
}

// MarshalJSON returns the pipeline contents in JSON format.
func (p *Pipeline) MarshalJSON() (asJSON []byte, err error) {
	switch p.Format {
	case "json":
		asJSON = p.Content
	case "yaml", "yml":
		var node map[string]interface{}
		err = yaml.Unmarshal(p.Content, &node)
		if err != nil {
			return nil, errors.Wrapf(err, "unmarshalling pipeline content failed (pipeline: %s)", p.Name)
		}
		if asJSON, err = json.Marshal(node); err != nil {
			return nil, errors.Wrapf(err, "marshalling pipeline content failed (pipeline: %s)", p.Name)
		}
	default:
		return nil, errors.Errorf("unsupported pipeline format '%s' (pipeline: %s)", p.Format, p.Name)
	}
	return asJSON, nil
}
