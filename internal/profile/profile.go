// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package profile

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/elastic/elastic-package/internal/logger"
)

// Profile manages a a given user config profile
type Profile struct {
	// ProfilePath is the absolute path to the profile
	ProfilePath      string
	ProfileStackPath string
	profileName      string
	configFiles      map[configFile]*simpleFile
}

const (
	// DefaultProfile is the name of the default profile
	DefaultProfile = "default"
)

// ErrNotAProfile is returned in cases where we don't have a valid profile directory
var ErrNotAProfile = fmt.Errorf("not a profile")

// configFile is a type for for the config file names in a managed profile config
type configFile string

// managedProfileFiles is the list of all files managed in a profile
// If you create a new file that's managed by a profile, it needs to go in this list
var managedProfileFiles = map[configFile]NewConfig{
	ElasticAgentDefaultEnvFile:     newElasticAgentDefaultEnv,
	ElasticAgent8xEnvFile:          newElasticAgent8xEnv,
	ElasticAgent80EnvFile:          newElasticAgent80Env,
	ElasticsearchConfigDefaultFile: newElasticsearchConfigDefault,
	ElasticsearchConfig8xFile:      newElasticsearchConfig8x,
	ElasticsearchConfig80File:      newElasticsearchConfig80,
	KibanaConfigDefaultFile:        newKibanaConfigDefault,
	KibanaConfig8xFile:             newKibanaConfig8x,
	KibanaConfig80File:             newKibanaConfig80,
	PackageRegistryDockerfileFile:  newPackageRegistryDockerfile,
	PackageRegistryConfigFile:      newPackageRegistryConfig,
	SnapshotFile:                   newSnapshotFile,
	PackageProfileMetaFile:         createProfileMetadata,
}

// NewConfigProfile creates a new config profile manager
func NewConfigProfile(elasticPackagePath string, profileName string) (*Profile, error) {
	profilePath := filepath.Join(elasticPackagePath, profileName)

	var configMap = map[configFile]*simpleFile{}
	for fileItem, configInit := range managedProfileFiles {
		cfg, err := configInit(profileName, profilePath)
		if err != nil {
			return nil, fmt.Errorf("error initializing config %s: %s", cfg, err)
		}
		configMap[fileItem] = cfg
	}

	err := initTLSCertificates(profilePath, configMap)
	if err != nil {
		return nil, fmt.Errorf("error initializing TLS certificates: %s", err)
	}

	newProfile := &Profile{
		profileName:      profileName,
		ProfilePath:      profilePath,
		ProfileStackPath: filepath.Join(profilePath, profileStackPath),
		configFiles:      configMap,
	}
	return newProfile, nil
}

// newProfileFromExistingFiles creates a profile from a list of absolute filepaths
// This can be used when migrating a config from a non-profiles-managed config set
// ignoreMissing will treat non-existant files as soft errors
func newProfileFromExistingFiles(elasticPackagePath string, profileName string, files []string, ignoreMissing bool) (*Profile, error) {
	profilePath := filepath.Join(elasticPackagePath, profileName)
	var configMap = map[configFile]*simpleFile{}
	for _, file := range files {
		if ignoreMissing {
			// if we're treating missing files as soft errors,
			// just continue on ErrNotExist
			// If it's another kind of error, we'll pick it up in ReadFile
			if _, err := os.Stat(file); errors.Is(err, os.ErrNotExist) {
				continue
			}
		}

		byteFile, err := os.ReadFile(file)
		if err != nil {
			return nil, fmt.Errorf("error reading file %s: %s", file, err)
		}
		//format this in the way configFile expects
		name := filepath.Base(file)
		configMap[configFile(name)] = &simpleFile{
			name: name,
			path: filepath.Join(profilePath, name),
			body: string(byteFile),
		}
	}

	//add metadata file
	metadata, err := createProfileMetadata(profileName, profilePath)
	if err != nil {
		return nil, fmt.Errorf("error creating profile metadata: %s", err)
	}
	configMap[PackageProfileMetaFile] = metadata

	newProfile := &Profile{
		profileName: profileName,
		ProfilePath: profilePath,
		configFiles: configMap,
	}
	return newProfile, nil
}

// loadProfile loads an existing profile
func loadProfile(elasticPackagePath string, profileName string) (*Profile, error) {
	profilePath := filepath.Join(elasticPackagePath, profileName)

	isValid, err := isProfileDir(profilePath)
	if err != nil {
		return nil, fmt.Errorf("error checking profile %s: %s", profileName, err)
	}

	if !isValid {
		return nil, ErrNotAProfile
	}

	var configMap = map[configFile]*simpleFile{}
	for fileItem, configInit := range managedProfileFiles {
		cfg, err := configInit(profileName, profilePath)
		if err != nil {
			return nil, fmt.Errorf("error initializing config %s: %s", cfg, err)
		}
		configMap[fileItem] = cfg
	}

	err = initTLSCertificates(profilePath, configMap)
	if err != nil {
		return nil, fmt.Errorf("error initializing TLS certificates: %s", err)
	}

	profile := &Profile{
		profileName:      profileName,
		ProfilePath:      profilePath,
		ProfileStackPath: filepath.Join(profilePath, profileStackPath),
		configFiles:      configMap,
	}

	exists, err := profile.alreadyExists()
	if err != nil {
		return nil, fmt.Errorf("error checking if profile %s exists: %s", profileName, err)
	}

	if !exists {
		return nil, fmt.Errorf("profile %s does not exist", profile.ProfilePath)
	}

	err = profile.readProfileResources()
	if err != nil {
		return nil, fmt.Errorf("error reading in profile %s: %s", profileName, err)
	}

	return profile, nil

}

