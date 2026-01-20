// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package validators

import (
	"fmt"
	"net/url"
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

// VendorSetupContent holds parsed vendor setup information from service_info.md
// This is used to determine if the generated documentation MUST include vendor setup
type VendorSetupContent struct {
	HasVendorPrerequisites bool              // Has "Vendor prerequisites" section
	HasVendorSetupSteps    bool              // Has "Vendor set up steps" section
	HasKibanaSetupSteps    bool              // Has "Kibana set up steps" section
	HasValidationSteps     bool              // Has "Validation Steps" section
	HasTroubleshooting     bool              // Has "Troubleshooting" section
	VendorPrerequisites    string            // Extracted content
	VendorSetupSteps       string            // Extracted content
	KibanaSetupSteps       string            // Extracted content
	ValidationSteps        string            // Extracted content
	Troubleshooting        string            // Extracted content
	VendorLinks            []ServiceInfoLink // Links to vendor documentation
}

// HasAnyVendorSetup returns true if service_info.md contains any vendor setup content
func (v *VendorSetupContent) HasAnyVendorSetup() bool {
	return v.HasVendorPrerequisites || v.HasVendorSetupSteps || v.HasKibanaSetupSteps
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

	// InputTypes contains unique input types used in this package (e.g., "logfile", "tcp", "httpjson")
	InputTypes []string

	// Fields maps data stream names to their field definitions
	Fields map[string][]FieldInfo

	// ServiceInfo contains the service_info.md content (if available)
	ServiceInfo string

	// ServiceInfoLinks contains links extracted from service_info.md that should appear in generated docs
	ServiceInfoLinks []ServiceInfoLink

	// VendorSetup contains parsed vendor setup content from service_info.md
	VendorSetup *VendorSetupContent

	// ExistingReadme contains the current README content (if available)
	ExistingReadme string

	// ReadmeTemplate contains the README template content
	ReadmeTemplate string
}

// DataStreamInfo holds metadata about a single data stream
type DataStreamInfo struct {
	Name            string
	Type            string // "logs", "metrics", "traces", "synthetics"
	Title           string
	Description     string
	Dataset         string
	HasExampleEvent bool // true if data_stream/<name>/sample_event.json exists
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
		InputTypes:  []string{},
		Fields:      make(map[string][]FieldInfo),
	}

	// 1. Load manifest.yml
	manifest, err := packages.ReadPackageManifestFromPackageRoot(packageRoot)
	if err != nil {
		return nil, fmt.Errorf("failed to load manifest.yml: %w", err)
	}
	ctx.Manifest = manifest

	// 2. Extract input types from policy templates
	ctx.InputTypes = extractInputTypes(manifest)

	// 3. Enumerate and load data streams
	dataStreams, err := loadDataStreams(packageRoot)
	if err != nil {
		// Non-fatal: package might not have data streams
		dataStreams = []DataStreamInfo{}
	}
	ctx.DataStreams = dataStreams

	// 4. Load fields for each data stream
	for _, ds := range dataStreams {
		fields, err := loadFieldsForDataStream(packageRoot, ds.Name)
		if err != nil {
			// Non-fatal: continue with empty fields
			continue
		}
		ctx.Fields[ds.Name] = fields
	}

	// 5. Load service_info.md (if exists) and extract links + vendor setup content
	serviceInfoPath := filepath.Join(packageRoot, "docs", "knowledge_base", "service_info.md")
	if content, err := os.ReadFile(serviceInfoPath); err == nil {
		ctx.ServiceInfo = string(content)
		ctx.ServiceInfoLinks = extractMarkdownLinks(ctx.ServiceInfo)
	}

	// 6. Load existing README (if exists)
	readmePath := filepath.Join(packageRoot, "_dev", "build", "docs", "README.md")
	if content, err := os.ReadFile(readmePath); err == nil {
		ctx.ExistingReadme = string(content)
	}

	return ctx, nil
}

