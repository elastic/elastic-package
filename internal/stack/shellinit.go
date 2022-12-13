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

	"github.com/elastic/go-sysinfo"
	"github.com/elastic/go-sysinfo/types"

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

var shellType string
var shellDetectError error

func init() {
	shellType, shellDetectError = detectShell()
}

// SelectShell selects the shell to use.
func SelectShell(shell string) {
	shellType = shell
	shellDetectError = nil
}

// AutodetectedShell returns an error if shell could not be detected.
func AutodetectedShell() (string, error) {
	return shellType, shellDetectError
}

// ShellInit method exposes environment variables that can be used for testing purposes.
func ShellInit(elasticStackProfile *profile.Profile) (string, error) {
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
	// NOTE: remove .exe extension from executable names present in Windows
	shell = strings.TrimSuffix(shell, ".exe")
	return shell
}

func detectShell() (string, error) {
	ppid := os.Getppid()
	parentInfo, err := getParentInfo(ppid)
	if errors.Is(err, types.ErrNotImplemented) {
		// Sysinfo doesn't implement some features in some platforms.
		// This mainly affects osx when building without CGO.
		// Assume bash in that case.
		// See https://github.com/elastic/elastic-package/issues/1030.
		logger.Debugf("Failed to determine parent process info while detecting shell, will assume bash")
		return "bash", nil
	}
	if err != nil {
		return "", err
	}

	shell := getShellName(parentInfo.Exe)
	if shell == "go" {
		parentParentInfo, err := getParentInfo(parentInfo.PPID)
		if err != nil {
			return "", fmt.Errorf("cannot retrieve parent process info: %w", err)
		}
		return getShellName(parentParentInfo.Exe), nil
	}

	return shell, nil
}

func getParentInfo(ppid int) (types.ProcessInfo, error) {
	parent, err := sysinfo.Process(ppid)
	if err != nil {
		return types.ProcessInfo{}, fmt.Errorf("cannot retrieve information for process %d: %w", ppid, err)
	}

	parentInfo, err := parent.Info()
	if err != nil {
		return types.ProcessInfo{}, fmt.Errorf("cannot retrieve information for parent of process %d: %w", ppid, err)
	}

	return parentInfo, nil
}
