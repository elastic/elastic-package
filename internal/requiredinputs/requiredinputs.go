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

// Resolver bundles required input package templates into a built package tree and merges
// variables from those input packages when applicable.
type Resolver interface {
	Bundle(buildPackageRoot string) error
}

// NoopRequiredInputsResolver is a no-op implementation of Resolver.
// TODO: Replace with a resolver that supports test overrides (e.g. local package paths)
// when implementing local input package resolution for development and testing workflows.
type NoopRequiredInputsResolver struct{}

func (r *NoopRequiredInputsResolver) Bundle(_ string) error {
	return nil
}

// RequiredInputsResolver is a helper for resolving required input packages.
type RequiredInputsResolver struct {
	eprClient eprClient
}

// NewRequiredInputsResolver returns a Resolver that downloads required input packages from the registry.
func NewRequiredInputsResolver(eprClient eprClient) (*RequiredInputsResolver, error) {
	return &RequiredInputsResolver{
		eprClient: eprClient,
	}, nil
}

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
		logger.Debug("Package has no required input packages, skipping template bundling")
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

	if err := r.mergeVariables(manifest, inputPkgPaths, buildRoot); err != nil {
		return fmt.Errorf("merging variables from input packages: %w", err)
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
