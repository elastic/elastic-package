// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package stack

import (
	"github.com/elastic/elastic-package/internal/profile"
)

type InitConfig struct {
	ElasticsearchAPIKey   string
	ElasticsearchHostPort string
	ElasticsearchUsername string
	ElasticsearchPassword string
	KibanaHostPort        string
	CACertificatePath     string
}

func StackInitConfig(profile *profile.Profile) (*InitConfig, error) {
	config, err := LoadConfig(profile)
	if err != nil {
		return nil, err
	}

	return &InitConfig{
		ElasticsearchAPIKey:   config.ElasticsearchAPIKey,
		ElasticsearchHostPort: config.ElasticsearchHost,
		ElasticsearchUsername: config.ElasticsearchUsername,
		ElasticsearchPassword: config.ElasticsearchPassword,
		KibanaHostPort:        config.KibanaHost,
		CACertificatePath:     config.CACertFile,
	}, nil
}
