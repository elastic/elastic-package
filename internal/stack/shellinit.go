// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package stack

import (
	"fmt"

	"github.com/elastic/elastic-package/internal/profile"
)

const (
	elasticPackageEnvPrefix = "ELASTIC_PACKAGE_"
)

// Environment variables describing the stack.
var (
	ElasticsearchHostEnv     = elasticPackageEnvPrefix + "ELASTICSEARCH_HOST"
	ElasticsearchUsernameEnv = elasticPackageEnvPrefix + "ELASTICSEARCH_USERNAME"
	ElasticsearchPasswordEnv = elasticPackageEnvPrefix + "ELASTICSEARCH_PASSWORD"
	KibanaHostEnv            = elasticPackageEnvPrefix + "KIBANA_HOST"
	CACertificateEnv         = elasticPackageEnvPrefix + "CA_CERT"
)

var shellInitFormat = "export " + ElasticsearchHostEnv + "=%s\nexport " + ElasticsearchUsernameEnv + "=%s\nexport " +
	ElasticsearchPasswordEnv + "=%s\nexport " + KibanaHostEnv + "=%s"

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
		config.KibanaHostPort), nil
}
