// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package files

import (
	"fmt"
	"path/filepath"
	"slices"

	"github.com/elastic/elastic-package/internal/logger"
	"github.com/elastic/elastic-package/internal/packages"
	"github.com/elastic/package-spec/v3/code/go/pkg/linkedfiles"
)

// AreLinkedFilesUpToDate function checks if all the linked files are up-to-date.
func AreLinkedFilesUpToDate(fromDir string) ([]linkedfiles.Link, error) {
	root, err := linkedfiles.FindRepositoryRoot()
	if err != nil {
		return nil, err
	}

	fromRel, err := func() (string, error) {
		if filepath.IsAbs(fromDir) {
			return filepath.Rel(root.Name(), fromDir)
		}
		return fromDir, nil
	}()
	if err != nil {
		return nil, err
	}

	links, err := linkedfiles.ListLinkedFilesInRoot(root, fromRel)
	if err != nil {
		return nil, fmt.Errorf("including linked files failed: %w", err)
	}

	var outdated []linkedfiles.Link
	for _, l := range links {
		logger.Debugf("Check if %s is up-to-date", l.LinkFilePath)
		if !l.UpToDate {
			outdated = append(outdated, l)
		}
	}

	return outdated, nil
}

// UpdateLinkedFilesChecksums function updates the checksums of the linked files.
// It returns a slice of updated links.
// If no links were updated, it returns an empty slice.
func UpdateLinkedFilesChecksums(fromDir string) ([]linkedfiles.Link, error) {
	root, err := linkedfiles.FindRepositoryRoot()
	if err != nil {
		return nil, err
	}

	fromRel, err := func() (string, error) {
		if filepath.IsAbs(fromDir) {
			return filepath.Rel(root.Name(), fromDir)
		}
		return fromDir, nil
	}()
	if err != nil {
		return nil, err
	}

	links, err := linkedfiles.ListLinkedFilesInRoot(root, fromRel)
	if err != nil {
		return nil, fmt.Errorf("updating linked files checksums failed: %w", err)
	}

	var updatedLinks []linkedfiles.Link
	for _, l := range links {
		updated, err := l.UpdateChecksum()
		if err != nil {
			return nil, fmt.Errorf("updating linked files checksums failed: %w", err)
		}
		if updated {
			updatedLinks = append(updatedLinks, l)
		}
	}

	return updatedLinks, nil
}

// LinkedFilesByPackageFrom function returns a slice of maps containing linked files grouped by package.
// Each map contains the package name as the key and a slice of linked file paths as the value.
func LinkedFilesByPackageFrom(fromDir string) ([]map[string][]string, error) {
	root, err := linkedfiles.FindRepositoryRoot()
	if err != nil {
		return nil, err
	}
	links, err := linkedfiles.ListLinkedFilesInRoot(root, ".")
	if err != nil {
		return nil, fmt.Errorf("including linked files failed: %w", err)
	}

	packageRoot, _, _ := packages.FindPackageRootFrom(fromDir)
	packageName := filepath.Base(packageRoot)
	byPackageMap := map[string][]string{}
	for _, l := range links {
		linkPackageRoot, _, _ := packages.FindPackageRootFrom(filepath.Join(root.Name(), l.LinkFilePath))
		linkPackageName := filepath.Base(linkPackageRoot)
		includedPackageRoot, _, _ := packages.FindPackageRootFrom(filepath.Join(root.Name(), l.IncludedFilePath))
		includedPackageName := filepath.Base(includedPackageRoot)
		if linkPackageName == includedPackageName ||
			packageName != includedPackageName {
			continue
		}
		byPackageMap[linkPackageName] = append(byPackageMap[linkPackageName], l.LinkFilePath)
	}

	var packages []string
	for p := range byPackageMap {
		packages = append(packages, p)
	}
	slices.Sort(packages)

	var byPackage []map[string][]string
	for _, p := range packages {
		m := map[string][]string{p: byPackageMap[p]}
		byPackage = append(byPackage, m)
	}
	return byPackage, nil
}
