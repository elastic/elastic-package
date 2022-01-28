// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package ingest

import (
	"github.com/pkg/errors"
	"gopkg.in/yaml.v3"
)

// Processor represents an ingest processor.
type Processor struct {
	// Type of processor ("set", "script", etc.)
	Type string `yaml:"-"`
	// FirstLine is the line number where this processor definition starts
	// in the pipeline source code.
	FirstLine int `yaml:"-"`
	// LastLine is the line number where this processor definitions end
	// in the pipeline source code.
	LastLine int `yaml:"-"`
}

// Processors return the list of processors in an ingest pipeline.
func (p Pipeline) Processors() (procs []Processor, err error) {
	switch p.Format {
	case "yaml", "yml", "json":
		procs, err = processorsFromYAML(p.Content)
	default:
		return nil, errors.Errorf("unsupported pipeline format: %s", p.Format)
	}
	return procs, errors.Wrapf(err, "failure processing %s pipeline '%s'", p.Format, p.Filename())
}

// extract a list of processors from a pipeline definition in YAML format.
func processorsFromYAML(content []byte) (procs []Processor, err error) {
	var p struct {
		Processors []yaml.Node
	}
	if err = yaml.Unmarshal(content, &p); err != nil {
		return nil, err
	}
	for idx, entry := range p.Processors {
		if entry.Kind != yaml.MappingNode || len(entry.Content) != 2 {
			return nil, errors.Errorf("processor#%d is not a single-key map (kind:%v content:%d)", idx, entry.Kind, len(entry.Content))
		}
		var proc Processor
		if err := entry.Content[1].Decode(&proc); err != nil {
			return nil, errors.Wrapf(err, "error decoding processor#%d configuration", idx)
		}
		if err := entry.Content[0].Decode(&proc.Type); err != nil {
			return nil, errors.Wrapf(err, "error decoding processor#%d type", idx)
		}
		proc.FirstLine = entry.Line
		proc.LastLine = lastLine(&entry)
		procs = append(procs, proc)
	}
	return procs, nil
}

// returns the last (greater) line number used by a yaml.Node.
func lastLine(node *yaml.Node) int {
	if node == nil {
		return 0
	}
	last := node.Line
	for _, inner := range node.Content {
		if line := lastLine(inner); line > last {
			last = line
		}
	}
	return last
}
