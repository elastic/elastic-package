// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package profile

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/pkg/errors"

	"github.com/elastic/elastic-package/internal/locations"
)

// CreateProfile installs a new profile at the given package path location.
// overwriteExisting determines the behavior if a profile with the given name already exists.
// On true, it'll overwrite the profile, on false, it'll copy the existing profile over to profileName_old
func CreateProfile(elasticPackagePath string, profileName string, overwriteExisting bool) error {

	profile, err := NewConfigProfile(elasticPackagePath, profileName)
	if err != nil {
		return errors.Wrap(err, "error creating profile")
	}

	//check to see if we have an existing profile at that location.
	exists, err := profile.profileAlreadyExists()
	if err != nil {
		return errors.Wrap(err, "error checking for existing profile")
	}
	if exists {
		localChanges, err := profile.localFilesChanged()
		if err != nil {
			return errors.Wrapf(err, "error checking for changes in %s", profile.ProfilePath)
		}

		if localChanges && profileName == DefaultProfile {
			fmt.Printf("WARNING: default profile has been changed by user or updated by elastic-package. The current profile will be moved to default_old.\n")
		}

		// If there's changes and we've selected CreateNew, move the old path
		// TODO: do we want this to pe appended with some kind of version string instead?
		if localChanges && !overwriteExisting {
			os.Rename(profile.ProfilePath, filepath.Join(profile.ProfilePath+"_old"))
			os.Mkdir(profile.ProfilePath, 0755)
		}
	} else {
		os.Mkdir(profile.ProfilePath, 0755)
	}
	if err != nil {
		return errors.Wrapf(err, "stat file failed (path: %s)", profile.ProfilePath)
	}

	//write the resources
	return profile.writeProfileResources()

}

// CreateProfileFromDefaultLocation creates an existing profile from the default elastic-package config dir
func CreateProfileFromDefaultLocation(profileName string, from string) error {
	loc, err := locations.StackDir()
	if err != nil {
		return errors.Wrap(err, "error finding stack dir location")
	}

	return CreateProfileFrom(loc, profileName, from)
}

// CreateProfileFrom creates a new profile by copying over an existing profile
func CreateProfileFrom(elasticPackagePath string, newProfileName string, fromProfileName string) error {

	fromProfile, err := LoadProfile(elasticPackagePath, fromProfileName)
	if err != nil {
		return errors.Wrapf(err, "error loading %s profile", fromProfileName)
	}

	newProfile, err := NewConfigProfile(elasticPackagePath, newProfileName)
	if err != nil {
		return errors.Wrapf(err, "error creating %s profile", newProfileName)
	}

	newExists, err := newProfile.profileAlreadyExists()
	if err != nil {
		return errors.Wrapf(err, "error checking profile %s", newProfile.ProfilePath)
	}
	if newExists {
		return errors.Errorf("profile %s already exists", newProfile.profileName)
	}
	os.Mkdir(newProfile.ProfilePath, 0755)

	newProfile.updateFileBodies(fromProfile.configFiles)
	return newProfile.writeProfileResources()

}

// LoadProfileFromDefaultLocation loads an existing profile from the default elastic-package config dir
func LoadProfileFromDefaultLocation(profileName string) (*ConfigProfile, error) {

	loc, err := locations.StackDir()
	if err != nil {
		return nil, errors.Wrap(err, "error finding stack dir location")
	}

	return LoadProfile(loc, profileName)
}

// DeleteProfileFromDefaultLocation deletes a profile from the default elastic-package config dir
func DeleteProfileFromDefaultLocation(profileName string) error {
	loc, err := locations.StackDir()
	if err != nil {
		return errors.Wrap(err, "error finding stack dir location")
	}
	return DeleteProfile(loc, profileName)
}

// DeleteProfile deletes a given config profile.
func DeleteProfile(elasticPackagePath string, profileName string) error {

	if profileName == DefaultProfile {
		return errors.New("cannot remove default profile")
	}

	pathToDelete := filepath.Join(elasticPackagePath, profileName)

	return os.RemoveAll(pathToDelete)

}

// PrintProfilesFromDefaultLocation lists known packages in the default elastic-package install dir
func PrintProfilesFromDefaultLocation() error {
	loc, err := locations.StackDir()
	if err != nil {
		return errors.Wrap(err, "error finding stack dir location")
	}
	return PrintProfiles(loc)
}

// PrintProfiles lists known profiles
func PrintProfiles(elasticPackagePath string) error {

	profiles, err := FetchAllProfiles(elasticPackagePath)
	if err != nil {
		return errors.Wrap(err, "error fetching profiles")
	}

	//print later, after we've run into any errors, to avoid some ugly-half printing if something fails halfway through
	header := []string{"Name", "Date Created", "User", "elastic-package version", "Path"}
	for _, headerVal := range header {
		fmt.Printf("%-30s ", headerVal)
	}
	fmt.Printf("\n")
	for _, iter := range profiles {
		fmt.Printf("%-30s %-30s %-30s %-30s %-30s\n", iter.Name, iter.DateCreated.Format(time.RFC3339), iter.User, iter.Version, iter.From)
	}

	return nil
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
		profile, err := LoadProfile(elasticPackagePath, item.Name())
		if err == ErrNotAProfile {
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
