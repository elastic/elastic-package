// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package dump

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/elastic/elastic-package/internal/common"
	"github.com/elastic/elastic-package/internal/elasticsearch"
)

const (
	ComponentTemplatesDumpDir = "component_templates"
	ILMPoliciesDumpDir        = "ilm_policies"
	IndexTemplatesDumpDir     = "index_templates"
	IngestPipelinesDumpDir    = "ingest_pipelines"
)

// InstalledObjectsDumper discovers and dumps objects installed in Elasticsearch for a given package.
type InstalledObjectsDumper struct {
	packageName string
	client      *elasticsearch.API

	componentTemplates []ComponentTemplate
	ilmPolicies        []ILMPolicy
	indexTemplates     []IndexTemplate
	ingestPipelines    []IngestPipeline
}

// NewInstalledObjectsDumper creates an InstalledObjectsDumper for a given package.
func NewInstalledObjectsDumper(client *elasticsearch.API, packageName string) *InstalledObjectsDumper {
	return &InstalledObjectsDumper{
		packageName: packageName,
		client:      client,
	}
}

// DumpAll discovers and dumps all known resources as files in the given directory.
func (e *InstalledObjectsDumper) DumpAll(ctx context.Context, dir string) (count int, err error) {
	n, err := e.dumpIndexTemplates(ctx, dir)
	if err != nil {
		return count, fmt.Errorf("failed to dump index templates: %w", err)
	}
	count += n

	n, err = e.dumpComponentTemplates(ctx, dir)
	if err != nil {
		return count, fmt.Errorf("failed to dump component templates: %w", err)
	}
	count += n

	n, err = e.dumpILMPolicies(ctx, dir)
	if err != nil {
		return count, fmt.Errorf("failed to dump ILM policies: %w", err)
	}
	count += n

	n, err = e.dumpIngestPipelines(ctx, dir)
	if err != nil {
		return count, fmt.Errorf("failed to dump ingest pipelines: %w", err)
	}
	count += n

	return count, nil
}

type DumpableInstalledObject interface {
	Name() string
	JSON() []byte
}

func dumpInstalledObject(dir string, object DumpableInstalledObject) error {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create dump directory: %w", err)
	}
	formatted, err := formatJSON(object.JSON())
	if err != nil {
		return fmt.Errorf("failed to format JSON object: %w", err)
	}
	path := filepath.Join(dir, object.Name()+".json")
	err = ioutil.WriteFile(path, formatted, 0644)
	if err != nil {
		return fmt.Errorf("failed to dump object to file: %w", err)
	}
	return nil
}

