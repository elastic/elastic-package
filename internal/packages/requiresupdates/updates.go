// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package requiresupdates

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/Masterminds/semver/v3"

	"github.com/elastic/elastic-package/internal/packages"
	"github.com/elastic/elastic-package/internal/registry"
)

const integrationPackageType = "integration"

// DependencyKind identifies a requires block entry.
type DependencyKind string

const (
	InputDependency   DependencyKind = "input"
	ContentDependency DependencyKind = "content"
)

// UpdateProposal describes a single dependency version bump.
type UpdateProposal struct {
	Kind             DependencyKind `json:"kind"`
	Package          string         `json:"package"`
	Current          string         `json:"current"`
	Proposed         string         `json:"proposed"`
	Warning          string         `json:"warning,omitempty"`
	KibanaConstraint string         `json:"kibana_constraint,omitempty"`
}

// Options configures a requires update run.
type Options struct {
	PackageRoot    string
	RegistryClient *registry.Client
	DryRun         bool
}

// Result holds proposals and whether manifest.yml was written.
type Result struct {
	Package    string           `json:"package"`
	CodeOwner  string           `json:"codeowner,omitempty"`
	Proposals  []UpdateProposal `json:"proposals,omitempty"`
	Applied    bool             `json:"applied,omitempty"`
	SkipReason string           `json:"skip_reason,omitempty"` // set when the package is not applicable (not an error)
}

func resultFromManifest(manifest *packages.PackageManifest) Result {
	return Result{
		Package:   manifest.Name,
		CodeOwner: manifest.Owner.Github,
	}
}

// Update resolves and optionally applies dependency version bumps for an integration package with requires.
func Update(opts Options) (*Result, error) {
	manifest, err := packages.ReadPackageManifestFromPackageRoot(opts.PackageRoot)
	if err != nil {
		return nil, fmt.Errorf("reading package manifest failed: %w", err)
	}
	if manifest.Type != integrationPackageType {
		result := resultFromManifest(manifest)
		result.SkipReason = fmt.Sprintf(
			"package type is %q; requires update only applies to integration packages with requires",
			manifest.Type,
		)
		return &result, nil
	}
	if manifest.Requires == nil || (len(manifest.Requires.Input) == 0 && len(manifest.Requires.Content) == 0) {
		result := resultFromManifest(manifest)
		result.SkipReason = "package has no requires.input or requires.content dependencies; requires update only applies to integration packages with requires"
		return &result, nil
	}

	integrationKibana := manifest.Conditions.Kibana.Version
	var proposals []UpdateProposal

	inputProposals, err := resolveSection(opts, integrationKibana, InputDependency, manifest.Requires.Input)
	if err != nil {
		return nil, err
	}
	proposals = append(proposals, inputProposals...)

	contentProposals, err := resolveSection(opts, integrationKibana, ContentDependency, manifest.Requires.Content)
	if err != nil {
		return nil, err
	}
	proposals = append(proposals, contentProposals...)

	if len(proposals) == 0 {
		result := resultFromManifest(manifest)
		return &result, nil
	}

	if opts.DryRun {
		result := resultFromManifest(manifest)
		result.Proposals = proposals
		return &result, nil
	}

	manifestPath := filepath.Join(opts.PackageRoot, packages.PackageManifestFile)
	manifestBytes, err := os.ReadFile(manifestPath)
	if err != nil {
		return nil, fmt.Errorf("reading manifest file failed: %w", err)
	}
	for _, p := range proposals {
		if p.Proposed == "" {
			continue
		}
		manifestBytes, err = SetRequiresDependencyVersion(manifestBytes, string(p.Kind), p.Package, p.Proposed)
		if err != nil {
			return nil, fmt.Errorf("updating requires.%s for package %q: %w", p.Kind, p.Package, err)
		}
	}
	if err := os.WriteFile(manifestPath, manifestBytes, 0o644); err != nil {
		return nil, fmt.Errorf("writing manifest file failed: %w", err)
	}

	result := resultFromManifest(manifest)
	result.Proposals = proposals
	result.Applied = true
	return &result, nil
}

