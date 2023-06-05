// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package cobraext

import (
	"fmt"
	"strings"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"

	"github.com/elastic/elastic-package/internal/configuration/locations"
	"github.com/elastic/elastic-package/internal/install"
	"github.com/elastic/elastic-package/internal/profile"
	"github.com/elastic/elastic-package/internal/stack"
)

// GetProfileFlag returns the profile information
func GetProfileFlag(cmd *cobra.Command) (*profile.Profile, error) {
	profileName, err := cmd.Flags().GetString(ProfileFlagName)
	if err != nil {
		return nil, FlagParsingError(err, ProfileFlagName)
	}
	if profileName == "" {
		config, err := install.Configuration()
		if err != nil {
			return nil, fmt.Errorf("cannot read configuration: %w", err)
		}
		profileName = config.CurrentProfile()
	}

	p, err := profile.LoadProfile(profileName)
	if errors.Is(err, profile.ErrNotAProfile) {
		list, err := availableProfilesAsAList()
		if err != nil {
			return nil, errors.Wrap(err, "error listing known profiles")
		}
		if len(list) == 0 {
			return nil, fmt.Errorf("%s is not a valid profile", profileName)
		}
		return nil, fmt.Errorf("%s is not a valid profile, known profiles are: %s", profileName, strings.Join(list, ", "))
	}
	if err != nil {
		return nil, errors.Wrap(err, "error loading profile")
	}

	return p, nil
}

func availableProfilesAsAList() ([]string, error) {
	loc, err := locations.NewLocationManager()
	if err != nil {
		return []string{}, errors.Wrap(err, "error fetching profile path")
	}

	profileNames := []string{}
	profileList, err := profile.FetchAllProfiles(loc.ProfileDir())
	if err != nil {
		return profileNames, errors.Wrap(err, "error fetching all profiles")
	}
	for _, prof := range profileList {
		profileNames = append(profileNames, prof.Name)
	}

	return profileNames, nil
}

// GetStackProviderFromProfile returns the provider related to the given profile
func GetStackProviderFromProfile(cmd *cobra.Command, profile *profile.Profile, checkFlag bool) (stack.Provider, error) {
	var providerName = stack.DefaultProvider
	stackConfig, err := stack.LoadConfig(profile)
	if err != nil {
		return nil, err
	}
	if stackConfig.Provider != "" {
		providerName = stackConfig.Provider
	}

	if checkFlag {
		providerFlag, err := cmd.Flags().GetString(StackProviderFlagName)
		if err != nil {
			return nil, FlagParsingError(err, StackProviderFlagName)
		}
		if providerFlag != "" {
			providerName = providerFlag
		}
	}

	return stack.BuildProvider(providerName, profile)
}
