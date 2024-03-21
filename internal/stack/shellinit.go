// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package stack

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/shirou/gopsutil/v3/process"

	"github.com/elastic/elastic-package/internal/environment"
	"github.com/elastic/elastic-package/internal/logger"
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

// AutodetectShell returns the detected shell used.
func AutodetectShell() string {
	return detectShell()
}

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
export %s=%s`

	// fish shell init code.
	// fish shell is similar but not compliant to POSIX.
	fishTemplate = `set -x %s %s;
set -x %s %s;
set -x %s %s;
set -x %s %s;
set -x %s %s;`

	// PowerShell init code.
	// Output to be evaluated with `elastic-package stack shellinit | Invoke-Expression
	powershellTemplate = `$Env:%s="%s";
$Env:%s="%s";
$Env:%s="%s";
$Env:%s="%s";
$Env:%s="%s";`
)

// availableShellTypes list all available values for s in initTemplate
var availableShellTypes = []string{"bash", "dash", "fish", "sh", "zsh", "pwsh", "powershell"}

// InitTemplate returns code templates for shell initialization
func initTemplate(s string) (string, error) {
	switch s {
	case "bash", "dash", "sh", "zsh":
		return posixTemplate, nil
	case "fish":
		return fishTemplate, nil
	case "pwsh", "powershell":
		return powershellTemplate, nil
	default:
		return "", errors.New("shell type is unknown, should be one of " + strings.Join(availableShellTypes, ", "))
	}
}

// helpText returns the instrutions about how to set environment variables with shellinit
func helpText(shell string) string {
	switch shell {
	case "pwsh", "powershell":
		return `elastic-package stack shellinit | Invoke-Expression`
	default:
		return `eval "$(elastic-package stack shellinit)"`
	}
}

func getShellName(exe string) string {
	shell := filepath.Base(exe)
	cleanSuffixes := []string{
		// Remove .exe extension from executable names present in Windows.
		".exe",
		// Remove " (deleted)", that can appear here if the shell process has been
		// replaced by an upgrade in Linux while the terminal was open.
		" (deleted)",
	}
	for _, suffix := range cleanSuffixes {
		shell = strings.TrimSuffix(shell, suffix)
	}
	return shell
}

func detectShell() string {
	shell, err := getParentShell()
	if err != nil {
		logger.Debugf("Failed to determine parent process info while detecting shell, will assume bash: %v", err)
		return "bash"
	}

	return shell
}

func getParentShell() (string, error) {
	ppid := os.Getppid()
	p, err := process.NewProcess(int32(ppid))
	if err != nil {
		return "", err
	}
	exe, err := p.Exe()
	if err != nil {
		return "", err
	}

	shell := getShellName(exe)
	if shell == "go" {
		parent, err := p.Parent()
		if err != nil {
			return "", err
		}

		exe, err := parent.Exe()
		if err != nil {
			return "", err
		}

		return getShellName(exe), nil
	}

	return shell, nil
}
