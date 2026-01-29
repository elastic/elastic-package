// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package tools

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"google.golang.org/adk/tool"
	"google.golang.org/adk/tool/functiontool"

	"github.com/elastic/elastic-package/internal/packages/archetype"
)

// ServiceInfoProvider is an interface for accessing service_info.md sections
// This interface breaks the import cycle between tools and docagent packages
type ServiceInfoProvider interface {
	IsAvailable() bool
	GetAllSections() string
	GetSections(sectionTitles []string) string
}

// ListDirectoryArgs represents arguments for list_directory tool
type ListDirectoryArgs struct {
	Path string `json:"path"`
}

// ListDirectoryResult represents the result of list_directory tool
type ListDirectoryResult struct {
	Content string `json:"content,omitempty"`
	Error   string `json:"error,omitempty"`
}

// ReadFileArgs represents arguments for read_file tool
type ReadFileArgs struct {
	Path string `json:"path"`
}

// ReadFileResult represents the result of read_file tool
type ReadFileResult struct {
	Content string `json:"content,omitempty"`
	Error   string `json:"error,omitempty"`
}

// WriteFileArgs represents arguments for write_file tool
type WriteFileArgs struct {
	Path    string `json:"path"`
	Content string `json:"content"`
}

// WriteFileResult represents the result of write_file tool
type WriteFileResult struct {
	Content string `json:"content,omitempty"`
	Error   string `json:"error,omitempty"`
}

// GetReadmeTemplateArgs represents arguments for get_readme_template tool (empty)
type GetReadmeTemplateArgs struct{}

// GetReadmeTemplateResult represents the result of get_readme_template tool
type GetReadmeTemplateResult struct {
	Content string `json:"content,omitempty"`
	Error   string `json:"error,omitempty"`
}

// GetServiceInfoArgs represents arguments for get_service_info tool
type GetServiceInfoArgs struct {
	ReadmeSection string `json:"readme_section,omitempty"`
}

// GetServiceInfoResult represents the result of get_service_info tool
type GetServiceInfoResult struct {
	Content string `json:"content,omitempty"`
	Error   string `json:"error,omitempty"`
}

// PackageTools creates the tools available to the LLM for package operations.
// These tools do not allow access to `docs/`, to prevent the LLM from confusing the generated and non-generated README versions.
func PackageTools(packageRoot string, serviceInfoProvider ServiceInfoProvider) []tool.Tool {
	var result []tool.Tool

	// list_directory tool
	listDirTool, err := functiontool.New(
		functiontool.Config{
			Name:        "list_directory",
			Description: "List files and directories in a given path within the package",
		},
		listDirectoryHandler(packageRoot),
	)
	if err != nil {
		panic("failed to create list_directory tool: " + err.Error())
	}
	result = append(result, listDirTool)

	// read_file tool
	readFileTool, err := functiontool.New(
		functiontool.Config{
			Name:        "read_file",
			Description: "Read the contents of a file within the package.",
		},
		readFileHandler(packageRoot),
	)
	if err != nil {
		panic("failed to create read_file tool: " + err.Error())
	}
	result = append(result, readFileTool)

	// write_file tool
	writeFileTool, err := functiontool.New(
		functiontool.Config{
			Name:        "write_file",
			Description: "Write content to a file within the package. This tool can only write in _dev/build/docs/.",
		},
		writeFileHandler(packageRoot),
	)
	if err != nil {
		panic("failed to create write_file tool: " + err.Error())
	}
	result = append(result, writeFileTool)

	// get_readme_template tool
	readmeTemplateTool, err := functiontool.New(
		functiontool.Config{
			Name:        "get_readme_template",
			Description: "Get the README.md template that should be used as the structure for generating package documentation. This template contains the required sections and format.",
		},
		getReadmeTemplateHandler(),
	)
	if err != nil {
		panic("failed to create get_readme_template tool: " + err.Error())
	}
	result = append(result, readmeTemplateTool)

	// Add example tools (list_examples and get_example) for category-based example retrieval
	result = append(result, CreateExampleTools()...)

	// get_service_info tool
	serviceInfoTool, err := functiontool.New(
		functiontool.Config{
			Name:        "get_service_info",
			Description: "Get relevant sections from the service_info.md knowledge base file. If no section is specified, returns the complete file. This file contains authoritative information about the service/technology.",
		},
		getServiceInfoHandler(serviceInfoProvider),
	)
	if err != nil {
		panic("failed to create get_service_info tool: " + err.Error())
	}
	result = append(result, serviceInfoTool)

	return result
}

