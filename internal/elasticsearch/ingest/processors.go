// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package ingest

import (
	"bufio"
	"bytes"
	"fmt"

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
		return nil, fmt.Errorf("unsupported pipeline format: %s", p.Format)
	}
	if err != nil {
		return nil, fmt.Errorf("failure processing %s pipeline '%s': %w", p.Format, p.Filename(), err)
	}
	return procs, nil
}

// OriginalProcessors return the original list of processors in an ingest pipeline.
func (p Pipeline) OriginalProcessors() (procs []Processor, err error) {
	switch p.Format {
	case "yaml", "yml", "json":
		procs, err = processorsFromYAML(p.ContentOriginal)
	default:
		return nil, fmt.Errorf("unsupported pipeline format: %s", p.Format)
	}
	if err != nil {
		return nil, fmt.Errorf("failure processing %s pipeline '%s': %w", p.Format, p.Filename(), err)
	}
	return procs, nil
}

// processorsFromYAML extracts a list of processors from a pipeline definition in YAML format.
func processorsFromYAML(content []byte) (procs []Processor, err error) {
	var p struct {
		Processors []yaml.Node
	}
	if err = yaml.Unmarshal(content, &p); err != nil {
		return nil, err
	}

	for idx, entry := range p.Processors {
		if entry.Kind != yaml.MappingNode || len(entry.Content) != 2 {
			return nil, fmt.Errorf("processor#%d is not a single-key map (kind:%v content:%d)", idx, entry.Kind, len(entry.Content))
		}
		var proc Processor
		if err := entry.Content[1].Decode(&proc); err != nil {
			return nil, fmt.Errorf("error decoding processor#%d configuration: %w", idx, err)
		}
		if err := entry.Content[0].Decode(&proc.Type); err != nil {
			return nil, fmt.Errorf("error decoding processor#%d type: %w", idx, err)
		}
		proc.FirstLine = entry.Line
		lastLine, err := getProcessorLastLine(idx, p.Processors, proc, content)
		if err != nil {
			return nil, err
		}
		proc.LastLine = lastLine

		procs = append(procs, proc)
	}
	return procs, err
}

// getProcessorLastLine determines the last line number for the given processor.
func getProcessorLastLine(idx int, processors []yaml.Node, currentProcessor Processor, content []byte) (int, error) {
	if idx < len(processors)-1 {
		var endProcessor = processors[idx+1].Line - 1
		if endProcessor < currentProcessor.FirstLine {
			return currentProcessor.FirstLine, nil
		} else {
			return processors[idx+1].Line - 1, nil
		}
	}

	return nextProcessorOrEndOfPipeline(content)
}

// nextProcessorOrEndOfPipeline get the line before the node after the processors node. If there is none, it returns the end of file line
func nextProcessorOrEndOfPipeline(content []byte) (int, error) {
	var root yaml.Node
	if err := yaml.Unmarshal(content, &root); err != nil {
		return 0, fmt.Errorf("error unmarshaling YAML: %v", err)
	}

	var nodes []*yaml.Node
	extractNodesFromMapping(&root, &nodes)
	for i, node := range nodes {

		if node.Value == "processors" {
			if i < len(nodes)-1 {

				return nodes[i+1].Line - 1, nil
			}
		}

	}
	return countLinesInBytes(content)
}

// extractNodesFromMapping recursively extracts all nodes from MappingNodes within DocumentNodes.
func extractNodesFromMapping(node *yaml.Node, nodes *[]*yaml.Node) {
	if node == nil {
		return
	}

	if node.Kind == yaml.DocumentNode {
		for _, child := range node.Content {
			extractNodesFromMapping(child, nodes)
		}
		return
	}

	if node.Kind == yaml.MappingNode {
		for _, child := range node.Content {
			if child.Kind == yaml.MappingNode || child.Kind == yaml.ScalarNode {
				*nodes = append(*nodes, child)
			}
			extractNodesFromMapping(child, nodes)
		}
	}
}

// countLinesInBytes counts the number of lines in the given byte slice.
func countLinesInBytes(data []byte) (int, error) {
	scanner := bufio.NewScanner(bytes.NewReader(data))
	lineCount := 0

	for scanner.Scan() {
		lineCount++
	}

	if err := scanner.Err(); err != nil {
		return 0, fmt.Errorf("error reading data: %w", err)
	}

	return lineCount, nil
}
