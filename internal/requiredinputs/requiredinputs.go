// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package requiredinputs

import (
	"archive/zip"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"

	"github.com/elastic/elastic-package/internal/logger"
	"github.com/elastic/elastic-package/internal/packages"
)

type eprClient interface {
	DownloadPackage(packageName string, packageVersion string, tmpDir string) (string, error)
}

// Resolver enriches a built integration package using required input packages from the registry:
// policy and data stream templates, merged manifest variables, data stream field definitions,
// and resolution of package: references on inputs and streams to the effective input type
// from the required input package, where applicable.
type Resolver interface {
	Bundle(buildPackageRoot string) error

	// ResolveInputTypes resolves package: references in-place on:
	//   - manifest.PolicyTemplates[].Inputs[] (sets Type, clears Package)
	//   - each element of dataStreams[].Streams[] (sets Input, clears Package)
	// It is a no-op when manifest.Requires is nil or has no input dependencies.
	// This allows policy builders to work with source manifests without a prior build.
	ResolveInputTypes(manifest *packages.PackageManifest, dataStreams []packages.DataStreamManifest) error
}

// NoopRequiredInputsResolver is a no-op implementation of Resolver.
// TODO: Replace with a resolver that supports test overrides (e.g. local package paths)
// when implementing local input package resolution for development and testing workflows.
type NoopRequiredInputsResolver struct{}

func (r *NoopRequiredInputsResolver) Bundle(_ string) error {
	return nil
}

func (r *NoopRequiredInputsResolver) ResolveInputTypes(_ *packages.PackageManifest, _ []packages.DataStreamManifest) error {
	return nil
}

// RequiredInputsResolver implements Resolver by downloading required input packages via an EPR client
// and applying Bundle to the built package tree.
type RequiredInputsResolver struct {
	eprClient eprClient
}

// NewRequiredInputsResolver returns a Resolver backed by eprClient. Required input packages are
// downloaded when Bundle runs.
func NewRequiredInputsResolver(eprClient eprClient) (*RequiredInputsResolver, error) {
	return &RequiredInputsResolver{
		eprClient: eprClient,
	}, nil
}