func resolveSection(opts Options, integrationKibana string, kind DependencyKind, deps []packages.PackageDependency) ([]UpdateProposal, error) {
	if len(deps) == 0 {
		return nil, nil
	}
	var proposals []UpdateProposal
	for _, dep := range deps {
		proposal, err := resolveDependency(opts, integrationKibana, kind, dep)
		if err != nil {
			return nil, err
		}
		if proposal != nil {
			proposals = append(proposals, *proposal)
		}
	}
	return proposals, nil
}

func resolveDependency(opts Options, integrationKibana string, kind DependencyKind, dep packages.PackageDependency) (*UpdateProposal, error) {
	unfiltered, err := opts.RegistryClient.Revisions(dep.Package, registry.SearchOptions{
		Prerelease:   true,
		Experimental: true,
	})
	if err != nil {
		return nil, fmt.Errorf("retrieving revisions for package %q failed: %w", dep.Package, err)
	}
	if len(unfiltered) == 0 {
		return nil, nil
	}

	compatible, err := fetchCompatibleRevisions(opts, integrationKibana, dep.Package)
	if err != nil {
		return nil, err
	}

	currentEffective, err := effectiveCurrentVersion(dep.Version)
	if err != nil {
		return nil, fmt.Errorf("package %q: %w", dep.Package, err)
	}

	latestCompatible := latestRevisionNewerThan(compatible, currentEffective)

	latestUnfiltered, err := latestRevision(unfiltered)
	if err != nil {
		return nil, fmt.Errorf("package %q: %w", dep.Package, err)
	}

	var warning string
	if latestCompatible == nil {
		if latestUnfiltered != nil && currentEffective != nil {
			latestVer, _ := semver.NewVersion(latestUnfiltered.Version)
			if latestVer != nil && latestVer.GreaterThan(currentEffective) {
				warning = formatKibanaBumpWarning(
					dep.Package,
					latestUnfiltered.Version,
					latestUnfiltered.Conditions.Kibana.Version,
					integrationKibana,
				)
			}
		}
		if warning != "" {
			return &UpdateProposal{
				Kind:    kind,
				Package: dep.Package,
				Current: dep.Version,
				Warning: warning,
			}, nil
		}
		return nil, nil
	}

	latestCompatibleVer, err := semver.NewVersion(latestCompatible.Version)
	if err != nil {
		return nil, fmt.Errorf("invalid compatible version %q: %w", latestCompatible.Version, err)
	}
	if currentEffective != nil && !latestCompatibleVer.GreaterThan(currentEffective) {
		if latestUnfiltered != nil {
			latestUnfilteredVer, _ := semver.NewVersion(latestUnfiltered.Version)
			if latestUnfilteredVer != nil && latestUnfilteredVer.GreaterThan(currentEffective) &&
				latestUnfilteredVer.GreaterThan(latestCompatibleVer) {
				warning = formatKibanaBumpWarning(
					dep.Package,
					latestUnfiltered.Version,
					latestUnfiltered.Conditions.Kibana.Version,
					integrationKibana,
				)
			}
		}
		if warning == "" {
			return nil, nil
		}
		return &UpdateProposal{
			Kind:    kind,
			Package: dep.Package,
			Current: dep.Version,
			Warning: warning,
		}, nil
	}

	proposal := &UpdateProposal{
		Kind:             kind,
		Package:          dep.Package,
		Current:          dep.Version,
		Proposed:         latestCompatible.Version,
		KibanaConstraint: latestCompatible.Conditions.Kibana.Version,
	}

	if latestUnfiltered != nil && latestUnfiltered.Version != latestCompatible.Version {
		latestUnfilteredVer, err := semver.NewVersion(latestUnfiltered.Version)
		if err == nil && latestUnfilteredVer.GreaterThan(latestCompatibleVer) {
			proposal.Warning = formatKibanaBumpWarning(
				dep.Package,
				latestUnfiltered.Version,
				latestUnfiltered.Conditions.Kibana.Version,
				integrationKibana,
			)
		}
	}

	return proposal, nil
}

