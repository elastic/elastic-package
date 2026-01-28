// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package stack

import (
	"github.com/elastic/elastic-package/internal/install"
	"github.com/elastic/elastic-package/internal/profile"
	"github.com/elastic/elastic-package/internal/registry"
)

// packageRegistryProxyToURL returns the package registry URL to be used, considering
// profile settings and application configuration. The priority is given to
// profile settings over application configuration.
func packageRegistryProxyToURL(profile *profile.Profile, appConfig *install.ApplicationConfiguration) string {
	if registryURL := profile.Config(configElasticEPRProxyTo, ""); registryURL != "" {
		return registryURL
	}
	if registryURL := profile.Config(configElasticEPRURL, ""); registryURL != "" {
		return registryURL
	}
	if appConfig != nil {
		if registryURL := appConfig.PackageRegistryBaseURL(); registryURL != "" {
			return registryURL
		}
	}
	return registry.ProductionURL
}

// packageRegistryBaseURL returns the package registry base URL to be used, considering
// profile settings and application configuration. The priority is given to
// profile settings over application configuration.
func packageRegistryBaseURL(profile *profile.Profile, appConfig *install.ApplicationConfiguration) string {
	if registryURL := profile.Config(configElasticEPRURL, ""); registryURL != "" {
		return registryURL
	}
	if appConfig != nil {
		if registryURL := appConfig.PackageRegistryBaseURL(); registryURL != "" {
			return registryURL
		}
	}
	return registry.ProductionURL
}
