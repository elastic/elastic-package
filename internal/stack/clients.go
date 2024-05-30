// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package stack

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"

	"github.com/elastic/elastic-package/internal/elasticsearch"
	"github.com/elastic/elastic-package/internal/kibana"
	"github.com/elastic/elastic-package/internal/logger"
	"github.com/elastic/elastic-package/internal/profile"
)

type Clients struct {
	Profile *profile.Profile
	Logger  *slog.Logger
}

// NewElasticsearchClient creates an Elasticsearch client with the settings provided by the shellinit
// environment variables.
func NewElasticsearchClient(customOptions ...elasticsearch.ClientOption) (*elasticsearch.Client, error) {
	options := []elasticsearch.ClientOption{
		elasticsearch.OptionWithAddress(os.Getenv(ElasticsearchHostEnv)),
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
	// TODO: receive logger from caller
	logger := logger.Logger
	profileConfig, err := StackInitConfig(profile)
	if err != nil {
		return nil, fmt.Errorf("failed to load config from profile: %w", err)
	}

	elasticsearchHost, found := os.LookupEnv(ElasticsearchHostEnv)
	if !found {
		// Using backgound context on initial call to avoid context cancellation.
		status, err := Status(context.Background(), Options{Profile: profile, Logger: logger})
		if err != nil {
			return nil, fmt.Errorf("failed to check status of stack in current profile: %w", err)
		}
		if len(status) == 0 {
			return nil, ErrUnavailableStack
		}

		elasticsearchHost = profileConfig.ElasticsearchHostPort
		logger.Debug("Connecting with Elasticsearch host from current profile",
			slog.String("profile", profile.ProfileName),
			slog.String("host", elasticsearchHost))
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
		kibana.Password(os.Getenv(ElasticsearchPasswordEnv)),
		kibana.Username(os.Getenv(ElasticsearchUsernameEnv)),
		kibana.CertificateAuthority(os.Getenv(CACertificateEnv)),
		kibana.Logger(logger.Logger),
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
	// TODO: receive logger from caller
	logger := logger.Logger
	profileConfig, err := StackInitConfig(profile)
	if err != nil {
		return nil, fmt.Errorf("failed to load config from profile: %w", err)
	}

	kibanaHost, found := os.LookupEnv(KibanaHostEnv)
	if !found {
		// Using backgound context on initial call to avoid context cancellation.
		status, err := Status(context.Background(), Options{Profile: profile, Logger: logger})
		if err != nil {
			return nil, fmt.Errorf("failed to check status of stack in current profile: %w", err)
		}
		if len(status) == 0 {
			return nil, ErrUnavailableStack
		}

		kibanaHost = profileConfig.KibanaHostPort
		logger.Debug("Connecting with Kibana host from current profile",
			slog.String("profile", profile.ProfileName),
			slog.String("host", kibanaHost))
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
		kibana.Password(elasticsearchPassword),
		kibana.Username(elasticsearchUsername),
		kibana.CertificateAuthority(caCertificate),
		kibana.Logger(logger),
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

	return caCertPath, nil
}
