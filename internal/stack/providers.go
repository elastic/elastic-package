// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package stack

import (
	"fmt"
	"strings"

	"github.com/elastic/elastic-package/internal/profile"
)

const (
	ProviderCompose = "compose"
	ProviderCloud   = "cloud"
)

var (
	DefaultProvider    = ProviderCompose
	SupportedProviders = []string{
		ProviderCompose,
		ProviderCloud,
	}
)

// Provider is the implementation of a stack provider.
type Provider interface {
	// BootUp starts a stack.
	BootUp(Options) error

	// TearDown stops and/or removes a stack.
	TearDown(Options) error

	// Update updates resources associated to a stack.
	Update(Options) error

	// Dump dumps data for debug purpouses.
	Dump(DumpOptions) (string, error)

	// Status obtains status information of the stack.
	Status() ([]ServiceStatus, error)
}

// BuildProvider returns the provider for the given name.
func BuildProvider(name string, profile *profile.Profile) (Provider, error) {
	switch name {
	case "compose":
		return &composeProvider{}, nil
	case "cloud":
		return newCloudProvider(profile)
	}
	return nil, fmt.Errorf("unknown provider %q, supported providers: %s", name, strings.Join(SupportedProviders, ", "))
}

type composeProvider struct{}

func (*composeProvider) BootUp(options Options) error {
	return BootUp(options)
}

func (*composeProvider) TearDown(options Options) error {
	return TearDown(options)
}

func (*composeProvider) Update(options Options) error {
	return Update(options)
}

func (*composeProvider) Dump(options DumpOptions) (string, error) {
	return Dump(options)
}

func (*composeProvider) Status() ([]ServiceStatus, error) {
	return Status()
}
