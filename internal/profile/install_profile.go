// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package profile

import (
	"fmt"
	"io/ioutil"
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
		options.PackagePath = loc.StackDir()
	}

	if options.FromProfile == "" {
		return createProfile(options)
	}

	return createProfileFrom(options)
}

// LoadProfile loads an existing profile from the default elastic-package config dir
func LoadProfile(profileName string) (*Profile, error) {
	loc, err := locations.NewLocationManager()
	if err != nil {
		return nil, errors.Wrap(err, "error finding stack dir location")
	}

	return loadProfile(loc.StackDir(), profileName)
}

// DeleteProfile deletes a profile from the default elastic-package config dir
func DeleteProfile(profileName string) error {
	loc, err := locations.NewLocationManager()
	if err != nil {
		return errors.Wrap(err, "error finding stack dir location")
	}
	return deleteProfile(loc.StackDir(), profileName)
}

// FetchAllProfiles returns a list of profile values
func FetchAllProfiles(elasticPackagePath string) ([]Metadata, error) {
	dirList, err := ioutil.ReadDir(elasticPackagePath)
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
// On true, it'll overwrite the profile, on false, it'll copy the existing profile over to profileName_old
func createProfile(options Options) error {
	profile, err := NewConfigProfile(options.PackagePath, options.Name)
	if err != nil {
		return errors.Wrap(err, "error creating profile")
	}

	// check to see if we have an existing profile at that location.
	exists, err := profile.alreadyExists()
	if err != nil {
		return errors.Wrap(err, "error checking for existing profile")
	}
	if exists {
		localChanges, err := profile.localFilesChanged()
		if err != nil {
			return errors.Wrapf(err, "error checking for changes in %s", profile.ProfilePath)
		}

		// If there are changes and we've selected CreateNew, move the old path
		if localChanges && !options.OverwriteExisting {
			if localChanges && options.Name == DefaultProfile {
				logger.Warn("default profile has been changed by user or updated by elastic-package. The current profile will be moved.")
			}
			err = updateExistingDefaultProfile(options.PackagePath)
			if err != nil {
				return errors.Wrap(err, "error moving old profile")
			}
			err = os.Mkdir(profile.ProfilePath, 0755)
			if err != nil {
				return errors.Wrapf(err, "error crating profile directory %s", profile.ProfilePath)
			}
		}
	} else {
		err = os.Mkdir(profile.ProfilePath, 0755)
		if err != nil {
			return errors.Wrapf(err, "error crating profile directory %s", profile.ProfilePath)
		}
	}
	if err != nil {
		return errors.Wrapf(err, "stat file failed (path: %s)", profile.ProfilePath)
	}

	// write the resources
	err = profile.writeProfileResources()
	if err != nil {
		return errors.Wrap(err, "error writing profile file")
	}
	return nil
}

// updateExistingDefaultProfile migrates the old default profile to profile_old
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
	meta.From = newFilePath

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

// createProfileFrom creates a new profile by copying over an existing profile
func createProfileFrom(option Options) error {
	fromProfile, err := loadProfile(option.PackagePath, option.FromProfile)
	if err != nil {
		return errors.Wrapf(err, "error loading %s profile", option.FromProfile)
	}

	newProfile, err := NewConfigProfile(option.PackagePath, option.Name)
	if err != nil {
		return errors.Wrapf(err, "error creating %s profile", option.Name)
	}

	newExists, err := newProfile.alreadyExists()
	if err != nil {
		return errors.Wrapf(err, "error checking profile %s", newProfile.ProfilePath)
	}
	if newExists {
		return errors.Errorf("profile %s already exists", newProfile.profileName)
	}
	os.Mkdir(newProfile.ProfilePath, 0755)

	newProfile.overwrite(fromProfile.configFiles)
	return newProfile.writeProfileResources()

}

// deleteProfile deletes a given config profile.
func deleteProfile(elasticPackagePath string, profileName string) error {
	if profileName == DefaultProfile {
		return errors.New("cannot remove default profile")
	}

	pathToDelete := filepath.Join(elasticPackagePath, profileName)

	return os.RemoveAll(pathToDelete)

}
