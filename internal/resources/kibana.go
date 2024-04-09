// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package resources

import (
	"fmt"

	"github.com/Masterminds/semver/v3"
	"github.com/elastic/go-resource"

	"github.com/elastic/elastic-package/internal/kibana"
)

const DefaultKibanaProviderName = "kibana"

type KibanaProvider struct {
	Client *kibana.Client
}

func RegisterKibanaProvider(manager *resource.Manager, name string, client *kibana.Client) {
	manager.RegisterProvider(name, &KibanaProvider{Client: client})
}

func (p *KibanaProvider) version() (*semver.Version, error) {
	kibanaVersion, err := p.Client.Version()
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve kibana version: %w", err)
	}
	stackVersion, err := semver.NewVersion(kibanaVersion.Version())
	if err != nil {
		return nil, fmt.Errorf("failed to parse kibana version: %w", err)
	}
	return stackVersion, nil
}
