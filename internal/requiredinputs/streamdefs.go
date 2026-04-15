// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package requiredinputs

import (
	"fmt"
	"io/fs"
	"os"
	"path"

	"github.com/elastic/elastic-package/internal/logger"
	"github.com/elastic/elastic-package/internal/packages"
)

// inputPkgInfo holds the resolved metadata from an input package needed to
// replace package: references in composable package manifests.
type inputPkgInfo struct {
	identifier     string // policy_templates[0].input; if several templates exist, only the first is used
	pkgTitle       string // manifest.title (fallback title)
	pkgDescription string // manifest.description (fallback description)
}

// resolveStreamInputTypes replaces all package: <pkg-name> references in the
// composable package's manifest.yml (policy_templates[].inputs) and in each
// data_stream/*/manifest.yml (streams[]) with the actual input type identifier
// from the referenced input package, then removes the package: key.
//
// This step must run last, after mergeVariables, because that step uses
// stream.Package and input.Package to identify which entries to process.
// It resolves metadata per required input via buildInputPkgInfoByName, then
// rewrites the root manifest and each data stream manifest.
func (r *RequiredInputsResolver) resolveStreamInputTypes(
	manifest *packages.PackageManifest,
	inputPkgPaths map[string]string,
	buildRoot *os.Root,
) error {
	infoByPkg, err := buildInputPkgInfoByName(inputPkgPaths)
	if err != nil {
		return err
	}

	if err := applyInputTypesToComposableManifest(manifest, buildRoot, infoByPkg); err != nil {
		return err
	}

	return applyInputTypesToDataStreamManifests(buildRoot, infoByPkg)
}

// buildInputPkgInfoByName loads inputPkgInfo for each downloaded required input package path.
func buildInputPkgInfoByName(inputPkgPaths map[string]string) (map[string]inputPkgInfo, error) {
	infoByPkg := make(map[string]inputPkgInfo, len(inputPkgPaths))
	for pkgName, pkgPath := range inputPkgPaths {
		info, err := loadInputPkgInfo(pkgPath)
		if err != nil {
			return nil, fmt.Errorf("loading input package info for %q: %w", pkgName, err)
		}
		infoByPkg[pkgName] = info
	}
	return infoByPkg, nil
}

// applyInputTypesToComposableManifest sets type (and optional title/description) on
// package-backed policy template inputs in manifest.yml and drops package:.
func applyInputTypesToComposableManifest(
	manifest *packages.PackageManifest,
	buildRoot *os.Root,
	infoByPkg map[string]inputPkgInfo,
) error {
	manifestBytes, err := buildRoot.ReadFile("manifest.yml")
	if err != nil {
		return fmt.Errorf("reading manifest: %w", err)
	}
	root, err := parseDocumentRootMapping(manifestBytes)
	if err != nil {
		return fmt.Errorf("parsing manifest YAML: %w", err)
	}

	for ptIdx, pt := range manifest.PolicyTemplates {
		for inputIdx, input := range pt.Inputs {
			if input.Package == "" {
				continue
			}
			info, ok := infoByPkg[input.Package]
			if !ok {
				return fmt.Errorf("input package %q referenced in policy_templates[%d].inputs[%d] not found in required inputs", input.Package, ptIdx, inputIdx)
			}

			inputNode, err := getInputMappingNode(root, ptIdx, inputIdx)
			if err != nil {
				return fmt.Errorf("getting input node at pt[%d].inputs[%d]: %w", ptIdx, inputIdx, err)
			}

			upsertKey(inputNode, "type", strVal(info.identifier))

			if mappingValue(inputNode, "title") == nil && info.pkgTitle != "" {
				upsertKey(inputNode, "title", strVal(info.pkgTitle))
			}
			if mappingValue(inputNode, "description") == nil && info.pkgDescription != "" {
				upsertKey(inputNode, "description", strVal(info.pkgDescription))
			}

			removeKey(inputNode, "package")
		}
	}

	updated, err := formatYAMLNode(root)
	if err != nil {
		return fmt.Errorf("formatting updated manifest: %w", err)
	}
	if err := buildRoot.WriteFile("manifest.yml", updated, 0664); err != nil {
		return fmt.Errorf("writing updated manifest: %w", err)
	}
	return nil
}