// validatePathInRoot ensures the path stays within the root directory and is safe to access.
// It protects against path traversal attacks and symlink attacks.
func validatePathInRoot(packageRoot, userPath string) (string, error) {
	fullPath := filepath.Join(packageRoot, userPath)

	// Resolve symlinks to prevent symlink attacks
	resolvedPath, err := filepath.EvalSymlinks(fullPath)
	if err != nil {
		// If file doesn't exist yet, that's okay - validate the directory structure
		if os.IsNotExist(err) {
			resolvedPath = filepath.Clean(fullPath)
		} else {
			return "", fmt.Errorf("failed to resolve path: %w", err)
		}
	}

	// Resolve the package root too
	resolvedRoot, err := filepath.EvalSymlinks(packageRoot)
	if err != nil {
		return "", fmt.Errorf("failed to resolve package root: %w", err)
	}

	// Security check: ensure we stay within package root
	cleanPath := filepath.Clean(resolvedPath)
	cleanRoot := filepath.Clean(resolvedRoot)
	relPath, err := filepath.Rel(cleanRoot, cleanPath)
	if err != nil || strings.HasPrefix(relPath, "..") {
		return "", fmt.Errorf("path '%s' is outside package root", userPath)
	}

	return fullPath, nil
}

// listDirectoryHandler returns a handler for the list_directory tool
func listDirectoryHandler(packageRoot string) functiontool.Func[ListDirectoryArgs, ListDirectoryResult] {
	return func(ctx tool.Context, args ListDirectoryArgs) (ListDirectoryResult, error) {
		// Validate path security
		fullPath, err := validatePathInRoot(packageRoot, args.Path)
		if err != nil {
			return ListDirectoryResult{Error: fmt.Sprintf("access denied: %v", err)}, nil
		}

		entries, err := os.ReadDir(fullPath)
		if err != nil {
			return ListDirectoryResult{Error: fmt.Sprintf("failed to read directory: %v", err)}, nil
		}

		var result strings.Builder
		result.WriteString(fmt.Sprintf("Contents of %s:\n", args.Path))

		for _, entry := range entries {
			// Hide docs/ directory from LLM - it contains generated artifacts
			if entry.Name() == "docs" {
				continue
			}

			if entry.IsDir() {
				result.WriteString(fmt.Sprintf("  %s/ (directory)\n", entry.Name()))
			} else {
				info, err := entry.Info()
				if err == nil {
					result.WriteString(fmt.Sprintf("  %s (file, %d bytes)\n", entry.Name(), info.Size()))
				} else {
					result.WriteString(fmt.Sprintf("  %s (file)\n", entry.Name()))
				}
			}
		}

		return ListDirectoryResult{Content: result.String()}, nil
	}
}

// readFileHandler returns a handler for the read_file tool
func readFileHandler(packageRoot string) functiontool.Func[ReadFileArgs, ReadFileResult] {
	return func(ctx tool.Context, args ReadFileArgs) (ReadFileResult, error) {
		// Block access to generated artifacts in docs/ directory, except docs/knowledge_base/
		// which contains authoritative service information
		if strings.HasPrefix(args.Path, "docs/") && !strings.HasPrefix(args.Path, "docs/knowledge_base/") {
			return ReadFileResult{Error: "access denied: cannot read generated documentation in docs/ (use _dev/build/docs/ instead)"}, nil
		}

		// Validate path security
		fullPath, err := validatePathInRoot(packageRoot, args.Path)
		if err != nil {
			return ReadFileResult{Error: fmt.Sprintf("access denied: %v", err)}, nil
		}

		content, err := os.ReadFile(fullPath)
		if err != nil {
			return ReadFileResult{Error: fmt.Sprintf("failed to read file: %v", err)}, nil
		}

		return ReadFileResult{Content: string(content)}, nil
	}
}