// FetchPath returns an absolute path to the given file
func (profile Profile) FetchPath(file configFile) string {
	return profile.configFiles[file].path
}

// ComposeEnvVars returns a list of environment variables that can be passed
// to docker-compose for the sake of filling out paths and names in the snapshot.yml file.
func (profile Profile) ComposeEnvVars() []string {
	return []string{
		fmt.Sprintf("PROFILE_NAME=%s", profile.profileName),
		fmt.Sprintf("STACK_PATH=%s", profile.ProfileStackPath),
	}
}

// writeProfileResources writes the config files
func (profile Profile) writeProfileResources() error {
	return writeConfigFiles(profile.configFiles)
}

func writeConfigFiles(configFiles map[configFile]*simpleFile) error {
	for _, cfgFiles := range configFiles {
		err := cfgFiles.writeConfig()
		if err != nil {
			return fmt.Errorf("error writing config file: %s", err)
		}
	}

	return nil
}

// overwrite updates the string contents of the config files
func (profile *Profile) overwrite(newBody map[configFile]*simpleFile) {
	for key := range profile.configFiles {
		// skip metadata
		if key == PackageProfileMetaFile {
			continue
		}
		toReplace, ok := newBody[key]
		if ok {
			updatedProfile := profile.configFiles[key]
			updatedProfile.body = toReplace.body
			profile.configFiles[key] = updatedProfile
		}
	}

}

// alreadyExists checks to see if a profile with this name already exists
func (profile Profile) alreadyExists() (bool, error) {
	packageMetadata := profile.configFiles[PackageProfileMetaFile]
	// We do this in stages to make sure we return the right error.
	_, err := os.Stat(profile.ProfilePath)
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("error checking root directory: %s: %s", packageMetadata.path, err)
	}

	// If the folder exists, check to make sure it's a profile folder
	_, err = os.Stat(packageMetadata.path)
	if errors.Is(err, os.ErrNotExist) {
		return false, ErrNotAProfile
	}
	if err != nil {
		return false, fmt.Errorf("error checking metadata: %s: %s", packageMetadata.path, err)
	}

	//if it is, see if it has the same profile name
	profileInfo, err := profile.metadata()
	if err != nil {
		return false, fmt.Errorf("error reading metadata: %s", err)
	}

	//TODO: this will break default_old, as we don't update the json
	if profileInfo.Name != profile.profileName {
		return false, nil
	}

	return true, nil
}

func (profile Profile) localFilesChanged() (bool, error) {
	for cfgName, cfgFile := range profile.configFiles {
		// skip checking the metadata file
		// TODO: in the future, we might want to check version to see if the default profile needs to be updated
		if cfgName == PackageProfileMetaFile {
			continue
		}
		changes, err := cfgFile.configFilesDiffer()
		if err != nil {
			return false, fmt.Errorf("error checking config file: %s", err)
		}
		if changes {
			return true, nil
		}
	}
	return false, nil
}

// readProfileResources reads the associated files into the config, as opposed to writing them out.
func (profile Profile) readProfileResources() error {
	for _, cfgFile := range profile.configFiles {
		err := cfgFile.readConfig()
		if errors.Is(err, os.ErrNotExist) {
			logger.Debugf("File %s not found while reading profile.", cfgFile.path)
			continue
		}
		if err != nil {
			return fmt.Errorf("error reading in profile: %s", err)
		}
	}
	return nil
}

// metadata returns the metadata struct for the profile
func (profile Profile) metadata() (Metadata, error) {
	packageMetadata := profile.configFiles[PackageProfileMetaFile]
	rawPackageMetadata, err := os.ReadFile(packageMetadata.path)
	if err != nil {
		return Metadata{}, fmt.Errorf("error reading metadata file: %s", err)
	}

	profileInfo := Metadata{}

	err = json.Unmarshal(rawPackageMetadata, &profileInfo)
	if err != nil {
		return Metadata{}, fmt.Errorf("error unmarshalling JSON: %s", err)
	}
	return profileInfo, nil
}

// updateMetadata updates the metadata json file
func (profile *Profile) updateMetadata(meta Metadata) error {
	packageMetadata := profile.configFiles[PackageProfileMetaFile]
	metaString, err := json.Marshal(meta)
	if err != nil {
		return fmt.Errorf("error marshalling metadata json: %s", err)
	}
	err = os.WriteFile(packageMetadata.path, metaString, 0664)
	if err != nil {
		return fmt.Errorf("error writing metadata file: %s", err)
	}
	return nil
}

// isProfileDir checks to see if the given path points to a valid profile
func isProfileDir(path string) (bool, error) {
	metaPath := filepath.Join(path, string(PackageProfileMetaFile))
	_, err := os.Stat(metaPath)
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("error stat: %s: %s", metaPath, err)
	}
	return true, nil
}