func formatJSON(in []byte) ([]byte, error) {
	var buf bytes.Buffer
	err := json.Indent(&buf, in, "", "  ")
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func (e *InstalledObjectsDumper) dumpIndexTemplates(ctx context.Context, dir string) (count int, err error) {
	indexTemplates, err := e.getIndexTemplates(ctx)
	if err != nil {
		return count, err
	}

	dir = filepath.Join(dir, IndexTemplatesDumpDir)
	for i, t := range indexTemplates {
		err := dumpInstalledObject(dir, t)
		if err != nil {
			return i, fmt.Errorf("failed to dump index template %s: %w", t.Name(), err)
		}
	}
	return len(indexTemplates), nil
}

func (e *InstalledObjectsDumper) getIndexTemplates(ctx context.Context) ([]IndexTemplate, error) {
	if len(e.indexTemplates) == 0 {
		indexTemplates, err := getIndexTemplatesForPackage(ctx, e.client, e.packageName)
		if err != nil {
			return nil, fmt.Errorf("failed to get index templates: %w", err)
		}
		e.indexTemplates = indexTemplates
	}

	return e.indexTemplates, nil
}

func (e *InstalledObjectsDumper) dumpComponentTemplates(ctx context.Context, dir string) (count int, err error) {
	componentTemplates, err := e.getComponentTemplates(ctx)
	if err != nil {
		return count, fmt.Errorf("failed to get component templates: %w", err)
	}

	dir = filepath.Join(dir, ComponentTemplatesDumpDir)
	for i, t := range componentTemplates {
		err := dumpInstalledObject(dir, t)
		if err != nil {
			return i, fmt.Errorf("failed to dump component template %s: %w", t.Name(), err)
		}
	}
	return len(componentTemplates), nil
}

func (e *InstalledObjectsDumper) getComponentTemplates(ctx context.Context) ([]ComponentTemplate, error) {
	if len(e.componentTemplates) == 0 {
		indexTemplates, err := e.getIndexTemplates(ctx)
		if err != nil {
			return nil, err
		}
		names := getComponentTemplatesFromIndexTemplates(indexTemplates)
		componentTemplates, err := getComponentTemplates(ctx, e.client, names...)
		if err != nil {
			return nil, fmt.Errorf("failed to get component templates: %w", err)
		}
		e.componentTemplates = componentTemplates
	}

	return e.componentTemplates, nil
}

func getComponentTemplatesFromIndexTemplates(indexTemplates []IndexTemplate) []string {
	var templates []string
	for _, it := range indexTemplates {
		composedOf := it.IndexTemplate.ComposedOf
		if len(composedOf) == 0 {
			continue
		}
		for _, ct := range composedOf {
			if !common.StringSliceContains(templates, ct) {
				templates = append(templates, ct)
			}
		}
	}
	return templates
}

func (e *InstalledObjectsDumper) dumpILMPolicies(ctx context.Context, dir string) (count int, err error) {
	ilmPolicies, err := e.getILMPolicies(ctx)
	if err != nil {
		return count, fmt.Errorf("failed to get index templates: %w", err)
	}

	dir = filepath.Join(dir, ILMPoliciesDumpDir)
	for i, t := range ilmPolicies {
		err := dumpInstalledObject(dir, t)
		if err != nil {
			return i, fmt.Errorf("failed to dump ILM policy %s: %w", t.Name(), err)
		}
	}
	return len(ilmPolicies), nil
}

func (e *InstalledObjectsDumper) getILMPolicies(ctx context.Context) ([]ILMPolicy, error) {
	if len(e.ilmPolicies) == 0 {
		templates, err := e.getTemplatesWithSettings(ctx)
		if err != nil {
			return nil, err
		}
		names := getILMPoliciesFromTemplates(templates)
		ilmPolicies, err := getILMPolicies(ctx, e.client, names...)
		if err != nil {
			return nil, fmt.Errorf("failed to get ILM policies: %w", err)
		}
		e.ilmPolicies = ilmPolicies
	}

	return e.ilmPolicies, nil
}

func getILMPoliciesFromTemplates(templates []TemplateWithSettings) []string {
	var policies []string
	for _, template := range templates {
		name := template.TemplateSettings().Index.Lifecycle.Name
		if name != "" && !common.StringSliceContains(policies, name) {
			policies = append(policies, name)
		}
	}
	return policies
}

func (e *InstalledObjectsDumper) dumpIngestPipelines(ctx context.Context, dir string) (count int, err error) {
	ingestPipelines, err := e.getIngestPipelines(ctx)
	if err != nil {
		return count, fmt.Errorf("failed to get ingest pipelines: %w", err)
	}

	dir = filepath.Join(dir, IngestPipelinesDumpDir)
	for i, t := range ingestPipelines {
		err := dumpInstalledObject(dir, t)
		if err != nil {
			return i, fmt.Errorf("failed to dump ingest pipeline %s: %w", t.Name(), err)
		}
	}
	return len(ingestPipelines), nil
}

func (e *InstalledObjectsDumper) getIngestPipelines(ctx context.Context) ([]IngestPipeline, error) {
	if len(e.ingestPipelines) == 0 {
		templates, err := e.getTemplatesWithSettings(ctx)
		if err != nil {
			return nil, err
		}

		names := getIngestPipelinesFromTemplates(templates)
		ingestPipelines, err := getIngestPipelines(ctx, e.client, names...)
		if err != nil {
			return nil, fmt.Errorf("failed to get ingest pipelines: %w", err)
		}
		e.ingestPipelines = ingestPipelines
	}

	return e.ingestPipelines, nil
}

func (e *InstalledObjectsDumper) getTemplatesWithSettings(ctx context.Context) ([]TemplateWithSettings, error) {
	var templates []TemplateWithSettings
	indexTemplates, err := e.getIndexTemplates(ctx)
	if err != nil {
		return nil, err
	}
	for _, template := range indexTemplates {
		templates = append(templates, template)
	}

	componentTemplates, err := e.getComponentTemplates(ctx)
	if err != nil {
		return nil, err
	}
	for _, template := range componentTemplates {
		templates = append(templates, template)
	}

	return templates, nil
}

type TemplateWithSettings interface {
	TemplateSettings() TemplateSettings
}

func getIngestPipelinesFromTemplates(templates []TemplateWithSettings) []string {
	var pipelines []string
	for _, template := range templates {
		settings := template.TemplateSettings()
		settingsPipelines := []string{
			settings.Index.DefaultPipeline,
			settings.Index.FinalPipeline,
		}
		for _, pipeline := range settingsPipelines {
			if pipeline == "" {
				continue
			}
			if common.StringSliceContains(pipelines, pipeline) {
				continue
			}
			pipelines = append(pipelines, pipeline)
		}
	}
	return pipelines
}
