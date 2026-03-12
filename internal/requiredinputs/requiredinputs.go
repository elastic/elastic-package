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

type registryClient interface {
	DownloadPackage(packageName string, packageVersion string, tmpDir string) (string, error)
}

// InputRequiredResolver is a helper for resolving required input packages.
type InputRequiredResolver struct {
	eprClient registryClient
	tmpDir    string   // temporary directory for input packages only used for downloading input packages
	buildRoot *os.Root // root directory of the build package where we will bundle input package templates and update manifests
}

// NewInputRequiredResolver creates a new InputRequiredResolver.
// It creates a temporary directory for input packages and returns a new InputRequiredResolver.
func NewInputRequiredResolver(eprClient registryClient, buildPackageRoot string) (*InputRequiredResolver, error) {
	tmpDir, err := os.MkdirTemp("", "elastic-package-input-pkgs-*")
	if err != nil {
		return nil, fmt.Errorf("creating temp directory for input packages: %w", err)
	}
	buildRoot, err := os.OpenRoot(buildPackageRoot)
	if err != nil {
		return nil, fmt.Errorf("opening build package root: %w", err)
	}
	return &InputRequiredResolver{
		eprClient: eprClient,
		tmpDir:    tmpDir,
		buildRoot: buildRoot,
	}, nil
}

func (r *InputRequiredResolver) Cleanup() error {
	return errors.Join(
		os.RemoveAll(r.tmpDir),
		r.buildRoot.Close(),
	)
}

func (r *InputRequiredResolver) BundleInputPackageTemplates() error {

	manifestBytes, err := r.buildRoot.ReadFile("manifest.yml")
	if err != nil {
		return fmt.Errorf("reading package manifest: %w", err)
	}
	manifest, err := packages.ReadPackageManifestBytes(manifestBytes)
	if err != nil {
		return fmt.Errorf("reading package manifest: %w", err)
	}

	// validate that the package is an integration and has required input packages
	if manifest.Type != "integration" {
		return nil
	}
	if manifest.Requires == nil || len(manifest.Requires.Input) == 0 {
		logger.Debug("Package has no required input packages, skipping template bundling")
		return nil
	}

	inputPkgPaths, err := r.downloadInputsToTmp(manifest.Requires.Input)
	if err != nil {
		return err
	}

	if err := r.bundlePolicyTemplatesInputPackageTemplates(manifestBytes, manifest, inputPkgPaths); err != nil {
		return fmt.Errorf("bundling policy template input package templates: %w", err)
	}

	if err := r.bundleDataStreamTemplates(inputPkgPaths); err != nil {
		return fmt.Errorf("bundling data stream input package templates: %w", err)
	}

	return nil
}

// downloadInputsToTmp downloads required input packages to the temporary directory.
// It returns a map of package name to zip path.
func (r *InputRequiredResolver) downloadInputsToTmp(manifestInputRequires []packages.PackageDependency) (map[string]string, error) {
	inputPkgPaths := make(map[string]string, len(manifestInputRequires))
	errs := make([]error, 0, len(manifestInputRequires))
	for _, inputDependency := range manifestInputRequires {
		if _, ok := inputPkgPaths[inputDependency.Package]; ok {
			// skip if already downloaded
			continue
		}
		zipPath, err := r.eprClient.DownloadPackage(inputDependency.Package, inputDependency.Version, r.tmpDir)
		if err != nil {
			// all required input packages must be downloaded successfully
			errs = append(errs, fmt.Errorf("downloading input package %q: %w", inputDependency.Package, err))
			continue
		}

		// key is package name, for now we only support one version per package
		inputPkgPaths[inputDependency.Package] = zipPath
		logger.Debugf("Resolved input package %q at %s", inputDependency.Package, zipPath)
	}

	return inputPkgPaths, errors.Join(errs...)
}

// openPackageFS returns an fs.FS rooted at the package root (manifest.yml at
// the top level) and a close function that must be called when done. For
// directory packages the close function is a no-op; for zip packages it closes
// the underlying zip.ReadCloser.
func openPackageFS(pkgPath string) (fs.FS, func() error, error) {
	info, err := os.Stat(pkgPath)
	if err != nil {
		return nil, nil, err
	}
	if info.IsDir() {
		return os.DirFS(pkgPath), func() error { return nil }, nil
	}
	zipRC, err := zip.OpenReader(pkgPath)
	if err != nil {
		return nil, nil, err
	}
	matched, err := fs.Glob(zipRC, "*/"+packages.PackageManifestFile)
	if err != nil || len(matched) == 0 {
		zipRC.Close()
		return nil, nil, fmt.Errorf("no manifest found in zip %s", pkgPath)
	}
	subFS, err := fs.Sub(zipRC, path.Dir(matched[0]))
	if err != nil {
		zipRC.Close()
		return nil, nil, err
	}
	return subFS, zipRC.Close, nil
}