// writeFileHandler returns a handler for the write_file tool
func writeFileHandler(packageRoot string) functiontool.Func[WriteFileArgs, WriteFileResult] {
	return func(ctx tool.Context, args WriteFileArgs) (WriteFileResult, error) {
		// First validate against package root
		fullPath, err := validatePathInRoot(packageRoot, args.Path)
		if err != nil {
			return WriteFileResult{Error: fmt.Sprintf("access denied: %v", err)}, nil
		}

		// Additional security check: ensure we only write in "_dev/build/docs"
		allowedDir := filepath.Join(packageRoot, "_dev", "build", "docs")

		// Resolve symlinks for the allowed directory too
		resolvedAllowed, err := filepath.EvalSymlinks(allowedDir)
		if err != nil {
			// If the directory doesn't exist yet, use the clean path
			if os.IsNotExist(err) {
				resolvedAllowed = filepath.Clean(allowedDir)
			} else {
				return WriteFileResult{Error: fmt.Sprintf("failed to resolve allowed directory: %v", err)}, nil
			}
		}

		cleanPath := filepath.Clean(fullPath)
		cleanAllowed := filepath.Clean(resolvedAllowed)
		relPath, err := filepath.Rel(cleanAllowed, cleanPath)
		if err != nil || strings.HasPrefix(relPath, "..") {
			return WriteFileResult{Error: fmt.Sprintf("access denied: path '%s' is outside allowed directory (_dev/build/docs/)", args.Path)}, nil
		}

		// Create directory if it doesn't exist
		dir := filepath.Dir(fullPath)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return WriteFileResult{Error: fmt.Sprintf("failed to create directory: %v", err)}, nil
		}

		// Write the file
		if err := os.WriteFile(fullPath, []byte(args.Content), 0o644); err != nil {
			return WriteFileResult{Error: fmt.Sprintf("failed to write file: %v", err)}, nil
		}

		return WriteFileResult{Content: fmt.Sprintf("Successfully wrote %d bytes to %s", len(args.Content), args.Path)}, nil
	}
}

// getReadmeTemplateHandler returns a handler for the get_readme_template tool
func getReadmeTemplateHandler() functiontool.Func[GetReadmeTemplateArgs, GetReadmeTemplateResult] {
	return func(ctx tool.Context, args GetReadmeTemplateArgs) (GetReadmeTemplateResult, error) {
		// Get the embedded template content
		templateContent := archetype.GetPackageDocsReadmeTemplate()
		return GetReadmeTemplateResult{Content: templateContent}, nil
	}
}

// getServiceInfoHandler returns a handler for the get_service_info tool
func getServiceInfoHandler(serviceInfoProvider ServiceInfoProvider) functiontool.Func[GetServiceInfoArgs, GetServiceInfoResult] {
	return func(ctx tool.Context, args GetServiceInfoArgs) (GetServiceInfoResult, error) {
		// Check if service_info is available
		if !serviceInfoProvider.IsAvailable() {
			return GetServiceInfoResult{Content: "Note: service_info.md file is not available for this package."}, nil
		}

		// If no readme_section provided, return all sections
		if args.ReadmeSection == "" {
			content := serviceInfoProvider.GetAllSections()
			return GetServiceInfoResult{Content: content}, nil
		}

		// Get mapping for the specified README section (hardcoded here to avoid import cycle)
		serviceInfoSections := GetServiceInfoMappingForSection(args.ReadmeSection)

		if len(serviceInfoSections) == 0 {
			// No mapping exists for this section, return all sections
			content := serviceInfoProvider.GetAllSections()
			return GetServiceInfoResult{Content: content}, nil
		}

		// Get the relevant sections
		content := serviceInfoProvider.GetSections(serviceInfoSections)

		if content == "" {
			// Requested sections not found, return all sections as fallback
			content = serviceInfoProvider.GetAllSections()
		}

		return GetServiceInfoResult{Content: content}, nil
	}
}

// GetServiceInfoMappingForSection returns service_info sections for a README section
// This mapping defines which service_info.md sections are relevant for each README section
func GetServiceInfoMappingForSection(readmeSectionTitle string) []string {
	// Mapping of README sections to service_info sections
	mapping := map[string][]string{
		"Overview": {
			"Common use cases",
			"Data types collected",
			"Compatibility",
		},
		"What do I need to use this integration?": {
			"Vendor prerequisites",
			"Elastic prerequisites",
		},
		"What data does this integration collect?": {
			"Data types collected",
		},
		"How do I deploy this integration?": {
			"Vendor set up steps",
			"Vendor set up resources",
			"Kibana set up steps",
			"Validation steps",
		},
		"Reference": {
			"Vendor resources",
			"Documentation sites",
		},
		"Performance and scaling": {
			"Performance and scaling",
			"Scaling and performance",
		},
		"Troubleshooting": {
			"Troubleshooting",
		},
	}

	// Normalize the title for lookup (case-insensitive)
	lowerTitle := strings.ToLower(strings.TrimSpace(readmeSectionTitle))

	// First pass: try exact matches (non-wildcard keys)
	for key, sections := range mapping {
		if !strings.HasSuffix(key, "*") {
			if strings.ToLower(key) == lowerTitle {
				return sections
			}
		}
	}

	// Second pass: try wildcard matches (keys ending with *)
	for key, sections := range mapping {
		if strings.HasSuffix(key, "*") {
			// Remove the * and check if the title starts with the prefix
			prefix := strings.ToLower(strings.TrimSuffix(key, "*"))
			if strings.HasPrefix(lowerTitle, prefix) {
				return sections
			}
		}
	}

	return []string{}
}
