// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package stack

import (
	"fmt"

	"github.com/elastic/elastic-package/internal/profile"
)

type cloudProvider struct {
	profile *profile.Profile
}

func newCloudProvider(profile *profile.Profile) (*cloudProvider, error) {
	return &cloudProvider{
		profile: profile,
	}, nil
}

func (*cloudProvider) BootUp(options Options) error {
	return fmt.Errorf("not implemented")
}

func (*cloudProvider) TearDown(options Options) error {
	return fmt.Errorf("not implemented")
}

func (*cloudProvider) Update(options Options) error {
	fmt.Println("Nothing to do.")
	return nil
}

func (*cloudProvider) Dump(options DumpOptions) (string, error) {
	return "", fmt.Errorf("not implemented")
}

func (*cloudProvider) Status(options Options) ([]ServiceStatus, error) {
	return nil, fmt.Errorf("not implemented")
}
