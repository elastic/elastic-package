package cobraext

import (
	"fmt"
	"strings"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"

	"github.com/elastic/elastic-package/internal/install"
	"github.com/elastic/elastic-package/internal/profile"
	"github.com/elastic/elastic-package/internal/stack"
)

func getProfileFlag(cmd *cobra.Command) (*profile.Profile, error) {
	profileName, err := cmd.Flags().GetString(cobraext.ProfileFlagName)
	if err != nil {
		return nil, cobraext.FlagParsingError(err, cobraext.ProfileFlagName)
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

func getProviderFromProfile(cmd *cobra.Command, profile *profile.Profile, checkFlag bool) (stack.Provider, error) {
	var providerName = stack.DefaultProvider
	stackConfig, err := stack.LoadConfig(profile)
	if err != nil {
		return nil, err
	}
	if stackConfig.Provider != "" {
		providerName = stackConfig.Provider
	}

	if checkFlag {
		providerFlag, err := cmd.Flags().GetString(cobraext.StackProviderFlagName)
		if err != nil {
			return nil, cobraext.FlagParsingError(err, cobraext.StackProviderFlagName)
		}
		if providerFlag != "" {
			providerName = providerFlag
		}
	}

	return stack.BuildProvider(providerName, profile)
}
