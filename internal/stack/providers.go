// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package stack

import (
	"context"
	"fmt"
	"strings"

	"github.com/elastic/elastic-package/internal/profile"
)

const (
	ProviderCompose     = "compose"
	ProviderEnvironment = "environment"
	ProviderServerless  = "serverless"
)

var (
	DefaultProvider    = ProviderCompose
	SupportedProviders = []string{
		ProviderCompose,
		ProviderEnvironment,
		ProviderServerless,
	}
)

// Printer is the interface that can be used to print information on operations.
type Printer interface {
	Print(i ...interface{})
	Println(i ...interface{})
	Printf(format string, i ...interface{})
}

// Provider is the implementation of a stack provider.
type Provider interface {
	// BootUp starts a stack.
	BootUp(context.Context, Options) error

	// TearDown stops and/or removes a stack.
	TearDown(context.Context, Options) error

	// Update updates resources associated to a stack.
	Update(context.Context, Options) error

	// Dump dumps data for debug purpouses.
	Dump(context.Context, DumpOptions) ([]DumpResult, error)

	// Status obtains status information of the stack.
	Status(context.Context, Options) ([]ServiceStatus, error)
}

// BuildProvider returns the provider for the given name.
func BuildProvider(name string, profile *profile.Profile) (Provider, error) {
	switch name {
	case ProviderCompose:
		return &composeProvider{}, nil
	case ProviderEnvironment:
		return newEnvironmentProvider(profile)
	case ProviderServerless:
		return newServerlessProvider(profile)
	}
	return nil, fmt.Errorf("unknown provider %q, supported providers: %s", name, strings.Join(SupportedProviders, ", "))
}

type composeProvider struct{}

func (*composeProvider) BootUp(ctx context.Context, options Options) error {
	return BootUp(ctx, options)
}

func (*composeProvider) TearDown(ctx context.Context, options Options) error {
	return TearDown(ctx, options)
}

func (*composeProvider) Update(ctx context.Context, options Options) error {
	return Update(ctx, options)
}

func (*composeProvider) Dump(ctx context.Context, options DumpOptions) ([]DumpResult, error) {
	return Dump(ctx, options)
}

func (*composeProvider) Status(ctx context.Context, options Options) ([]ServiceStatus, error) {
	return Status(ctx, options)
}
