// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package stack

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"time"

	"github.com/elastic/elastic-package/internal/compose"
	"github.com/elastic/elastic-package/internal/docker"
	"github.com/elastic/elastic-package/internal/install"
	"github.com/elastic/elastic-package/internal/logger"
	"github.com/elastic/elastic-package/internal/profile"
)

const (
	elasticAgentService = "elastic-agent"
	fleetServerService  = "fleet-server"
)

// DumpOptions defines dumping options for Elatic stack data.
type DumpOptions struct {
	Profile *profile.Profile

	// Output is the path where the logs are copied. If not defined, logs are only returned as part of the dump results.
	Output string

	// Services is the list of services to get the logs from. If not defined, logs from all available services are dumped.
	Services []string

	// Since is the time to dump logs from.
	Since time.Time
}

// DumpResult contains the result of a dump operation.
type DumpResult struct {
	ServiceName     string
	Logs            []byte
	LogsFile        string
	InternalLogsDir string
}

// Dump function exports stack data and dumps them as local artifacts, which can be used for debug purposes.
func Dump(ctx context.Context, options DumpOptions) ([]DumpResult, error) {
	logger.Debugf("Dump Elastic stack data")

	results, err := dumpStackLogs(ctx, options)
	if err != nil {
		return nil, fmt.Errorf("can't dump Elastic stack logs: %w", err)
	}
	return results, nil
}

func dumpStackLogs(ctx context.Context, options DumpOptions) ([]DumpResult, error) {
	logger.Debugf("Dump stack logs (location: %s)", options.Output)
	err := os.RemoveAll(options.Output)
	if err != nil {
		return nil, fmt.Errorf("can't remove output location: %w", err)
	}

	services, err := localServiceNames(DockerComposeProjectName(options.Profile))
	if err != nil {
		return nil, fmt.Errorf("failed to get local services: %w", err)
	}

	for _, requestedService := range options.Services {
		if !slices.Contains(services, requestedService) {
			return nil, fmt.Errorf("local service %s does not exist", requestedService)
		}
	}

	var results []DumpResult
	for _, serviceName := range services {
		if len(options.Services) > 0 && !slices.Contains(options.Services, serviceName) {
			continue
		}

		logger.Debugf("Dump stack logs for %s", serviceName)

		content, err := dockerComposeLogsSince(ctx, serviceName, options.Profile, options.Since)
		if err != nil {
			return nil, fmt.Errorf("can't fetch service logs (service: %s): %v", serviceName, err)
		}
		if options.Output == "" {
			results = append(results, DumpResult{
				ServiceName: serviceName,
				Logs:        content,
			})
			continue
		}

		result := DumpResult{
			ServiceName: serviceName,
		}

		logsPath := filepath.Join(options.Output, "logs")
		err = os.MkdirAll(logsPath, 0755)
		if err != nil {
			return nil, fmt.Errorf("can't create output location (path: %s): %w", logsPath, err)
		}

		logPath, err := writeLogFiles(logsPath, serviceName, content)
		if err != nil {
			return nil, fmt.Errorf("can't write log files for service %q: %w", serviceName, err)
		}
		result.LogsFile = logPath

		switch serviceName {
		case elasticAgentService, fleetServerService:
			logPath, err := copyDockerInternalLogs(serviceName, logsPath, options.Profile)
			if err != nil {
				return nil, fmt.Errorf("can't copy internal logs (service: %s): %w", serviceName, err)
			}
			result.InternalLogsDir = logPath
		}

		results = append(results, result)
	}

	return results, nil
}

func writeLogFiles(logsPath, serviceName string, content []byte) (string, error) {
	logPath := filepath.Join(logsPath, serviceName+".log")
	err := os.WriteFile(logPath, content, 0644)
	if err != nil {
		return "", fmt.Errorf("can't write service logs (service: %s): %v", serviceName, err)
	}

	return logPath, nil
}

func dockerComposeLogsSince(ctx context.Context, serviceName string, profile *profile.Profile, since time.Time) ([]byte, error) {
	appConfig, err := install.Configuration()
	if err != nil {
		return nil, fmt.Errorf("can't read application configuration: %w", err)
	}

	snapshotFile := profile.Path(ProfileStackPath, SnapshotFile)

	p, err := compose.NewProject(DockerComposeProjectName(profile), snapshotFile)
	if err != nil {
		return nil, fmt.Errorf("could not create docker compose project: %w", err)
	}

	opts := compose.CommandOptions{
		Env: newEnvBuilder().
			withEnvs(appConfig.StackImageRefs(install.DefaultStackVersion).AsEnv()).
			withEnv(stackVariantAsEnv(install.DefaultStackVersion)).
			withEnvs(profile.ComposeEnvVars()).
			build(),
		Services: []string{serviceName},
	}

	if !since.IsZero() {
		opts.ExtraArgs = append(opts.ExtraArgs, "--since", since.UTC().Format("2006-01-02T15:04:05Z"))
	}

	out, err := p.Logs(ctx, opts)
	if err != nil {
		return nil, fmt.Errorf("running command failed: %w", err)
	}
	return out, nil
}

func copyDockerInternalLogs(serviceName, outputPath string, profile *profile.Profile) (string, error) {
	p, err := compose.NewProject(DockerComposeProjectName(profile))
	if err != nil {
		return "", fmt.Errorf("could not create docker compose project: %w", err)
	}

	outputPath = filepath.Join(outputPath, serviceName+"-internal")
	serviceContainer := p.ContainerName(serviceName)
	err = docker.Copy(serviceContainer, "/usr/share/elastic-agent/state/data/logs/", outputPath)
	if err != nil {
		return "", fmt.Errorf("docker copy failed: %w", err)
	}
	return outputPath, nil
}
