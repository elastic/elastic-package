// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package stack

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/elastic/elastic-package/internal/elasticsearch"
	"github.com/elastic/elastic-package/internal/kibana"
	"github.com/elastic/elastic-package/internal/logger"
	"github.com/elastic/elastic-package/internal/profile"
)

// NewElasticsearchClient creates an Elasticsearch client with the settings provided by the shellinit
// environment variables.
func NewElasticsearchClient(customOptions ...elasticsearch.ClientOption) (*elasticsearch.Client, error) {
	options := []elasticsearch.ClientOption{
		elasticsearch.OptionWithAddress(os.Getenv(ElasticsearchHostEnv)),
		elasticsearch.OptionWithAPIKey(os.Getenv(ElasticsearchAPIKeyEnv)),
		elasticsearch.OptionWithPassword(os.Getenv(ElasticsearchPasswordEnv)),
		elasticsearch.OptionWithUsername(os.Getenv(ElasticsearchUsernameEnv)),
		elasticsearch.OptionWithCertificateAuthority(os.Getenv(CACertificateEnv)),
	}
	options = append(options, customOptions...)
	client, err := elasticsearch.NewClient(options...)

	if errors.Is(err, elasticsearch.ErrUndefinedAddress) {
		return nil, UndefinedEnvError(ElasticsearchHostEnv)
	}

	return client, err
}

// NewElasticsearchClientFromProfile creates an Elasticsearch client with the settings provided by the shellinit
// environment variables. If these environment variables are not set, it uses the information
// in the provided profile.
func NewElasticsearchClientFromProfile(profile *profile.Profile, customOptions ...elasticsearch.ClientOption) (*elasticsearch.Client, error) {
	profileConfig, err := StackInitConfig(profile)
	if err != nil {
		return nil, fmt.Errorf("failed to load config from profile: %w", err)
	}

	elasticsearchHost, found := os.LookupEnv(ElasticsearchHostEnv)
	if !found {
		err := checkClientStackAvailability(profile)
		if err != nil {
			return nil, err
		}

		elasticsearchHost = profileConfig.ElasticsearchHostPort
		logger.Debugf("Connecting with Elasticsearch host from current profile (profile: %s, host: %q)", profile.ProfileName, elasticsearchHost)
	}
	elasticsearchAPIKey, found := os.LookupEnv(ElasticsearchAPIKeyEnv)
	if !found {
		elasticsearchAPIKey = profileConfig.ElasticsearchAPIKey
	}
	elasticsearchPassword, found := os.LookupEnv(ElasticsearchPasswordEnv)
	if !found {
		elasticsearchPassword = profileConfig.ElasticsearchPassword
	}
	elasticsearchUsername, found := os.LookupEnv(ElasticsearchUsernameEnv)
	if !found {
		elasticsearchUsername = profileConfig.ElasticsearchUsername
	}
	caCertificate, found := os.LookupEnv(CACertificateEnv)
	if !found {
		caCertificate = profileConfig.CACertificatePath
	}

	options := []elasticsearch.ClientOption{
		elasticsearch.OptionWithAddress(elasticsearchHost),
		elasticsearch.OptionWithAPIKey(elasticsearchAPIKey),
		elasticsearch.OptionWithPassword(elasticsearchPassword),
		elasticsearch.OptionWithUsername(elasticsearchUsername),
		elasticsearch.OptionWithCertificateAuthority(caCertificate),
	}
	options = append(options, customOptions...)
	return elasticsearch.NewClient(options...)
}

// NewKibanaClient creates a kibana client with the settings provided by the shellinit
// environment variables.
func NewKibanaClient(customOptions ...kibana.ClientOption) (*kibana.Client, error) {
	options := []kibana.ClientOption{
		kibana.Address(os.Getenv(KibanaHostEnv)),
		kibana.APIKey(os.Getenv(ElasticsearchAPIKeyEnv)),
		kibana.Password(os.Getenv(ElasticsearchPasswordEnv)),
		kibana.Username(os.Getenv(ElasticsearchUsernameEnv)),
		kibana.CertificateAuthority(os.Getenv(CACertificateEnv)),
	}
	options = append(options, customOptions...)
	client, err := kibana.NewClient(options...)

	if errors.Is(err, kibana.ErrUndefinedHost) {
		return nil, UndefinedEnvError(KibanaHostEnv)
	}

	return client, err
}

// NewKibanaClientFromProfile creates a kibana client with the settings provided by the shellinit
// environment variables. If these environment variables are not set, it uses the information
// in the provided profile.
func NewKibanaClientFromProfile(profile *profile.Profile, customOptions ...kibana.ClientOption) (*kibana.Client, error) {
	profileConfig, err := StackInitConfig(profile)
	if err != nil {
		return nil, fmt.Errorf("failed to load config from profile: %w", err)
	}

	kibanaHost, found := os.LookupEnv(KibanaHostEnv)
	if !found {
		err := checkClientStackAvailability(profile)
		if err != nil {
			return nil, err
		}

		kibanaHost = profileConfig.KibanaHostPort
		logger.Debugf("Connecting with Kibana host from current profile (profile: %s, host: %q)", profile.ProfileName, kibanaHost)
	}
	elasticsearchAPIKey, found := os.LookupEnv(ElasticsearchAPIKeyEnv)
	if !found {
		elasticsearchAPIKey = profileConfig.ElasticsearchAPIKey
	}
	elasticsearchPassword, found := os.LookupEnv(ElasticsearchPasswordEnv)
	if !found {
		elasticsearchPassword = profileConfig.ElasticsearchPassword
	}
	elasticsearchUsername, found := os.LookupEnv(ElasticsearchUsernameEnv)
	if !found {
		elasticsearchUsername = profileConfig.ElasticsearchUsername
	}
	caCertificate, found := os.LookupEnv(CACertificateEnv)
	if !found {
		caCertificate = profileConfig.CACertificatePath
	}

	options := []kibana.ClientOption{
		kibana.Address(kibanaHost),
		kibana.APIKey(elasticsearchAPIKey),
		kibana.Password(elasticsearchPassword),
		kibana.Username(elasticsearchUsername),
		kibana.CertificateAuthority(caCertificate),
	}
	options = append(options, customOptions...)
	return kibana.NewClient(options...)
}

// FindCACertificate looks for the CA certificate for the stack in the current profile.
// If not found, it uses the environment variable provided by shellinit.
func FindCACertificate(profile *profile.Profile) (string, error) {
	caCertPath, found := os.LookupEnv(CACertificateEnv)
	if !found {
		profileConfig, err := StackInitConfig(profile)
		if err != nil {
			return "", fmt.Errorf("failed to load config from profile: %w", err)
		}
		caCertPath = profileConfig.CACertificatePath
	}

	// Avoid returning an empty certificate path, fallback to the default path.
	if caCertPath == "" {
		caCertPath = profile.Path(CACertificateFile)
	}

	return caCertPath, nil
}

func checkClientStackAvailability(profile *profile.Profile) error {
	config, err := LoadConfig(profile)
	if err != nil {
		return fmt.Errorf("cannot load stack configuration: %w", err)
	}

	// Checking it only with the compose provider because other providers need
	// a client, and we fall in infinite recursion.
	if config.Provider == ProviderCompose || config.Provider == "" {
		// Using backgound context on initial call to avoid context cancellation.
		status, err := Status(context.Background(), Options{Profile: profile})
		if err != nil {
			return fmt.Errorf("failed to check status of stack in current profile: %w", err)
		}
		if len(status) == 0 {
			return ErrUnavailableStack
		}
	}

	return nil
}
