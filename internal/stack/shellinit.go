// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package stack

import (
	"fmt"

	"github.com/elastic/elastic-package/internal/environment"
	"github.com/elastic/elastic-package/internal/profile"
)

// Environment variables describing the stack.
var (
	ElasticsearchHostEnv     = environment.WithElasticPackagePrefix("ELASTICSEARCH_HOST")
	ElasticsearchUsernameEnv = environment.WithElasticPackagePrefix("ELASTICSEARCH_USERNAME")
	ElasticsearchPasswordEnv = environment.WithElasticPackagePrefix("ELASTICSEARCH_PASSWORD")
	KibanaHostEnv            = environment.WithElasticPackagePrefix("KIBANA_HOST")
	CACertificateEnv         = environment.WithElasticPackagePrefix("CA_CERT")
)

var shellInitFormat = "export " + ElasticsearchHostEnv + "=%s\n" +
	"export " + ElasticsearchUsernameEnv + "=%s\n" +
	"export " + ElasticsearchPasswordEnv + "=%s\n" +
	"export " + KibanaHostEnv + "=%s\n" +
	"export " + CACertificateEnv + "=%s"

// ShellInit method exposes environment variables that can be used for testing purposes.
func ShellInit(elasticStackProfile *profile.Profile) (string, error) {
	config, err := StackInitConfig(elasticStackProfile)
	if err != nil {
		return "", nil
	}

	return fmt.Sprintf(shellInitFormat,
		config.ElasticsearchHostPort,
		config.ElasticsearchUsername,
		config.ElasticsearchPassword,
		config.KibanaHostPort,
		config.CACertificatePath,
	), nil
}
