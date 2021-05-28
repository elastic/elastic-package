// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package profile

import (
	"encoding/json"
	"os/user"
	"path/filepath"
	"time"

	"github.com/pkg/errors"

	"github.com/elastic/elastic-package/internal/version"
)

// Metadata stores the data associated with a given profile
type Metadata struct {
	Name        string    `json:"name"`
	DateCreated time.Time `json:"date_created"`
	User        string    `json:"user"`
	Version     string    `json:"version"`
	Path        string    `json:"path"`
}

// PackageProfileMetaFile is the filename of the profile metadata file
const PackageProfileMetaFile configFile = "profile.json"

// createProfileMetadata creates the body of the profile.json file
func createProfileMetadata(profileName string, profilePath string) (*simpleFile, error) {
	currentUser, err := user.Current()
	if err != nil {
		return nil, errors.Wrap(err, "error fetching current user")
	}

	profileData := Metadata{
		profileName,
		time.Now(),
		currentUser.Username,
		version.CommitHash,
		profilePath,
	}

	jsonRaw, err := json.MarshalIndent(profileData, "", "  ")
	if err != nil {
		return nil, errors.Wrap(err, "error marshalling json")
	}

	return &simpleFile{
		name: string(PackageProfileMetaFile),
		path: filepath.Join(profilePath, string(PackageProfileMetaFile)),
		body: string(jsonRaw),
	}, nil
}
