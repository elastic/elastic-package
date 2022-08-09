// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package stack

import (
	"fmt"

	"github.com/elastic/elastic-package/internal/environment"
	"github.com/elastic/elastic-package/internal/profile"
	"github.com/elastic/elastic-package/internal/stack/shellinit/shell"
)

// Environment variables describing the stack.
var (
	ElasticsearchHostEnv     = environment.WithElasticPackagePrefix("ELASTICSEARCH_HOST")
	ElasticsearchUsernameEnv = environment.WithElasticPackagePrefix("ELASTICSEARCH_USERNAME")
	ElasticsearchPasswordEnv = environment.WithElasticPackagePrefix("ELASTICSEARCH_PASSWORD")
	KibanaHostEnv            = environment.WithElasticPackagePrefix("KIBANA_HOST")
	CACertificateEnv         = environment.WithElasticPackagePrefix("CA_CERT")
)

// ShellInit method exposes environment variables that can be used for testing purposes.
func ShellInit(elasticStackProfile *profile.Profile, st shell.Type) (string, error) {
	config, err := StackInitConfig(elasticStackProfile)
	if err != nil {
		return "", nil
	}

	// NOTE: to add new env vars, the template need to be adjusted
	return fmt.Sprintf(shell.InitTemplate(st),
		ElasticsearchHostEnv, config.ElasticsearchHostPort,
		ElasticsearchUsernameEnv, config.ElasticsearchUsername,
		ElasticsearchPasswordEnv, config.ElasticsearchPassword,
		KibanaHostEnv, config.KibanaHostPort,
		CACertificateEnv, config.CACertificatePath,
	), nil
}