func fetchCompatibleRevisions(opts Options, integrationKibana, packageName string) ([]packages.PackageManifest, error) {
	kibanaVersions := deriveEPRKibanaVersions(integrationKibana)
	if len(kibanaVersions) == 0 {
		revisions, err := opts.RegistryClient.Revisions(packageName, registry.SearchOptions{
			Prerelease:   true,
			Experimental: true,
		})
		if err != nil {
			return nil, err
		}
		return filterByKibanaOverlap(revisions, integrationKibana)
	}

	byVersion := make(map[string]packages.PackageManifest)
	for _, kv := range kibanaVersions {
		revisions, err := opts.RegistryClient.Revisions(packageName, registry.SearchOptions{
			KibanaVersion: kv,
			Prerelease:    true,
			Experimental:  true,
		})
		if err != nil {
			return nil, fmt.Errorf("retrieving revisions for kibana.version=%s failed: %w", kv, err)
		}
		for _, rev := range revisions {
			byVersion[rev.Version] = rev
		}
	}
	merged := make([]packages.PackageManifest, 0, len(byVersion))
	for _, rev := range byVersion {
		merged = append(merged, rev)
	}
	sort.SliceStable(merged, func(i, j int) bool {
		vi, _ := semver.NewVersion(merged[i].Version)
		vj, _ := semver.NewVersion(merged[j].Version)
		if vi == nil {
			return true
		}
		if vj == nil {
			return false
		}
		return vi.LessThan(vj)
	})
	return filterByKibanaOverlap(merged, integrationKibana)
}

func filterByKibanaOverlap(revisions []packages.PackageManifest, integrationKibana string) ([]packages.PackageManifest, error) {
	if integrationKibana == "" {
		return revisions, nil
	}
	var filtered []packages.PackageManifest
	for _, rev := range revisions {
		ok, err := kibanaConstraintsOverlap(integrationKibana, rev.Conditions.Kibana.Version)
		if err != nil {
			return nil, err
		}
		if ok {
			filtered = append(filtered, rev)
		}
	}
	return filtered, nil
}

func effectiveCurrentVersion(pinned string) (*semver.Version, error) {
	ver, err := semver.NewVersion(pinned)
	if err != nil {
		return nil, fmt.Errorf("invalid requires version %q (must be an exact semver, not a constraint): %w", pinned, err)
	}
	return ver, nil
}

func latestRevision(revisions []packages.PackageManifest) (*packages.PackageManifest, error) {
	if len(revisions) == 0 {
		return nil, nil
	}
	best := revisions[len(revisions)-1]
	bestVer, err := semver.NewVersion(best.Version)
	if err != nil {
		return nil, fmt.Errorf("invalid version %q: %w", best.Version, err)
	}
	for i := len(revisions) - 2; i >= 0; i-- {
		ver, err := semver.NewVersion(revisions[i].Version)
		if err != nil {
			continue
		}
		if ver.GreaterThan(bestVer) {
			best = revisions[i]
			bestVer = ver
		}
	}
	return &best, nil
}

func latestRevisionNewerThan(revisions []packages.PackageManifest, current *semver.Version) *packages.PackageManifest {
	var best *packages.PackageManifest
	var bestVer *semver.Version
	for _, rev := range revisions {
		ver, err := semver.NewVersion(rev.Version)
		if err != nil {
			continue
		}
		if current != nil && !ver.GreaterThan(current) {
			continue
		}
		if bestVer == nil || ver.GreaterThan(bestVer) {
			copy := rev
			best = &copy
			bestVer = ver
		}
	}
	return best
}
