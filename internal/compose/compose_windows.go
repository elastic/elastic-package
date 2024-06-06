// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

//go:build windows

package compose

import (
	"context"
	"os"
	"os/exec"

	"github.com/elastic/elastic-package/internal/logger"
)

func (p *Project) runDockerComposeCmd(ctx context.Context, opts dockerComposeOptions) error {
	name, args := p.dockerComposeBaseCommand()
	args = append(args, opts.args...)

	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Cancel = func() error {
		// Interrupt is not implemented in Windows.
		return cmd.Process.Kill()
	}
	cmd.Env = append(os.Environ(), opts.env...)

	// TODO: Use a Windows Pseudo-Console (ConPTY) to capture stderr without losing the default output.
	if logger.IsDebugMode() {
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
	}
	if opts.stdout != nil {
		cmd.Stdout = opts.stdout
	}
	return cmd.Run()
}