// applyInputTypesToDataStreamManifests sets input on package-backed streams in each
// data_stream/*/manifest.yml and drops package:.
func applyInputTypesToDataStreamManifests(buildRoot *os.Root, infoByPkg map[string]inputPkgInfo) error {
	dsManifestPaths, err := fs.Glob(buildRoot.FS(), "data_stream/*/manifest.yml")
	if err != nil {
		return fmt.Errorf("globbing data stream manifests: %w", err)
	}

	for _, manifestPath := range dsManifestPaths {
		dsManifestBytes, err := buildRoot.ReadFile(manifestPath)
		if err != nil {
			return fmt.Errorf("reading data stream manifest %q: %w", manifestPath, err)
		}

		dsRoot, err := parseDocumentRootMapping(dsManifestBytes)
		if err != nil {
			return fmt.Errorf("parsing data stream manifest YAML %q: %w", manifestPath, err)
		}

		dsManifest, err := packages.ReadDataStreamManifestBytes(dsManifestBytes)
		if err != nil {
			return fmt.Errorf("parsing data stream manifest %q: %w", manifestPath, err)
		}

		for streamIdx, stream := range dsManifest.Streams {
			if stream.Package == "" {
				continue
			}
			info, ok := infoByPkg[stream.Package]
			if !ok {
				return fmt.Errorf("input package %q referenced in %q streams[%d] not found in required inputs", stream.Package, path.Dir(manifestPath), streamIdx)
			}

			streamNode, err := getStreamMappingNode(dsRoot, streamIdx)
			if err != nil {
				return fmt.Errorf("getting stream node at index %d in %q: %w", streamIdx, manifestPath, err)
			}

			upsertKey(streamNode, "input", strVal(info.identifier))

			if stream.Title == "" && info.pkgTitle != "" {
				upsertKey(streamNode, "title", strVal(info.pkgTitle))
			}
			if stream.Description == "" && info.pkgDescription != "" {
				upsertKey(streamNode, "description", strVal(info.pkgDescription))
			}

			removeKey(streamNode, "package")
		}

		dsUpdated, err := formatYAMLNode(dsRoot)
		if err != nil {
			return fmt.Errorf("formatting updated data stream manifest %q: %w", manifestPath, err)
		}
		if err := buildRoot.WriteFile(manifestPath, dsUpdated, 0664); err != nil {
			return fmt.Errorf("writing updated data stream manifest %q: %w", manifestPath, err)
		}
	}

	return nil
}

// loadInputPkgInfo reads an input package's manifest and extracts the metadata
// needed to replace package: references in composable packages. When the input
// package has several policy templates, only the first template's input id is
// used and a warning is logged.
func loadInputPkgInfo(pkgPath string) (inputPkgInfo, error) {
	pkgFS, closeFn, err := openPackageFS(pkgPath)
	if err != nil {
		return inputPkgInfo{}, fmt.Errorf("opening package: %w", err)
	}
	defer func() { _ = closeFn() }()

	manifestBytes, err := fs.ReadFile(pkgFS, packages.PackageManifestFile)
	if err != nil {
		return inputPkgInfo{}, fmt.Errorf("reading manifest: %w", err)
	}

	m, err := packages.ReadPackageManifestBytes(manifestBytes)
	if err != nil {
		return inputPkgInfo{}, fmt.Errorf("parsing manifest: %w", err)
	}

	if len(m.PolicyTemplates) == 0 {
		return inputPkgInfo{}, fmt.Errorf("input package %q has no policy templates", m.Name)
	}
	if len(m.PolicyTemplates) > 1 {
		logger.Warnf("Input package %q has multiple policy templates; using input identifier %q from first policy template only", m.Name, m.PolicyTemplates[0].Input)
	}

	pt := m.PolicyTemplates[0]
	if pt.Input == "" {
		return inputPkgInfo{}, fmt.Errorf("input package %q policy template %q has no input identifier", m.Name, pt.Name)
	}

	return inputPkgInfo{
		identifier:     pt.Input,
		pkgTitle:       m.Title,
		pkgDescription: m.Description,
	}, nil
}