// extractInputTypes extracts unique input types from the package manifest
func extractInputTypes(manifest *packages.PackageManifest) []string {
	seen := make(map[string]bool)
	var inputTypes []string

	for _, pt := range manifest.PolicyTemplates {
		// Check inputs within policy template
		for _, input := range pt.Inputs {
			if input.Type != "" && !seen[input.Type] {
				seen[input.Type] = true
				inputTypes = append(inputTypes, input.Type)
			}
		}
		// Check input package style (pt.Input field)
		if pt.Input != "" && !seen[pt.Input] {
			seen[pt.Input] = true
			inputTypes = append(inputTypes, pt.Input)
		}
	}

	return inputTypes
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

		// Check if sample_event.json exists for this data stream
		sampleEventPath := filepath.Join(dataStreamDir, dsName, "sample_event.json")
		_, sampleEventErr := os.Stat(sampleEventPath)
		hasExampleEvent := sampleEventErr == nil

		dataStreams = append(dataStreams, DataStreamInfo{
			Name:            dsName,
			Type:            dsManifest.Type,
			Title:           dsManifest.Title,
			Description:     dsManifest.Description,
			Dataset:         dsManifest.Dataset,
			HasExampleEvent: hasExampleEvent,
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
		// Extract fields recursively with full path names
		extractedFields := extractFieldsRecursively(rf, "")
		fields = append(fields, extractedFields...)
	}

	return fields, nil
}

// extractFieldsRecursively extracts fields from a nested YAML structure
// building full dotted field names like "citrix_adc.system.cpu.utilization.pct"
func extractFieldsRecursively(fieldMap map[string]interface{}, prefix string) []FieldInfo {
	var fields []FieldInfo

	name := getStringField(fieldMap, "name")
	if name == "" {
		return fields
	}

	// Build full field name with prefix
	fullName := name
	if prefix != "" {
		fullName = prefix + "." + name
	}

	// Add this field
	field := FieldInfo{
		Name:        fullName,
		Type:        getStringField(fieldMap, "type"),
		Description: getStringField(fieldMap, "description"),
		Unit:        getStringField(fieldMap, "unit"),
		MetricType:  getStringField(fieldMap, "metric_type"),
	}
	fields = append(fields, field)

	// Recursively process nested fields
	if nestedFields, ok := fieldMap["fields"].([]interface{}); ok {
		for _, nf := range nestedFields {
			if nfMap, ok := nf.(map[string]interface{}); ok {
				nestedExtracted := extractFieldsRecursively(nfMap, fullName)
				fields = append(fields, nestedExtracted...)
			}
		}
	}

	return fields
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

			text := match[1]
			// If the link text is just "link", derive a better description from the URL
			if strings.ToLower(strings.TrimSpace(text)) == "link" {
				text = deriveLinkTextFromURL(url)
			}

			links = append(links, ServiceInfoLink{
				Text: text,
				URL:  url,
			})
		}
	}

	return links
}

// deriveLinkTextFromURL creates descriptive link text from a URL
func deriveLinkTextFromURL(urlStr string) string {
	// Extract meaningful parts from the URL path
	parsedURL, err := url.Parse(urlStr)
	if err != nil {
		return "Documentation"
	}

	path := parsedURL.Path
	host := parsedURL.Host

	// Map common domains to friendly names
	domainNames := map[string]string{
		"developer-docs.netscaler.com": "NetScaler",
		"developer-docs.citrix.com":    "Citrix",
		"docs.netscaler.com":           "NetScaler Docs",
		"docs.citrix.com":              "Citrix Docs",
		"support.citrix.com":           "Citrix Support",
		"elastic.co":                   "Elastic",
		"www.elastic.co":               "Elastic",
	}

	vendor := "Documentation"
	for domain, name := range domainNames {
		if strings.Contains(host, domain) {
			vendor = name
			break
		}
	}

	// Extract meaningful path components
	pathParts := strings.Split(strings.Trim(path, "/"), "/")
	var meaningfulParts []string

	skipWords := map[string]bool{
		"en":       true,
		"us":       true,
		"current":  true,
		"latest":   true,
		"12.0":     true,
		"projects": true,
		"s":        true,
		"article":  true,
	}

	for _, part := range pathParts {
		if part == "" || skipWords[part] {
			continue
		}
		// Clean up the part
		part = strings.ReplaceAll(part, "-", " ")
		part = strings.ReplaceAll(part, "_", " ")
		// Capitalize first letter
		if len(part) > 0 {
			part = strings.ToUpper(part[:1]) + part[1:]
		}
		meaningfulParts = append(meaningfulParts, part)
		// Only take first 2-3 meaningful parts
		if len(meaningfulParts) >= 3 {
			break
		}
	}

	if len(meaningfulParts) > 0 {
		return vendor + " " + strings.Join(meaningfulParts, " ")
	}

	return vendor + " Documentation"
}

