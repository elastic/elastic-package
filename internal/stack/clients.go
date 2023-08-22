// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package stack

import (
	"errors"
	"os"

	"github.com/elastic/elastic-package/internal/elasticsearch"
	"github.com/elastic/elastic-package/internal/kibana"
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
