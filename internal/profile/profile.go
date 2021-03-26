// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package profile

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/pkg/errors"
)

const (

	// DefaultProfile is the name of the default profile
	DefaultProfile = "default"
)

// ErrNotAProfile is returned in cases where we don't have a valid profile directory
var ErrNotAProfile = errors.New("Is is not a profile")

// ConfigFile is a type for for the config file names in a managed profile config
type ConfigFile string

// ManagedProfileFiles is the list of all files managed in a profile
// If you create a new file that's managed by a profile, it needs to go in this list
var ManagedProfileFiles = map[ConfigFile]NewConfig{
	KibanaConfigFile:              NewKibanaConfig,
	PackageRegistryDockerfileFile: NewPackageRegistryDockerfile,
	PackageRegistryConfigFile:     NewPackageRegistryConfig,
	SnapshotFile:                  NewSnapshotFile,
	PackageProfileMetaFile:        CreateProfileMetadata,
}

// ConfigProfile manages a stack config
type ConfigProfile struct {
	profileName string
	// ProfilePath is the absolute path to the profile
	ProfilePath string
	configFiles map[ConfigFile]*SimpleFile
}

// NewConfigProfile creates a new config profile manager
func NewConfigProfile(elasticPackagePath string, profileName string) (*ConfigProfile, error) {

	profilePath := filepath.Join(elasticPackagePath, profileName)

	var configMap = map[ConfigFile]*SimpleFile{}
	for fileItem, configInit := range ManagedProfileFiles {
		cfg, err := configInit(profileName, profilePath)
		if err != nil {
			return nil, errors.Wrapf(err, "error initializing config %s", cfg)
		}
		configMap[fileItem] = cfg
	}

	newProfile := &ConfigProfile{
		profileName: profileName,
		ProfilePath: profilePath,
		configFiles: configMap,
	}
	return newProfile, nil
}

// LoadProfile loads an existing profile
func LoadProfile(elasticPackagePath string, profileName string) (*ConfigProfile, error) {

	profilePath := filepath.Join(elasticPackagePath, profileName)

	isValid, err := isProfileDir(profilePath)
	if err != nil {
		return nil, errors.Wrapf(err, "error checking profile %s", profileName)
	}

	if !isValid {
		return nil, ErrNotAProfile
	}

	var configMap = map[ConfigFile]*SimpleFile{}
	for fileItem, configInit := range ManagedProfileFiles {
		cfg, err := configInit(profileName, profilePath)
		if err != nil {
			return nil, errors.Wrapf(err, "error initializing config %s", cfg)
		}
		configMap[fileItem] = cfg
	}

	profile := &ConfigProfile{
		profileName: profileName,
		ProfilePath: profilePath,
		configFiles: configMap,
	}

	exists, err := profile.profileAlreadyExists()
	if err != nil {
		return nil, errors.Wrapf(err, "error checking if profile %s exists", profileName)
	}

	if !exists {
		return nil, fmt.Errorf("Profile %s does not exist", profile.ProfilePath)
	}

	err = profile.readProfileResources()
	if err != nil {
		return nil, errors.Wrapf(err, "error reading in profile %s", profileName)
	}

	return profile, nil

}

// Fetch returns an absolute path to the given file
func (profile ConfigProfile) Fetch(file ConfigFile) string {
	return profile.configFiles[file].FilePath
}

// UpdateFileBodies updates the string contents of the config files
func (profile *ConfigProfile) updateFileBodies(newBody map[ConfigFile]*SimpleFile) {

	for key := range profile.configFiles {
		// skip metadata
		if key == PackageProfileMetaFile {
			continue
		}
		toReplace, ok := newBody[key]
		if ok {
			updatedProfile := profile.configFiles[key]
			updatedProfile.FileBody = toReplace.FileBody
			profile.configFiles[key] = updatedProfile
		}
	}

}

// ProfileAlreadyExists checks to see if a profile with this name already exists
func (profile ConfigProfile) profileAlreadyExists() (bool, error) {

	packageMetadata := profile.configFiles[PackageProfileMetaFile]
	// We do this in stages to make sure we return the right error.
	_, err := os.Stat(profile.ProfilePath)
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, errors.Wrapf(err, "error checking root directory: %s", packageMetadata.FilePath)
	}

	// If the folder exists, check to make sure it's a profile folder
	_, err = os.Stat(packageMetadata.FilePath)
	if os.IsNotExist(err) {
		return false, ErrNotAProfile
	}
	if err != nil {
		return false, errors.Wrapf(err, "error checking metadata: %s", packageMetadata.FilePath)
	}

	// if it is, see if it has the same profile name
	profileInfo, err := profile.metadata()
	if err != nil {
		return false, errors.Wrap(err, "error reading metadata")
	}

	if profileInfo.Name != profile.profileName {
		return false, nil
	}

	return true, nil
}

func (profile ConfigProfile) localFilesChanged() (bool, error) {

	for cfgName, cfgFile := range profile.configFiles {
		// skip checking the metadata file
		// TODO: in the future, we might want to check version to see if the default profile needs to be updated
		if cfgName == PackageProfileMetaFile {
			continue
		}
		changes, err := cfgFile.ConfigfilesDiffer()
		if err != nil {
			return false, errors.Wrap(err, "error checking config file")
		}
		if changes {
			return true, nil
		}
	}
	return false, nil
}

// readProfileResources reads the associated files into the config, as opposed to writing them out.
func (profile *ConfigProfile) readProfileResources() error {

	for _, cfgFile := range profile.configFiles {
		err := cfgFile.ReadConfig()
		if err != nil {
			return errors.Wrap(err, "error reading in profile")
		}
	}

	return nil
}

// writeProfileResources writes the config files
func (profile ConfigProfile) writeProfileResources() error {

	for _, cfgFiles := range profile.configFiles {
		err := cfgFiles.WriteConfig()
		if err != nil {
			return errors.Wrap(err, "error writing config file")
		}
	}
	return nil
}

// metadata returns the metadata struct for the profile
func (profile ConfigProfile) metadata() (Metadata, error) {
	packageMetadata := profile.configFiles[PackageProfileMetaFile]
	rawPackageMetadata, err := os.ReadFile(packageMetadata.FilePath)
	if err != nil {
		return Metadata{}, errors.Wrap(err, "error reading metadata file")
	}

	profileInfo := Metadata{}

	err = json.Unmarshal(rawPackageMetadata, &profileInfo)
	if err != nil {
		return Metadata{}, errors.Wrap(err, "error unmarshalling JSON")
	}

	return profileInfo, nil

}

// isProfileDir checks to see if the given path points to a valid profile
func isProfileDir(path string) (bool, error) {
	metaPath := filepath.Join(path, string(PackageProfileMetaFile))
	_, err := os.Stat(metaPath)
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, errors.Wrapf(err, "error stat: %s", metaPath)
	}
	return true, nil
}
