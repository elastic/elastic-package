// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package pipelinetag

import (
	"encoding/json"
	"fmt"
	"hash/fnv"
	"strings"

	"github.com/elastic/elastic-package/internal/fleetpkg"
	"github.com/elastic/elastic-package/internal/modify"
	"github.com/elastic/elastic-package/internal/yamledit"
)

const Name = "pipeline-tag"

var pathCleaner = strings.NewReplacer(".", "_", " ", "_", "@", "")

var Modifier = &modify.Modifier{
	Name: Name,
	Doc:  "Generate tags for ingest pipeline processors",
	Run:  run,
}

type processorNode struct {
	Processor *fleetpkg.Processor
	Parent    *processorNode
}

func (p *processorNode) ParentProcessor() *fleetpkg.Processor {
	if p.Parent != nil {
		return p.Parent.Processor
	}

	return nil
}

func run(pkg *fleetpkg.Package) error {
	fmt.Println("Generating pipeline tags")

	for _, ds := range pkg.DataStreams {
		for _, pipeline := range ds.Pipelines {
			if err := processPipeline(pipeline); err != nil {
				return err
			}

			if pipeline.Doc.Modified() {
				if err := pipeline.Doc.WriteFile(); err != nil {
					return err
				}
			}
		}
	}

	return nil
}

func processPipeline(pipeline *fleetpkg.Pipeline) error {
	seen := map[string]*processorNode{}

	for _, proc := range pipeline.Processors {
		node := processorNode{
			Processor: proc,
		}

		if err := processTag(pipeline, &node, seen); err != nil {
			return err
		}
	}

	return nil
}

func processTag(pipeline *fleetpkg.Pipeline, node *processorNode, seen map[string]*processorNode) error {
	var invalid bool
	var err error

	tag, ok := node.Processor.Attributes["tag"].(string)
	if ok {
		if tag == "" {
			invalid = true
		} else if _, dup := seen[tag]; dup {
			invalid = true

		}
	} else {
		invalid = true
	}

	if invalid {
		if tag, err = generateTag(node.Processor, node.ParentProcessor()); err != nil {
			return err
		}
		if _, err = pipeline.Doc.SetKeyValue(fmt.Sprintf("%s.%s", node.Processor.Node.GetPath(), node.Processor.Type), "tag", tag, yamledit.IndexPrepend); err != nil {
			return err
		}
	}

	seen[tag] = node

	for _, onFailProc := range node.Processor.OnFailure {
		onFailProcNode := &processorNode{
			Processor: onFailProc,
			Parent:    node,
		}

		if err = processTag(pipeline, onFailProcNode, seen); err != nil {
			return err
		}
	}

	return nil
}

func generateTag(proc, parent *fleetpkg.Processor) (string, error) {
	hash, err := generateProcessorHash(proc, parent)
	if err != nil {
		return "", err
	}

	field, ok := proc.Attributes["field"].(string)
	if !ok || field == "" {
		return proc.Type + "_" + hash, nil
	}
	field = pathCleaner.Replace(field)

	targetField, ok := proc.Attributes["target_field"].(string)
	if !ok || targetField == "" {
		return fmt.Sprintf("%s_%s_%s", proc.Type, field, hash), nil
	}
	targetField = pathCleaner.Replace(targetField)

	return fmt.Sprintf("%s_%s_to_%s_%s", proc.Type, field, targetField, hash), nil
}

func generateProcessorHash(proc, parent *fleetpkg.Processor) (string, error) {
	b, err := json.Marshal(proc)
	if err != nil {
		return "", fmt.Errorf("failed to marshal processor for hashing: %w", err)
	}

	h := fnv.New32a()
	_, _ = h.Write(b)

	if parent != nil {
		b, err = json.Marshal(parent)
		if err != nil {
			return "", fmt.Errorf("failed to marshal parent processor for hashing: %w", err)
		}

		_, _ = h.Write(b)
	}

	return fmt.Sprintf("%08x", h.Sum32()), nil
}
