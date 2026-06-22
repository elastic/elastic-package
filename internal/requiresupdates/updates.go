// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package requiresupdates

import (
	"fmt"

	"github.com/Masterminds/semver/v3"

	"github.com/elastic/elastic-package/internal/logger"
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
	Prerelease     bool
}

// Result holds proposals from a Resolve call.
type Result struct {
	Package    string           `json:"package"`
	CodeOwner  string           `json:"codeowner,omitempty"`
	Proposals  []UpdateProposal `json:"proposals,omitempty"`
	SkipReason string           `json:"skip_reason,omitempty"` // set when the package is not applicable (not an error)
	// NewVersion is the package version after a --changelog run bumped it.
	// Empty unless the command layer set it.
	NewVersion string `json:"new_version,omitempty"`
}

func resultFromManifest(manifest *packages.PackageManifest) Result {
	return Result{
		Package:   manifest.Name,
		CodeOwner: manifest.Owner.Github,
	}
}

// Resolve reads the package manifest and resolves dependency version bumps
// without writing any files. Call Apply to produce updated manifest bytes, and
// write the result to disk only when not doing a dry-run.
func Resolve(opts Options) (*Result, error) {
	manifest, err := packages.ReadPackageManifestFromPackageRoot(opts.PackageRoot)
	if err != nil {
		return nil, fmt.Errorf("reading package manifest failed: %w", err)
	}
	result := resultFromManifest(manifest)
	if manifest.Type != integrationPackageType {
		result.SkipReason = fmt.Sprintf(
			"package type is %q; requires update only applies to integration packages with requires",
			manifest.Type,
		)
		return &result, nil
	}
	if manifest.Requires == nil || (len(manifest.Requires.Input) == 0 && len(manifest.Requires.Content) == 0) {
		result.SkipReason = "package has no requires.input or requires.content dependencies; requires update only applies to integration packages with requires"
		return &result, nil
	}

	integrationKibana := manifest.Conditions.Kibana.Version
	logger.Debugf("resolving requires for %q (type=%s, kibana=%q, input=%d, content=%d)",
		manifest.Name, manifest.Type, integrationKibana,
		len(manifest.Requires.Input), len(manifest.Requires.Content))
	proposals := make([]UpdateProposal, 0, len(manifest.Requires.Input)+len(manifest.Requires.Content))

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

	result.Proposals = proposals
	return &result, nil
}

