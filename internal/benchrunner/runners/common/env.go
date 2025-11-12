// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package common

import "github.com/elastic/elastic-package/internal/environment"

var (
	ESMetricstoreAPIKeyEnv        = environment.WithElasticPackagePrefix("ESMETRICSTORE_API_KEY")
	ESMetricstoreHostEnv          = environment.WithElasticPackagePrefix("ESMETRICSTORE_HOST")
	ESMetricstoreUsernameEnv      = environment.WithElasticPackagePrefix("ESMETRICSTORE_USERNAME")
	ESMetricstorePasswordEnv      = environment.WithElasticPackagePrefix("ESMETRICSTORE_PASSWORD")
	ESMetricstoreCACertificateEnv = environment.WithElasticPackagePrefix("ESMETRICSTORE_CA_CERT")
)
