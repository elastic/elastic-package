// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.
package agentdeployer

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"github.com/elastic/elastic-package/internal/builder"
	"github.com/elastic/elastic-package/internal/compose"
)

func processAgentContainerLogs(ctx context.Context, p *compose.Project, opts compose.CommandOptions, agentName string, logger *slog.Logger) {
	content, err := p.Logs(ctx, opts)
	if err != nil {
		logger.Error("can't export service logs", slog.Any("error", err))
		return
	}

	if len(content) == 0 {
		logger.Info("service container hasn't written anything logs.")
		return
	}

	err = writeAgentContainerLogs(agentName, content, logger)
	if err != nil {
		logger.Error("can't write service container logs", slog.Any("error", err))
	}
}

func writeAgentContainerLogs(agentName string, content []byte, logger *slog.Logger) error {
	buildDir, err := builder.BuildDirectory()
	if err != nil {
		return fmt.Errorf("locating build directory failed: %w", err)
	}

	containerLogsDir := filepath.Join(buildDir, "container-logs")
	err = os.MkdirAll(containerLogsDir, 0o755)
	if err != nil {
		return fmt.Errorf("can't create directory for agent container logs (path: %s): %w", containerLogsDir, err)
	}

	containerLogsFilepath := filepath.Join(containerLogsDir, fmt.Sprintf("%s-%d.log", agentName, time.Now().UnixNano()))
	logger.Info("Write container logs to file", slog.String("path", containerLogsFilepath))
	err = os.WriteFile(containerLogsFilepath, content, 0o644)
	if err != nil {
		return fmt.Errorf("can't write container logs to file (path: %s): %w", containerLogsFilepath, err)
	}
	return nil
}
