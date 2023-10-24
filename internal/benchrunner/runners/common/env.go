package common

import "github.com/elastic/elastic-package/internal/environment"

var (
	ESMetricstoreHostEnv          = environment.WithElasticPackagePrefix("ESMETRICSTORE_HOST")
	ESMetricstoreUsernameEnv      = environment.WithElasticPackagePrefix("ESMETRICSTORE_USERNAME")
	ESMetricstorePasswordEnv      = environment.WithElasticPackagePrefix("ESMETRICSTORE_PASSWORD")
	ESMetricstoreCACertificateEnv = environment.WithElasticPackagePrefix("ESMETRICSTORE_CA_CERT")
)