// Apply applies the proposed version bumps in proposals to manifestBytes and
// returns the modified YAML. Proposals with an empty Proposed field are skipped.
func Apply(manifestBytes []byte, proposals []UpdateProposal) ([]byte, error) {
	for _, p := range proposals {
		if p.Proposed == "" {
			continue
		}
		logger.Debugf("applying %s.%s: %s -> %s", p.Kind, p.Package, p.Current, p.Proposed)
		var err error
		manifestBytes, err = setRequiresDependencyVersion(manifestBytes, string(p.Kind), p.Package, p.Proposed)
		if err != nil {
			return nil, fmt.Errorf("updating requires.%s for package %q: %w", p.Kind, p.Package, err)
		}
	}
	return manifestBytes, nil
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
	logger.Debugf("resolving dependency %q (kind=%s, current=%s)", dep.Package, kind, dep.Version)
	unfiltered, err := fetchAllRevisions(opts.RegistryClient, dep.Package, opts.Prerelease)
	if err != nil {
		return nil, fmt.Errorf("retrieving revisions for package %q failed: %w", dep.Package, err)
	}
	if len(unfiltered) == 0 {
		return nil, nil
	}

	compatible, err := filterByKibanaSubset(unfiltered, integrationKibana)
	if err != nil {
		return nil, err
	}
	logger.Debugf("%d/%d revisions compatible for %q", len(compatible), len(unfiltered), dep.Package)

	currentEffective, currentConstraint, err := parseCurrentVersion(kind, dep.Version)
	if err != nil {
		return nil, fmt.Errorf("package %q: %w", dep.Package, err)
	}

	isOutdatedBy := func(ver *semver.Version) bool {
		return isVersionOutdated(currentEffective, currentConstraint, ver)
	}

	var latestCompatible *packages.PackageManifest
	if currentConstraint != nil {
		latestCompatible = latestRevisionBeyondConstraint(compatible, currentConstraint)
	} else {
		latestCompatible = latestRevisionNewerThan(compatible, currentEffective)
	}

	latestUnfiltered := latestRevision(unfiltered)

	if latestCompatible == nil {
		logger.Debugf("no compatible version found for %q, checking for kibana bump warning", dep.Package)
		warning := kibanaBumpWarning(dep, latestUnfiltered, integrationKibana, nil, isOutdatedBy)
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

	latestCompatibleVer, err := semver.NewVersion(latestCompatible.Version)
	if err != nil {
		return nil, fmt.Errorf("invalid compatible version %q: %w", latestCompatible.Version, err)
	}
	if !isOutdatedBy(latestCompatibleVer) {
		warning := kibanaBumpWarning(dep, latestUnfiltered, integrationKibana, latestCompatibleVer, isOutdatedBy)
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
		Warning:          kibanaBumpWarning(dep, latestUnfiltered, integrationKibana, latestCompatibleVer, nil),
	}
	logger.Debugf("proposal for %q: %s -> %s", dep.Package, dep.Version, latestCompatible.Version)
	return proposal, nil
}

// fetchAllRevisions fetches all versions of packageName from the registry.
func fetchAllRevisions(client *registry.Client, packageName string, prerelease bool) ([]packages.PackageManifest, error) {
	revisions, err := client.Revisions(packageName, registry.SearchOptions{
		All:          true,
		Prerelease:   prerelease,
		Experimental: true,
	})
	if err != nil {
		return nil, err
	}
	logger.Debugf("fetched %d revisions for %q (prerelease=%v)", len(revisions), packageName, prerelease)
	// If no stable versions exist, fall back to pre-releases so that packages
	// that have not yet shipped a stable release are still visible. Without this,
	// resolveDependency would treat the dependency as non-existent and produce
	// neither a proposal nor a warning.
	if len(revisions) == 0 && !prerelease {
		logger.Debugf("no stable revisions for %q, retrying with prerelease", packageName)
		revisions, err = client.Revisions(packageName, registry.SearchOptions{
			All:          true,
			Prerelease:   true,
			Experimental: true,
		})
		if err != nil {
			return nil, err
		}
	}
	return revisions, nil
}

func filterByKibanaSubset(revisions []packages.PackageManifest, integrationKibana string) ([]packages.PackageManifest, error) {
	var filtered []packages.PackageManifest
	for _, rev := range revisions {
		logger.Debugf("checking %s %s: integration kibana=%q dep kibana=%q",
			rev.Name, rev.Version, integrationKibana, rev.Conditions.Kibana.Version)
		ok, err := kibanaConstraintIsSubset(integrationKibana, rev.Conditions.Kibana.Version)
		if err != nil {
			return nil, err
		}
		if ok {
			filtered = append(filtered, rev)
		}
	}
	return filtered, nil
}

// parseCurrentVersion returns the current dependency version as either an exact
// semver.Version or a semver.Constraints, depending on the dep kind and the
// string format. Input deps must be exact semver pins; content deps additionally
// accept constraint expressions (e.g. "^0.3.0").
func parseCurrentVersion(kind DependencyKind, version string) (*semver.Version, *semver.Constraints, error) {
	if kind != ContentDependency {
		v, err := semver.NewVersion(version)
		if err != nil {
			return nil, nil, fmt.Errorf("invalid requires version %q (must be an exact semver, not a constraint): %w", version, err)
		}
		return v, nil, nil
	}
	if v, err := semver.NewVersion(version); err == nil {
		return v, nil, nil
	}
	c, err := semver.NewConstraint(version)
	if err != nil {
		return nil, nil, fmt.Errorf("invalid requires version %q (not a valid semver or constraint): %w", version, err)
	}
	return nil, c, nil
}

// isVersionOutdated reports whether ver represents a version bump over the current spec.
// For an exact pin: ver must be strictly greater. For a constraint: ver must fall
// outside it (i.e. it's a newer range the constraint does not cover).
func isVersionOutdated(current *semver.Version, constraint *semver.Constraints, ver *semver.Version) bool {
	if constraint != nil {
		return !constraint.Check(ver)
	}
	return current != nil && ver.GreaterThan(current)
}

// kibanaBumpWarning returns a warning when latest is available but requires a
// Kibana version incompatible with the integration. minVer, when non-nil, gates
// the warning on latest being strictly greater than that baseline. isOutdated,
// when non-nil, additionally requires the version to represent an actual bump
// over the current dependency spec.
func kibanaBumpWarning(dep packages.PackageDependency, latest *packages.PackageManifest, integrationKibana string, minVer *semver.Version, isOutdated func(*semver.Version) bool) string {
	if latest == nil {
		return ""
	}
	v, _ := semver.NewVersion(latest.Version)
	if v == nil {
		return ""
	}
	if minVer != nil && !v.GreaterThan(minVer) {
		return ""
	}
	if isOutdated != nil && !isOutdated(v) {
		return ""
	}
	return formatKibanaBumpWarning(dep.Package, latest.Version, latest.Conditions.Kibana.Version, integrationKibana)
}

// latestRevisionWhere returns the manifest with the highest parseable semantic
// version among those for which keep returns true.
func latestRevisionWhere(revisions []packages.PackageManifest, keep func(*semver.Version) bool) *packages.PackageManifest {
	var best *packages.PackageManifest
	var bestVer *semver.Version
	for i := range revisions {
		ver, err := semver.NewVersion(revisions[i].Version)
		if err != nil || !keep(ver) {
			continue
		}
		if bestVer == nil || ver.GreaterThan(bestVer) {
			revCopy := revisions[i]
			best = &revCopy
			bestVer = ver
		}
	}
	return best
}

// latestRevision returns the revision with the highest semantic version.
// Entries with unparseable versions are skipped. Returns nil when the slice is
// empty or every entry has an unparseable version.
func latestRevision(revisions []packages.PackageManifest) *packages.PackageManifest {
	return latestRevisionWhere(revisions, func(*semver.Version) bool { return true })
}

// latestRevisionBeyondConstraint returns the latest revision whose version does
// not satisfy constraint — meaning it falls outside the currently-pinned range
// and would represent a version bump.
func latestRevisionBeyondConstraint(revisions []packages.PackageManifest, constraint *semver.Constraints) *packages.PackageManifest {
	return latestRevisionWhere(revisions, func(ver *semver.Version) bool {
		return !constraint.Check(ver)
	})
}

func latestRevisionNewerThan(revisions []packages.PackageManifest, current *semver.Version) *packages.PackageManifest {
	return latestRevisionWhere(revisions, func(ver *semver.Version) bool {
		return current == nil || ver.GreaterThan(current)
	})
}
