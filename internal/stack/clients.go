// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package stack

import (
	"errors"
	"fmt"
	"os"

	"github.com/elastic/elastic-package/internal/elasticsearch"
	"github.com/elastic/elastic-package/internal/kibana"
	"github.com/elastic/elastic-package/internal/profile"
)

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

func NewElasticsearchClientFromProfile(profile *profile.Profile, customOptions ...elasticsearch.ClientOption) (*elasticsearch.Client, error) {
	profileConfig, err := StackInitConfig(profile)
	if err != nil {
		return nil, fmt.Errorf("failed to load config from profile: %w", err)
	}

	elasticsearchHost, found := os.LookupEnv(ElasticsearchHostEnv)
	if !found {
		elasticsearchHost = profileConfig.ElasticsearchHostPort
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
	client, err := elasticsearch.NewClient(options...)

	if errors.Is(err, elasticsearch.ErrUndefinedAddress) {
		return nil, UndefinedEnvError(ElasticsearchHostEnv)
	}

	return client, err
}

func NewKibanaClient(customOptions ...kibana.ClientOption) (*kibana.Client, error) {
	options := []kibana.ClientOption{
		kibana.Address(os.Getenv(KibanaHostEnv)),
		kibana.Password(os.Getenv(ElasticsearchPasswordEnv)),
		kibana.Username(os.Getenv(ElasticsearchUsernameEnv)),
		kibana.CertificateAuthority(os.Getenv(CACertificateEnv)),
	}
	options = append(options, customOptions...)
	client, err := kibana.NewClient(options...)

	if errors.Is(err, kibana.ErrUndefinedHost) {
		return nil, UndefinedEnvError(ElasticsearchHostEnv)
	}

	return client, err
}

func NewKibanaClientFromProfile(profile *profile.Profile, customOptions ...kibana.ClientOption) (*kibana.Client, error) {
	profileConfig, err := StackInitConfig(profile)
	if err != nil {
		return nil, fmt.Errorf("failed to load config from profile: %w", err)
	}

	kibanaHost, found := os.LookupEnv(KibanaHostEnv)
	if !found {
		kibanaHost = profileConfig.KibanaHostPort
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
		kibana.CertificateAuthority(caCertificateEnv),
	}
	options = append(options, customOptions...)
	client, err := kibana.NewClient(options...)

	if errors.Is(err, kibana.ErrUndefinedHost) {
		return nil, UndefinedEnvError(ElasticsearchHostEnv)
	}

	return client, err
}
