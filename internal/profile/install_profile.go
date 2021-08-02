// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package profile

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/pkg/errors"

	"github.com/elastic/elastic-package/internal/configuration/locations"
	"github.com/elastic/elastic-package/internal/logger"
)

// CreateProfile creates an existing profile from the default elastic-package config dir
// if option.Package has a value it'll be used, if not, the default location will be used
// if option.From does not have a supplied value, it'll create a default profile.
func CreateProfile(options Options) error {
	if options.PackagePath == "" {
		loc, err := locations.NewLocationManager()
		if err != nil {
			return errors.Wrap(err, "error finding stack dir location")
		}
		options.PackagePath = loc.ProfileDir()
	}

	// If they're creating from Default, assume they want the actual default, and
	// not whatever is currently inside default.
	if options.FromProfile == "" || options.FromProfile == DefaultProfile {
		return createProfile(options)
	}

	return createProfileFrom(options)
}

// MigrateProfileFiles creates a new profile based on existing filepaths
// that are stored elsewhere outside the profile system.
func MigrateProfileFiles(options Options, files []string) error {
	profile, err := newProfileFromExistingFiles(options.PackagePath, options.Name, files, true)
	if err != nil {
		return errors.Wrap(err, "error creating new profile from files")
	}
	err = os.Mkdir(profile.ProfilePath, 0755)
	if err != nil {
		return errors.Wrapf(err, "error crating profile directory %s", profile.ProfilePath)
	}
	err = profile.writeProfileResources()
	if err != nil {
		return errors.Wrap(err, "error writing out new profile config")
	}
	return nil
}

// LoadProfile loads an existing profile from the default elastic-package config dir
func LoadProfile(profileName string) (*Profile, error) {
	loc, err := locations.NewLocationManager()
	if err != nil {
		return nil, errors.Wrap(err, "error finding stack dir location")
	}

	return loadProfile(loc.ProfileDir(), profileName)
}

// DeleteProfile deletes a profile from the default elastic-package config dir
func DeleteProfile(profileName string) error {
	loc, err := locations.NewLocationManager()
	if err != nil {
		return errors.Wrap(err, "error finding stack dir location")
	}
	return deleteProfile(loc.ProfileDir(), profileName)
}

// FetchAllProfiles returns a list of profile values
func FetchAllProfiles(elasticPackagePath string) ([]Metadata, error) {
	dirList, err := os.ReadDir(elasticPackagePath)
	if err != nil {
		return []Metadata{}, errors.Wrapf(err, "error reading from directory %s", elasticPackagePath)
	}

	var profiles []Metadata
	// TODO: this should read a profile.json file or something like that
	for _, item := range dirList {
		if !item.IsDir() {
			continue
		}
		profile, err := loadProfile(elasticPackagePath, item.Name())
		if errors.Is(err, ErrNotAProfile) {
			continue
		}
		if err != nil {
			return profiles, errors.Wrapf(err, "error loading profile %s", item.Name())
		}
		metadata, err := profile.metadata()
		if err != nil {
			return profiles, errors.Wrap(err, "error reading profile metadata")
		}
		profiles = append(profiles, metadata)
	}
	return profiles, nil
}

// createProfile installs a new profile at the given package path location.
// overwriteExisting determines the behavior if a profile with the given name already exists.
// On true, it'll overwrite the profile, on false, it'll backup the existing profile to profilename_VERSION-DATE-CREATED
func createProfile(options Options) error {
	profile, err := createAndCheckProfile(options.PackagePath, options.Name, options.OverwriteExisting)
	if err != nil {
		return errors.Wrap(err, "error creating new profile")
	}
	// write the resources
	err = profile.writeProfileResources()
	if err != nil {
		return errors.Wrap(err, "error writing profile file")
	}
	return nil
}

// createProfileFrom creates a new profile by copying over an existing profile
func createProfileFrom(options Options) error {
	fromProfile, err := loadProfile(options.PackagePath, options.FromProfile)
	if err != nil {
		return errors.Wrapf(err, "error loading %s profile", options.FromProfile)
	}

	newProfile, err := createAndCheckProfile(options.PackagePath, options.Name, options.OverwriteExisting)
	if err != nil {
		return errors.Wrap(err, "error creating new profile")
	}

	newProfile.overwrite(fromProfile.configFiles)
	err = newProfile.writeProfileResources()
	if err != nil {
		return errors.Wrap(err, "error writing new profile")
	}
	return nil
}

// createAndCheckProfile does most of the heavy lifting for initializing a new profile,
// including dealing with profile overwrites
func createAndCheckProfile(packagePath, packageName string, overwriteExisting bool) (*Profile, error) {
	profile, err := NewConfigProfile(packagePath, packageName)
	if err != nil {
		return nil, errors.Wrap(err, "error creating profile")
	}

	// check to see if we have an existing profile at that location.
	exists, err := profile.alreadyExists()
	if err != nil {
		return nil, errors.Wrap(err, "error checking for existing profile")
	}
	if exists {
		localChanges, err := profile.localFilesChanged()
		if err != nil {
			return nil, errors.Wrapf(err, "error checking for changes in %s", profile.ProfilePath)
		}
		// If there are changes and we've selected CreateNew, move the old path
		if localChanges && !overwriteExisting {
			if localChanges && packageName == DefaultProfile {
				logger.Warn("Default profile has been changed by user or updated by elastic-package. The current profile will be moved.")
			}
			// Migrate the existing profile
			err = updateExistingDefaultProfile(packagePath)
			if err != nil {
				return nil, errors.Wrap(err, "error moving old profile")
			}
			err = os.Mkdir(profile.ProfilePath, 0755)
			if err != nil {
				return nil, errors.Wrapf(err, "error crating profile directory %s", profile.ProfilePath)
			}
			err = os.Mkdir(profile.ProfileStackPath, 0755)
			if err != nil {
				return nil, errors.Wrapf(err, "error crating profile directory %s", profile.ProfilePath)
			}
		}
	} else {
		err = os.Mkdir(profile.ProfilePath, 0755)
		if err != nil {
			return nil, errors.Wrapf(err, "error crating profile directory %s", profile.ProfilePath)
		}
		err = os.Mkdir(profile.ProfileStackPath, 0755)
		if err != nil {
			return nil, errors.Wrapf(err, "error crating profile directory %s", profile.ProfilePath)
		}
	}

	return profile, nil
}

// updateExistingDefaultProfile migrates the old default profile to profile_VERSION_DATE-CREATED
func updateExistingDefaultProfile(path string) error {
	profile, err := NewConfigProfile(path, DefaultProfile)
	if err != nil {
		return errors.Wrap(err, "error creating profile")
	}
	meta, err := profile.metadata()
	if err != nil {
		return errors.Wrap(err, "error updating metadata")
	}
	newName := fmt.Sprintf("default_%s_%d", meta.Version, meta.DateCreated.Unix())
	newFilePath := filepath.Join(filepath.Dir(profile.ProfilePath), newName)
	meta.Name = newName
	meta.Path = newFilePath

	err = profile.updateMetadata(meta)
	if err != nil {
		return errors.Wrap(err, "error updating metadata")
	}

	err = os.Rename(profile.ProfilePath, newFilePath)
	if err != nil {
		return errors.Wrap(err, "error moving default profile")
	}
	return nil
}

// deleteProfile deletes a given config profile.
func deleteProfile(elasticPackagePath string, profileName string) error {
	if profileName == DefaultProfile {
		return errors.New("cannot remove default profile")
	}

	pathToDelete := filepath.Join(elasticPackagePath, profileName)

	return os.RemoveAll(pathToDelete)

}
