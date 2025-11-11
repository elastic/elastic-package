// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.
package agentdeployer

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/elastic/elastic-package/internal/builder"
	"github.com/elastic/elastic-package/internal/compose"
	"github.com/elastic/elastic-package/internal/logger"
)

func processAgentContainerLogs(ctx context.Context, workDir string, p *compose.Project, opts compose.CommandOptions, agentName string) {
	content, err := p.Logs(ctx, opts)
	if err != nil {
		logger.Errorf("can't export service logs: %v", err)
		return
	}

	if len(content) == 0 {
		logger.Info("service container hasn't written anything logs.")
		return
	}

	err = writeAgentContainerLogs(workDir, agentName, content)
	if err != nil {
		logger.Errorf("can't write service container logs: %v", err)
	}
}

func writeAgentContainerLogs(workDir string, agentName string, content []byte) error {
	buildDir, err := builder.BuildDirectory(workDir)
	if err != nil {
		return fmt.Errorf("locating build directory failed: %w", err)
	}

	containerLogsDir := filepath.Join(buildDir, "container-logs")
	err = os.MkdirAll(containerLogsDir, 0o755)
	if err != nil {
		return fmt.Errorf("can't create directory for agent container logs (path: %s): %w", containerLogsDir, err)
	}

	containerLogsFilepath := filepath.Join(containerLogsDir, fmt.Sprintf("%s-%d.log", agentName, time.Now().UnixNano()))
	logger.Infof("Write container logs to file: %s", containerLogsFilepath)
	err = os.WriteFile(containerLogsFilepath, content, 0o644)
	if err != nil {
		return fmt.Errorf("can't write container logs to file (path: %s): %w", containerLogsFilepath, err)
	}
	return nil
}
