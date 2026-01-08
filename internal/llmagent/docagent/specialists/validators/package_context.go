// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package validators

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/elastic/elastic-package/internal/packages"
)

// ServiceInfoLink represents a link extracted from service_info.md
type ServiceInfoLink struct {
	Text string // The link text [text]
	URL  string // The link URL (url)
}

// PackageContext holds parsed package metadata for static validation.
// This provides ground-truth data from the package files that validators
// can use without requiring LLM calls.
type PackageContext struct {
	// PackageRoot is the root directory of the package
	PackageRoot string

	// Manifest is the parsed manifest.yml
	Manifest *packages.PackageManifest

	// DataStreams contains information about each data stream
	DataStreams []DataStreamInfo

	// Fields maps data stream names to their field definitions
	Fields map[string][]FieldInfo

	// ServiceInfo contains the service_info.md content (if available)
	ServiceInfo string

	// ServiceInfoLinks contains links extracted from service_info.md that should appear in generated docs
	ServiceInfoLinks []ServiceInfoLink

	// ExistingReadme contains the current README content (if available)
	ExistingReadme string

	// ReadmeTemplate contains the README template content
	ReadmeTemplate string
}

// DataStreamInfo holds metadata about a single data stream
type DataStreamInfo struct {
	Name        string
	Type        string // "logs", "metrics", "traces", "synthetics"
	Title       string
	Description string
	Dataset     string
}

// FieldInfo represents a field definition from fields.yml
type FieldInfo struct {
	Name        string
	Type        string
	Description string
	Unit        string
	MetricType  string
}

// DataStreamManifest represents the manifest.yml within a data stream
type DataStreamManifest struct {
	Title       string `yaml:"title"`
	Type        string `yaml:"type"`
	Dataset     string `yaml:"dataset"`
	Description string `yaml:"description,omitempty"`
}

// LoadPackageContext loads all package files needed for validation.
// This parses manifest.yml, data streams, fields, and other metadata.
func LoadPackageContext(packageRoot string) (*PackageContext, error) {
	ctx := &PackageContext{
		PackageRoot: packageRoot,
		DataStreams: []DataStreamInfo{},
		Fields:      make(map[string][]FieldInfo),
	}

	// 1. Load manifest.yml
	manifest, err := packages.ReadPackageManifestFromPackageRoot(packageRoot)
	if err != nil {
		return nil, fmt.Errorf("failed to load manifest.yml: %w", err)
	}
	ctx.Manifest = manifest

	// 2. Enumerate and load data streams
	dataStreams, err := loadDataStreams(packageRoot)
	if err != nil {
		// Non-fatal: package might not have data streams
		dataStreams = []DataStreamInfo{}
	}
	ctx.DataStreams = dataStreams

	// 3. Load fields for each data stream
	for _, ds := range dataStreams {
		fields, err := loadFieldsForDataStream(packageRoot, ds.Name)
		if err != nil {
			// Non-fatal: continue with empty fields
			continue
		}
		ctx.Fields[ds.Name] = fields
	}

	// 4. Load service_info.md (if exists) and extract links
	serviceInfoPath := filepath.Join(packageRoot, "docs", "knowledge_base", "service_info.md")
	if content, err := os.ReadFile(serviceInfoPath); err == nil {
		ctx.ServiceInfo = string(content)
		ctx.ServiceInfoLinks = extractMarkdownLinks(ctx.ServiceInfo)
	}

	// 5. Load existing README (if exists)
	readmePath := filepath.Join(packageRoot, "_dev", "build", "docs", "README.md")
	if content, err := os.ReadFile(readmePath); err == nil {
		ctx.ExistingReadme = string(content)
	}

	return ctx, nil
}

// loadDataStreams enumerates all data streams in the package
func loadDataStreams(packageRoot string) ([]DataStreamInfo, error) {
	dataStreamDir := filepath.Join(packageRoot, "data_stream")

	entries, err := os.ReadDir(dataStreamDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read data_stream directory: %w", err)
	}

	var dataStreams []DataStreamInfo
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		dsName := entry.Name()
		manifestPath := filepath.Join(dataStreamDir, dsName, "manifest.yml")

		content, err := os.ReadFile(manifestPath)
		if err != nil {
			// Skip data streams without manifest
			continue
		}

		var dsManifest DataStreamManifest
		if err := yaml.Unmarshal(content, &dsManifest); err != nil {
			continue
		}

		dataStreams = append(dataStreams, DataStreamInfo{
			Name:        dsName,
			Type:        dsManifest.Type,
			Title:       dsManifest.Title,
			Description: dsManifest.Description,
			Dataset:     dsManifest.Dataset,
		})
	}

	return dataStreams, nil
}

