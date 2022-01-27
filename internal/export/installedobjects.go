// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package export

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/elastic/elastic-package/internal/elasticsearch"
)

const (
	ComponentTemplatesExportDir = "component_templates"
	ILMPoliciesExportDir        = "ilm_policies"
	IndexTemplatesExportDir     = "index_templates"
	IngestPipelinesExportDir    = "ingest_pipelines"
)

// InstalledObjectsExporter export objects installed in Elasticsearch for a given package.
type InstalledObjectsExporter struct {
	packageName string
	client      *elasticsearch.API

	componentTemplates []ComponentTemplate
	ilmPolicies        []ILMPolicy
	indexTemplates     []IndexTemplate
	ingestPipelines    []IngestPipeline
}

// NewInstalledObjectsExporter creates an InstalledObjectsExporter for a given package.
func NewInstalledObjectsExporter(client *elasticsearch.API, packageName string) *InstalledObjectsExporter {
	return &InstalledObjectsExporter{
		packageName: packageName,
		client:      client,
	}
}

// ExportAll exports all known resources as files in the given directory.
func (e *InstalledObjectsExporter) ExportAll(ctx context.Context, dir string) (count int, err error) {
	n, err := e.exportIndexTemplates(ctx, dir)
	if err != nil {
		return count, fmt.Errorf("failed to export index templates: %w", err)
	}
	count += n

	n, err = e.exportComponentTemplates(ctx, dir)
	if err != nil {
		return count, fmt.Errorf("failed to export component templates: %w", err)
	}
	count += n

	n, err = e.exportILMPolicies(ctx, dir)
	if err != nil {
		return count, fmt.Errorf("failed to export ILM policies: %w", err)
	}
	count += n

	n, err = e.exportIngestPipelines(ctx, dir)
	if err != nil {
		return count, fmt.Errorf("failed to export ingest pipelines: %w", err)
	}
	count += n

	return count, nil
}

type ExportableInstalledObject interface {
	Name() string
	JSON() []byte
}

func exportInstalledObject(dir string, object ExportableInstalledObject) error {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create export directory: %w", err)
	}
	path := filepath.Join(dir, object.Name()+".json")
	err := ioutil.WriteFile(path, object.JSON(), 0644)
	if err != nil {
		return fmt.Errorf("failed to export object to file: %w", err)
	}
	return nil
}

func (e *InstalledObjectsExporter) exportIndexTemplates(ctx context.Context, dir string) (count int, err error) {
	indexTemplates, err := e.getIndexTemplates(ctx)
	if err != nil {
		return count, err
	}

	dir = filepath.Join(dir, IndexTemplatesExportDir)
	for i, t := range indexTemplates {
		err := exportInstalledObject(dir, t)
		if err != nil {
			return i, fmt.Errorf("failed to export index template %s: %w", t.Name(), err)
		}
	}
	return len(indexTemplates), nil
}

func (e *InstalledObjectsExporter) getIndexTemplates(ctx context.Context) ([]IndexTemplate, error) {
	if len(e.indexTemplates) == 0 {
		indexTemplates, err := getIndexTemplatesForPackage(ctx, e.client, e.packageName)
		if err != nil {
			return nil, fmt.Errorf("failed to get index templates: %w", err)
		}
		e.indexTemplates = indexTemplates
	}

	return e.indexTemplates, nil
}

func (e *InstalledObjectsExporter) exportComponentTemplates(ctx context.Context, dir string) (count int, err error) {
	componentTemplates, err := e.getComponentTemplates(ctx)
	if err != nil {
		return count, fmt.Errorf("failed to get component templates: %w", err)
	}

	dir = filepath.Join(dir, ComponentTemplatesExportDir)
	for i, t := range componentTemplates {
		err := exportInstalledObject(dir, t)
		if err != nil {
			return i, fmt.Errorf("failed to export component template %s: %w", t.Name(), err)
		}
	}
	return len(componentTemplates), nil
}

func (e *InstalledObjectsExporter) getComponentTemplates(ctx context.Context) ([]ComponentTemplate, error) {
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
			if !stringSliceContains(templates, ct) {
				templates = append(templates, ct)
			}
		}
	}
	return templates
}

func (e *InstalledObjectsExporter) exportILMPolicies(ctx context.Context, dir string) (count int, err error) {
	ilmPolicies, err := e.getILMPolicies(ctx)
	if err != nil {
		return count, fmt.Errorf("failed to get index templates: %w", err)
	}

	dir = filepath.Join(dir, ILMPoliciesExportDir)
	for i, t := range ilmPolicies {
		err := exportInstalledObject(dir, t)
		if err != nil {
			return i, fmt.Errorf("failed to export ILM policy %s: %w", t.Name(), err)
		}
	}
	return len(ilmPolicies), nil
}

func (e *InstalledObjectsExporter) getILMPolicies(ctx context.Context) ([]ILMPolicy, error) {
	if len(e.ilmPolicies) == 0 {
		componentTemplates, err := e.getComponentTemplates(ctx)
		if err != nil {
			return nil, err
		}
		names := getILMPoliciesFromComponentTemplates(componentTemplates)
		ilmPolicies, err := getILMPolicies(ctx, e.client, names...)
		if err != nil {
			return nil, fmt.Errorf("failed to get ILM policies: %w", err)
		}
		e.ilmPolicies = ilmPolicies
	}

	return e.ilmPolicies, nil
}

func getILMPoliciesFromComponentTemplates(componentTemplates []ComponentTemplate) []string {
	var policies []string
	for _, ct := range componentTemplates {
		name := ct.ComponentTemplate.Template.Settings.Index.Lifecycle.Name
		if name != "" && !stringSliceContains(policies, name) {
			policies = append(policies, name)
		}
	}
	return policies
}

func (e *InstalledObjectsExporter) exportIngestPipelines(ctx context.Context, dir string) (count int, err error) {
	ingestPipelines, err := e.getIngestPipelines(ctx)
	if err != nil {
		return count, fmt.Errorf("failed to get ingest pipelines: %w", err)
	}

	dir = filepath.Join(dir, IngestPipelinesExportDir)
	for i, t := range ingestPipelines {
		err := exportInstalledObject(dir, t)
		if err != nil {
			return i, fmt.Errorf("failed to export ingest pipeline %s: %w", t.Name(), err)
		}
	}
	return len(ingestPipelines), nil
}

func (e *InstalledObjectsExporter) getIngestPipelines(ctx context.Context) ([]IngestPipeline, error) {
	if len(e.ingestPipelines) == 0 {
		indexTemplates, err := e.getIndexTemplates(ctx)
		if err != nil {
			return nil, err
		}
		names := getIngestPipelinesFromIndexTemplates(indexTemplates)
		ingestPipelines, err := getIngestPipelines(ctx, e.client, names...)
		if err != nil {
			return nil, fmt.Errorf("failed to get ingest pipelines: %w", err)
		}
		e.ingestPipelines = ingestPipelines
	}

	return e.ingestPipelines, nil
}

func getIngestPipelinesFromIndexTemplates(indexTemplates []IndexTemplate) []string {
	var pipelines []string
	for _, it := range indexTemplates {
		pipeline := it.IndexTemplate.Template.Settings.Index.DefaultPipeline
		if pipeline == "" {
			continue
		}
		if stringSliceContains(pipelines, pipeline) {
			continue
		}
		pipelines = append(pipelines, pipeline)
	}
	return pipelines
}

func stringSliceContains(ss []string, s string) bool {
	for i := range ss {
		if ss[i] == s {
			return true
		}
	}
	return false
}
