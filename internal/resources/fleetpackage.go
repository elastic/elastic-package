// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package resources

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/Masterminds/semver/v3"
	"github.com/elastic/go-resource"

	"github.com/elastic/elastic-package/internal/kibana"
	"github.com/elastic/elastic-package/internal/packages"
	"github.com/elastic/elastic-package/internal/packages/installer"
)

type FleetPackage struct {
	// Provider is the name of the provider to use, defaults to "kibana".
	Provider string

	// PackageRoot is the root of the package source to install.
	PackageRoot string

	// RepositoryRoot is the root of the repository.
	RepositoryRoot *os.Root

	// Absent is set to true to indicate that the package should not be installed.
	Absent bool

	// Force forces operations, as reinstalling a package that seems to
	// be already installed.
	Force bool
}

func (f *FleetPackage) String() string {
	return fmt.Sprintf("[FleetPackage:%s:%s]", f.Provider, f.PackageRoot)
}

func (f *FleetPackage) provider(ctx resource.Context) (*KibanaProvider, error) {
	name := f.Provider
	if name == "" {
		name = DefaultKibanaProviderName
	}
	var provider *KibanaProvider
	ok := ctx.Provider(name, &provider)
	if !ok {
		return nil, fmt.Errorf("provider %q must be explicitly defined", name)
	}
	return provider, nil
}

func (f *FleetPackage) installer(ctx resource.Context) (installer.Installer, error) {
	provider, err := f.provider(ctx)
	if err != nil {
		return nil, err
	}

	return installer.NewForPackage(installer.Options{
		Kibana:         provider.Client,
		PackageRoot:    f.PackageRoot,
		SkipValidation: true,
		RepositoryRoot: f.RepositoryRoot,
	})
}

func (f *FleetPackage) Get(ctx resource.Context) (current resource.ResourceState, err error) {
	provider, err := f.provider(ctx)
	if err != nil {
		return nil, err
	}

	manifest, err := packages.ReadPackageManifestFromPackageRoot(f.PackageRoot)
	if err != nil {
		return nil, fmt.Errorf("failed to read manifest from %s: %w", f.PackageRoot, err)
	}

	fleetPackage, err := provider.Client.GetPackage(ctx, manifest.Name)
	var notFoundError *kibana.ErrPackageNotFound
	if errors.As(err, &notFoundError) {
		fleetPackage = &kibana.FleetPackage{
			Name:   manifest.Name,
			Status: "not_installed",
		}
	} else if err != nil {
		return nil, fmt.Errorf("failed to get current installation state for package %q: %w", manifest.Name, err)
	}

	kibanaVersion, err := provider.version()
	if err != nil {
		return nil, fmt.Errorf("failed to get current kibana version: %w", err)
	}

	return &FleetPackageState{
		manifest:      manifest,
		current:       fleetPackage,
		expected:      !f.Absent,
		kibanaVersion: kibanaVersion,
	}, nil
}

func (f *FleetPackage) Create(ctx resource.Context) error {
	installer, err := f.installer(ctx)
	if err != nil {
		return err
	}

	_, err = installer.Install(ctx)
	if err != nil {
		if errors.Is(err, context.Canceled) {
			provider, uninstallErr := f.provider(ctx)
			if uninstallErr != nil {
				return fmt.Errorf("failed to get client (%w) after installation failed: %w", uninstallErr, err)
			}

			// Using uninstallPachage instead of f.uninstall because we want to pass a context without cancellation.
			uninstallErr = uninstallPackage(context.WithoutCancel(ctx), provider.Client, f.PackageRoot)
			if uninstallErr != nil {
				return fmt.Errorf("failed to uninstall package (%w) after installation failed: %w", uninstallErr, err)
			}
		}
		return fmt.Errorf("installation failed: %w", err)
	}

	return nil
}

func (f *FleetPackage) uninstall(ctx resource.Context) error {
	provider, err := f.provider(ctx)
	if err != nil {
		return err
	}

	return uninstallPackage(ctx, provider.Client, f.PackageRoot)
}

func uninstallPackage(ctx context.Context, client *kibana.Client, rootPath string) error {
	manifest, err := packages.ReadPackageManifestFromPackageRoot(rootPath)
	if err != nil {
		return fmt.Errorf("failed to read manifest from %s: %w", rootPath, err)
	}

	_, err = client.RemovePackage(ctx, manifest.Name, manifest.Version)
	if err != nil {
		return fmt.Errorf("can't remove the package: %w", err)
	}
	return nil
}

func (f *FleetPackage) Update(ctx resource.Context) error {
	if f.Absent {
		return f.uninstall(ctx)
	}

	return f.Create(ctx)
}

type FleetPackageState struct {
	manifest      *packages.PackageManifest
	current       *kibana.FleetPackage
	expected      bool
	kibanaVersion *semver.Version
}

func (s *FleetPackageState) Found() bool {
	return !s.expected || (s.current != nil && s.current.Status != "not_installed")
}

func (s *FleetPackageState) NeedsUpdate(resource resource.Resource) (bool, error) {
	fleetPackage := resource.(*FleetPackage)
	if fleetPackage.Absent {
		if s.current.Status == "not_installed" {
			return fleetPackage.Force, nil
		}
		if s.manifest.Name == "system" && s.kibanaVersion.LessThan(semver.MustParse("8.0.0")) {
			// In Elastic stack 7.* , system package is installed in the default Agent policy and it cannot be deleted.
			// error: system is installed by default and cannot be removed
			return false, nil
		}
		if s.manifest.Name == "fleet_server" {
			// This package is always in use by the agent running Fleet Server.
			return false, nil
		}
	} else {
		if s.current.Status == "installed" && s.current.Version == s.manifest.Version {
			return fleetPackage.Force, nil
		}
	}

	return true, nil
}