// ResolveInputTypes resolves package: references in-place on policy template inputs and
// data stream stream entries by downloading the required input packages and reading their
// input type identifiers. It is a no-op when manifest.Requires has no input dependencies.
func (r *RequiredInputsResolver) ResolveInputTypes(manifest *packages.PackageManifest, dataStreams []packages.DataStreamManifest) error {
	if manifest.Requires == nil || len(manifest.Requires.Input) == 0 {
		return nil
	}

	tmpDir, err := os.MkdirTemp("", "elastic-package-input-pkgs-*")
	if err != nil {
		return fmt.Errorf("creating temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	inputPkgPaths, err := r.mapRequiredInputPackagesPaths(manifest.Requires.Input, tmpDir)
	if err != nil {
		return err
	}

	infoByPkg := make(map[string]inputPkgInfo, len(inputPkgPaths))
	for pkgName, pkgPath := range inputPkgPaths {
		info, err := loadInputPkgInfo(pkgPath)
		if err != nil {
			return fmt.Errorf("loading input package info for %q: %w", pkgName, err)
		}
		infoByPkg[pkgName] = info
	}

	for ptIdx := range manifest.PolicyTemplates {
		for inputIdx := range manifest.PolicyTemplates[ptIdx].Inputs {
			input := &manifest.PolicyTemplates[ptIdx].Inputs[inputIdx]
			if input.Package == "" {
				continue
			}
			info, ok := infoByPkg[input.Package]
			if !ok {
				return fmt.Errorf("input package %q referenced in policy_templates[%d].inputs[%d] not found in requires.input", input.Package, ptIdx, inputIdx)
			}
			input.Type = info.identifier
			input.Package = ""
		}
	}

	for dsIdx := range dataStreams {
		for streamIdx := range dataStreams[dsIdx].Streams {
			stream := &dataStreams[dsIdx].Streams[streamIdx]
			if stream.Package == "" {
				continue
			}
			info, ok := infoByPkg[stream.Package]
			if !ok {
				return fmt.Errorf("input package %q referenced in data stream %q streams[%d] not found in requires.input", stream.Package, dataStreams[dsIdx].Name, streamIdx)
			}
			stream.Input = info.identifier
			stream.Package = ""
		}
	}

	return nil
}

// Bundle updates buildPackageRoot (a built package directory) for integrations that declare
// requires.input: it downloads those input packages, copies policy and data stream templates,
// merges variables into the integration manifest, bundles data stream field definitions, and
// replaces package: references on policy template inputs and data stream streams with the
// concrete input type from the referenced input package (last, after variable merge).
// Non-integration packages or packages without requires.input are left unchanged.
func (r *RequiredInputsResolver) Bundle(buildPackageRoot string) error {
	buildRoot, err := os.OpenRoot(buildPackageRoot)
	if err != nil {
		return fmt.Errorf("failed to open build package root: %w", err)
	}
	defer buildRoot.Close()

	manifestBytes, err := buildRoot.ReadFile("manifest.yml")
	if err != nil {
		return fmt.Errorf("failed to read package manifest: %w", err)
	}
	manifest, err := packages.ReadPackageManifestBytes(manifestBytes)
	if err != nil {
		return fmt.Errorf("failed to parse package manifest: %w", err)
	}

	// validate that the package is an integration and has required input packages
	if manifest.Type != "integration" {
		return nil
	}
	if manifest.Requires == nil || len(manifest.Requires.Input) == 0 {
		logger.Debug("Package has no required input packages, skipping required input processing")
		return nil
	}

	tmpDir, err := os.MkdirTemp("", "elastic-package-input-pkgs-*")
	if err != nil {
		return fmt.Errorf("failed to create temp directory for input packages: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	inputPkgPaths, err := r.mapRequiredInputPackagesPaths(manifest.Requires.Input, tmpDir)
	if err != nil {
		return err
	}

	if err := r.bundlePolicyTemplatesInputPackageTemplates(manifestBytes, manifest, inputPkgPaths, buildRoot); err != nil {
		return fmt.Errorf("failed to bundle policy template input package templates: %w", err)
	}

	if err := r.bundleDataStreamTemplates(inputPkgPaths, buildRoot); err != nil {
		return fmt.Errorf("failed to bundle data stream input package templates: %w", err)
	}

	if err := r.mergeVariables(manifest, inputPkgPaths, buildRoot); err != nil {
		return fmt.Errorf("merging variables from input packages: %w", err)
	}

	if err := r.bundleDataStreamFields(inputPkgPaths, buildRoot); err != nil {
		return fmt.Errorf("bundling data stream fields from input packages: %w", err)
	}

	if err := r.resolveStreamInputTypes(manifest, inputPkgPaths, buildRoot); err != nil {
		return fmt.Errorf("resolving stream input types from input packages: %w", err)
	}

	return nil
}

// downloadInputsToTmp downloads required input packages to the temporary directory.
// It returns a map of package name to zip path.
func (r *RequiredInputsResolver) mapRequiredInputPackagesPaths(manifestInputRequires []packages.PackageDependency, tmpDir string) (map[string]string, error) {
	inputPkgPaths := make(map[string]string, len(manifestInputRequires))
	errs := make([]error, 0, len(manifestInputRequires))
	for _, inputDependency := range manifestInputRequires {
		if _, ok := inputPkgPaths[inputDependency.Package]; ok {
			// skip if already downloaded
			continue
		}
		path, err := r.eprClient.DownloadPackage(inputDependency.Package, inputDependency.Version, tmpDir)
		if err != nil {
			// all required input packages must be downloaded successfully
			errs = append(errs, fmt.Errorf("failed to download input package %q: %w", inputDependency.Package, err))
			continue
		}

		// key is package name, for now we only support one version per package
		inputPkgPaths[inputDependency.Package] = path
		logger.Debugf("Resolved input package %q at %s", inputDependency.Package, path)
	}

	return inputPkgPaths, errors.Join(errs...)
}

// openPackageFS returns an fs.FS rooted at the package root (manifest.yml at
// the top level) and a close function that must be called when done. For
// directory packages it closes the os.Root; for zip packages it closes the
// underlying zip.ReadCloser.
func openPackageFS(pkgPath string) (fs.FS, func() error, error) {
	info, err := os.Stat(pkgPath)
	if err != nil {
		return nil, nil, err
	}
	if info.IsDir() {
		// open the package directory as a root
		root, err := os.OpenRoot(pkgPath)
		if err != nil {
			return nil, nil, err
		}
		return root.FS(), root.Close, nil
	}
	// open the package zip as a zip reader
	zipRC, err := zip.OpenReader(pkgPath)
	if err != nil {
		return nil, nil, err
	}
	matched, err := fs.Glob(zipRC, "*/"+packages.PackageManifestFile)
	if err != nil || len(matched) == 0 {
		zipRC.Close()
		return nil, nil, fmt.Errorf("failed to find package manifest in zip %s", pkgPath)
	}
	subFS, err := fs.Sub(zipRC, path.Dir(matched[0]))
	if err != nil {
		zipRC.Close()
		return nil, nil, err
	}
	return subFS, zipRC.Close, nil
}
