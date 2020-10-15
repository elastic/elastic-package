// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package promote

import (
	"encoding/hex"
	"fmt"
	"io/ioutil"
	"sort"
	"strings"

	"github.com/cespare/xxhash/v2"
	"github.com/go-git/go-billy/v5"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/pkg/errors"
)

// SignedPackageVersion represents a package version stored in the package-storage with a calculated signature.
type SignedPackageVersion struct {
	PackageVersion

	Signature string
}

// NewSignedPackageVersion constructs a new SignedPackageVersion struct composed of the package version and signature
func NewSignedPackageVersion(version PackageVersion, signature string) SignedPackageVersion {
	return SignedPackageVersion{PackageVersion: version, Signature: signature}
}

// String method returns a string representation of the SignedPackageVersion.
func (s *SignedPackageVersion) String() string {
	return fmt.Sprintf("%s: %s", s.PackageVersion.String(), s.Signature)
}

// SignedPackageVersions is an array of SignedPackageVersion.
type SignedPackageVersions []SignedPackageVersion

// Strings method returns an array of string representations.
func (s SignedPackageVersions) Strings() []string {
	var entries []string
	for _, pr := range s {
		entries = append(entries, pr.String())
	}
	return entries
}

// CalculatePackageSignatures computes the combined sha1 hash for all the files in the package
// this is equivalent to doing find <package> -type f -exec <hash tool> {} + | awk '{print $1}' | sort | <hash tool>
// on the package version directory
func CalculatePackageSignatures(r *git.Repository, branch string, packageVersions PackageVersions) (SignedPackageVersions, error) {
	wt, err := r.Worktree()
	if err != nil {
		return nil, errors.Wrap(err, "fetching worktree reference failed while calculating directory hash")
	}

	err = wt.Checkout(&git.CheckoutOptions{
		Branch: plumbing.NewBranchReferenceName(branch),
	})
	if err != nil {
		return nil, errors.Wrapf(err, "changing branch failed (path: %s) while calculating directory hash", branch)
	}

	var signedPackages SignedPackageVersions
	for _, version := range packageVersions {
		resources, err := walkPackageResources(wt.Filesystem, version.path())
		if err != nil {
			return nil, errors.Wrap(err, "failed to retrieve package paths while calculating directory hash")
		}

		signature, err := calculateFilesSignature(wt.Filesystem, resources)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to calculate the package signature for %s", version.Name)
		}
		signedPackages = append(signedPackages, NewSignedPackageVersion(version, signature))
	}

	return signedPackages, nil
}

// calculateFilesSignature computes the sha1 hash of a list of files
// First it computes the hash of each of the file's contents then it sorts those
// encoded strings, creates a final string with the sorted file hashes delimited by a newline
// and hashes the final string.
// This effectively produces a hash of a directory
// It is equivalent to: find <path> -type f -exec <hash tool> {} + | awk '{print $1}' | sort | <hash tool>
func calculateFilesSignature(filesystem billy.Filesystem, files []string) (string, error) {
	var fileHashes []string
	for _, file := range files {
		if strings.Contains(file, "\n") {
			return "", errors.New("dirhash: filenames with newlines are not supported")
		}

		f, err := filesystem.Open(file)
		if err != nil {
			return "", errors.Wrapf(err, "reading file failed (path: %s)", file)
		}

		c, err := ioutil.ReadAll(f)
		if err != nil {
			return "", errors.Wrapf(err, "reading file content failed (path: %s)", file)
		}

		fileHash := xxhash.New()
		fileHash.Write(c)
		fileHashes = append(fileHashes, hex.EncodeToString(fileHash.Sum(nil)))
	}

	sort.Strings(fileHashes)
	var builder strings.Builder
	for _, fileHash := range fileHashes {
		builder.WriteString(fmt.Sprintf("%s\n", fileHash))
	}
	combinedHash := xxhash.New()
	combinedHash.Write([]byte(builder.String()))
	return hex.EncodeToString(combinedHash.Sum(nil)), nil
}