// GetServiceInfoLinks returns the extracted links from service_info.md
func (ctx *PackageContext) GetServiceInfoLinks() []ServiceInfoLink {
	return ctx.ServiceInfoLinks
}

// HasServiceInfoLinks returns true if there are links extracted from service_info.md
func (ctx *PackageContext) HasServiceInfoLinks() bool {
	return len(ctx.ServiceInfoLinks) > 0
}

// AdvancedSetting represents a configuration variable with important caveats
type AdvancedSetting struct {
	Name        string   // Variable name
	Title       string   // Human-readable title
	Description string   // Full description
	Type        string   // Variable type (bool, yaml, password, etc.)
	IsSecret    bool     // Whether this is a secret/password field
	ShowUser    bool     // Whether shown to user by default
	GotchaTypes []string // Types of gotchas (security, debug, ssl, sensitive, complex)
	Warning     string   // Extracted warning message
}

// GetAdvancedSettings extracts configuration variables with important caveats from the manifest
func (ctx *PackageContext) GetAdvancedSettings() []AdvancedSetting {
	var settings []AdvancedSetting
	if ctx.Manifest == nil {
		return settings
	}

	// Extract from policy templates
	for _, pt := range ctx.Manifest.PolicyTemplates {
		for _, input := range pt.Inputs {
			for _, v := range input.Vars {
				if setting := analyzeVarForAdvancedSetting(v); setting != nil {
					settings = append(settings, *setting)
				}
			}
		}
		// Check policy template level vars
		for _, v := range pt.Vars {
			if setting := analyzeVarForAdvancedSetting(v); setting != nil {
				settings = append(settings, *setting)
			}
		}
	}

	// Extract from package-level vars
	for _, v := range ctx.Manifest.Vars {
		if setting := analyzeVarForAdvancedSetting(v); setting != nil {
			settings = append(settings, *setting)
		}
	}

	return settings
}

// HasAdvancedSettings returns true if the package has advanced settings with gotchas
func (ctx *PackageContext) HasAdvancedSettings() bool {
	return len(ctx.GetAdvancedSettings()) > 0
}

// analyzeVarForAdvancedSetting checks if a variable has important caveats
func analyzeVarForAdvancedSetting(v packages.Variable) *AdvancedSetting {
	setting := &AdvancedSetting{
		Name:        v.Name,
		Title:       v.Title,
		Description: v.Description,
		Type:        v.Type,
		IsSecret:    v.Secret,
		ShowUser:    v.ShowUser,
		GotchaTypes: []string{},
	}

	descLower := strings.ToLower(v.Description)
	nameLower := strings.ToLower(v.Name)

	// Check for security-related gotchas
	securityPatterns := []string{
		"compromise", "security", "sensitive", "expose",
		"should only be used for debugging", "debug only",
		"not recommended for production",
	}
	for _, pattern := range securityPatterns {
		if strings.Contains(descLower, pattern) {
			setting.GotchaTypes = append(setting.GotchaTypes, "security")
			setting.Warning = extractWarning(v.Description)
			break
		}
	}

	// Check for debug/development settings
	debugPatterns := []string{"debug", "debugging", "development", "troubleshoot", "request tracer", "verbose", "trace"}
	for _, pattern := range debugPatterns {
		if strings.Contains(descLower, pattern) || strings.Contains(nameLower, pattern) {
			setting.GotchaTypes = append(setting.GotchaTypes, "debug")
			if setting.Warning == "" {
				setting.Warning = "This setting should only be used for debugging/troubleshooting"
			}
			break
		}
	}

	// Check for SSL/TLS configuration
	if strings.Contains(nameLower, "ssl") || strings.Contains(nameLower, "tls") || strings.Contains(nameLower, "certificate") {
		setting.GotchaTypes = append(setting.GotchaTypes, "ssl")
	}

	// Check for sensitive/secret fields
	if v.Secret || v.Type == "password" || strings.Contains(nameLower, "password") ||
		strings.Contains(nameLower, "secret") || strings.Contains(nameLower, "api_key") {
		setting.GotchaTypes = append(setting.GotchaTypes, "sensitive")
	}

	// Check for complex configurations (YAML/JSON types)
	if v.Type == "yaml" || v.Type == "json" {
		setting.GotchaTypes = append(setting.GotchaTypes, "complex")
	}

	// Only return if there are actual gotchas
	if len(setting.GotchaTypes) > 0 {
		return setting
	}

	return nil
}