// loadFieldsForDataStream loads fields.yml for a specific data stream
func loadFieldsForDataStream(packageRoot, dataStreamName string) ([]FieldInfo, error) {
	fieldsDir := filepath.Join(packageRoot, "data_stream", dataStreamName, "fields")

	entries, err := os.ReadDir(fieldsDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read fields directory: %w", err)
	}

	var allFields []FieldInfo
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".yml") {
			continue
		}

		fieldsPath := filepath.Join(fieldsDir, entry.Name())
		content, err := os.ReadFile(fieldsPath)
		if err != nil {
			continue
		}

		fields, err := parseFieldsYAML(content)
		if err != nil {
			continue
		}

		allFields = append(allFields, fields...)
	}

	return allFields, nil
}

// parseFieldsYAML parses a fields.yml file into FieldInfo structs
func parseFieldsYAML(content []byte) ([]FieldInfo, error) {
	var rawFields []map[string]interface{}
	if err := yaml.Unmarshal(content, &rawFields); err != nil {
		return nil, err
	}

	var fields []FieldInfo
	for _, rf := range rawFields {
		field := FieldInfo{
			Name:        getStringField(rf, "name"),
			Type:        getStringField(rf, "type"),
			Description: getStringField(rf, "description"),
			Unit:        getStringField(rf, "unit"),
			MetricType:  getStringField(rf, "metric_type"),
		}

		if field.Name != "" {
			fields = append(fields, field)
		}

		// Recursively handle nested fields
		if nestedFields, ok := rf["fields"].([]interface{}); ok {
			for _, nf := range nestedFields {
				if nfMap, ok := nf.(map[string]interface{}); ok {
					nestedField := FieldInfo{
						Name:        field.Name + "." + getStringField(nfMap, "name"),
						Type:        getStringField(nfMap, "type"),
						Description: getStringField(nfMap, "description"),
						Unit:        getStringField(nfMap, "unit"),
						MetricType:  getStringField(nfMap, "metric_type"),
					}
					if nestedField.Name != "" {
						fields = append(fields, nestedField)
					}
				}
			}
		}
	}

	return fields, nil
}

// getStringField safely extracts a string field from a map
func getStringField(m map[string]interface{}, key string) string {
	if v, ok := m[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

// GetAllFieldNames returns all field names across all data streams
func (ctx *PackageContext) GetAllFieldNames() []string {
	seen := make(map[string]bool)
	var names []string

	for _, fields := range ctx.Fields {
		for _, f := range fields {
			if !seen[f.Name] {
				seen[f.Name] = true
				names = append(names, f.Name)
			}
		}
	}

	return names
}

// GetDataStreamNames returns all data stream names
func (ctx *PackageContext) GetDataStreamNames() []string {
	names := make([]string, len(ctx.DataStreams))
	for i, ds := range ctx.DataStreams {
		names[i] = ds.Name
	}
	return names
}

// FieldExists checks if a field name exists in any data stream
func (ctx *PackageContext) FieldExists(fieldName string) bool {
	for _, fields := range ctx.Fields {
		for _, f := range fields {
			if f.Name == fieldName {
				return true
			}
		}
	}
	return false
}

// markdownLinkRegex matches markdown links: [text](url)
var markdownLinkRegex = regexp.MustCompile(`\[([^\]]+)\]\((https?://[^)]+)\)`)

// extractMarkdownLinks extracts all markdown links [text](url) from content
func extractMarkdownLinks(content string) []ServiceInfoLink {
	var links []ServiceInfoLink
	seen := make(map[string]bool) // Deduplicate by URL

	matches := markdownLinkRegex.FindAllStringSubmatch(content, -1)
	for _, match := range matches {
		if len(match) >= 3 {
			url := match[2]
			// Skip if we've already seen this URL
			if seen[url] {
				continue
			}
			seen[url] = true

			links = append(links, ServiceInfoLink{
				Text: match[1],
				URL:  url,
			})
		}
	}

	return links
}

// GetServiceInfoLinks returns the extracted links from service_info.md
func (ctx *PackageContext) GetServiceInfoLinks() []ServiceInfoLink {
	return ctx.ServiceInfoLinks
}

// HasServiceInfoLinks returns true if there are links extracted from service_info.md
func (ctx *PackageContext) HasServiceInfoLinks() bool {
	return len(ctx.ServiceInfoLinks) > 0
}
