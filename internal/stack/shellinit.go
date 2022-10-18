// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package stack

import (
	"errors"
	"fmt"
	"strings"

	"github.com/elastic/elastic-package/internal/environment"
	"github.com/elastic/elastic-package/internal/profile"
)

// Environment variables describing the stack.
var (
	ElasticsearchHostEnv     = environment.WithElasticPackagePrefix("ELASTICSEARCH_HOST")
	ElasticsearchUsernameEnv = environment.WithElasticPackagePrefix("ELASTICSEARCH_USERNAME")
	ElasticsearchPasswordEnv = environment.WithElasticPackagePrefix("ELASTICSEARCH_PASSWORD")
	KibanaHostEnv            = environment.WithElasticPackagePrefix("KIBANA_HOST")
	CACertificateEnv         = environment.WithElasticPackagePrefix("CA_CERT")
)

// ShellInit method exposes environment variables that can be used for testing purposes.
func ShellInit(elasticStackProfile *profile.Profile, shellType string) (string, error) {
	config, err := StackInitConfig(elasticStackProfile)
	if err != nil {
		return "", nil
	}

	// NOTE: to add new env vars, the template need to be adjusted
	t, err := initTemplate(shellType)
	if err != nil {
		return "", fmt.Errorf("cannot get shell init template: %w", err)
	}

	return fmt.Sprintf(t,
		ElasticsearchHostEnv, config.ElasticsearchHostPort,
		ElasticsearchUsernameEnv, config.ElasticsearchUsername,
		ElasticsearchPasswordEnv, config.ElasticsearchPassword,
		KibanaHostEnv, config.KibanaHostPort,
		CACertificateEnv, config.CACertificatePath,
	), nil
}

const (
	// shell init code for POSIX compliant shells.
	// IEEE POSIX Shell and Tools portion of the IEEE POSIX specification (IEEE Standard 1003.1)
	posixTemplate = `export %s=%s
export %s=%s
export %s=%s
export %s=%s
export %s=%s
`
	// fish shell init code.
	// fish shell is similar but not compliant to POSIX.
	fishTemplate = `set -x %s %s;
set -x %s %s;
set -x %s %s;
set -x %s %s;
set -x %s %s;
`
)

// availableShellTypes list all available values for s in initTemplate
var availableShellTypes = []string{"bash", "fish", "sh", "zsh"}

// InitTemplate returns code templates for shell initialization
func initTemplate(s string) (string, error) {
	switch s {
	case "bash":
		return posixTemplate, nil
	case "fish":
		return fishTemplate, nil
	case "sh":
		return posixTemplate, nil
	case "zsh":
		return posixTemplate, nil
	default:
		return "", errors.New("shell type is unknown, should be one of " + strings.Join(availableShellTypes, ", "))
	}
}