// extractWarning extracts the warning/caveat from a description
func extractWarning(description string) string {
	sentences := strings.Split(description, ".")
	for _, sentence := range sentences {
		lower := strings.ToLower(sentence)
		if strings.Contains(lower, "security") || strings.Contains(lower, "compromise") ||
			strings.Contains(lower, "should only") || strings.Contains(lower, "debug") ||
			strings.Contains(lower, "not recommended") {
			return strings.TrimSpace(sentence) + "."
		}
	}
	return ""
}

// FormatAdvancedSettingsForGenerator formats advanced settings for inclusion in generator context
func (ctx *PackageContext) FormatAdvancedSettingsForGenerator() string {
	settings := ctx.GetAdvancedSettings()
	if len(settings) == 0 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("\n=== ADVANCED SETTINGS (document these with appropriate warnings) ===\n")
	sb.WriteString("The following settings have important caveats that MUST be documented:\n\n")

	// Group by gotcha type for clarity
	securitySettings := []AdvancedSetting{}
	debugSettings := []AdvancedSetting{}
	sslSettings := []AdvancedSetting{}
	sensitiveSettings := []AdvancedSetting{}
	complexSettings := []AdvancedSetting{}

	for _, s := range settings {
		for _, t := range s.GotchaTypes {
			switch t {
			case "security":
				securitySettings = append(securitySettings, s)
			case "debug":
				debugSettings = append(debugSettings, s)
			case "ssl":
				sslSettings = append(sslSettings, s)
			case "sensitive":
				sensitiveSettings = append(sensitiveSettings, s)
			case "complex":
				complexSettings = append(complexSettings, s)
			}
		}
	}

	if len(securitySettings) > 0 {
		sb.WriteString("### Security Warnings (CRITICAL - must include warnings)\n")
		for _, s := range securitySettings {
			sb.WriteString(fmt.Sprintf("- **%s** (`%s`): %s\n", s.Title, s.Name, s.Description))
			if s.Warning != "" {
				sb.WriteString(fmt.Sprintf("  ⚠️ WARNING: %s\n", s.Warning))
			}
		}
		sb.WriteString("\n")
	}

	if len(debugSettings) > 0 {
		sb.WriteString("### Debug/Development Settings (warn about production use)\n")
		for _, s := range debugSettings {
			sb.WriteString(fmt.Sprintf("- **%s** (`%s`): %s\n", s.Title, s.Name, s.Description))
		}
		sb.WriteString("\n")
	}

	if len(sslSettings) > 0 {
		sb.WriteString("### SSL/TLS Configuration (document certificate setup)\n")
		for _, s := range sslSettings {
			sb.WriteString(fmt.Sprintf("- **%s** (`%s`): %s\n", s.Title, s.Name, s.Description))
		}
		sb.WriteString("\n")
	}

	if len(sensitiveSettings) > 0 {
		sb.WriteString("### Sensitive Fields (mention secure handling)\n")
		for _, s := range sensitiveSettings {
			sb.WriteString(fmt.Sprintf("- **%s** (`%s`): Type=%s, Secret=%v\n", s.Title, s.Name, s.Type, s.IsSecret))
		}
		sb.WriteString("\n")
	}

	if len(complexSettings) > 0 {
		sb.WriteString("### Complex Configuration (include examples)\n")
		for _, s := range complexSettings {
			sb.WriteString(fmt.Sprintf("- **%s** (`%s`): YAML/JSON configuration - provide example\n", s.Title, s.Name))
		}
		sb.WriteString("\n")
	}

	return sb.String()
}
