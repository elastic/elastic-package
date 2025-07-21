// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

//go:build !windows

package compose

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"

	"github.com/creack/pty"

	"github.com/elastic/elastic-package/internal/logger"
)

func (p *Project) runDockerComposeCmd(ctx context.Context, opts dockerComposeOptions) error {
	name, args := p.dockerComposeBaseCommand()
	args = append(args, opts.args...)

	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Cancel = func() error {
		return cmd.Process.Signal(os.Interrupt)
	}
	cmd.Env = append(os.Environ(), opts.env...)

	var errBuffer bytes.Buffer
	var stderr io.Writer = &errBuffer
	cmd.Stdout = io.Discard
	if logger.IsDebugMode() {
		cmd.Stdout = os.Stdout
		stderr = io.MultiWriter(&errBuffer, os.Stderr)
	}
	if opts.stdout != nil {
		cmd.Stdout = opts.stdout
	}

	ptty, err := pty.Start(cmd)
	if err != nil {
		return fmt.Errorf("failed to start command with pseudo-tty: %w", err)
	}
	defer ptty.Close()
	logger.Tracef("running command: %s", cmd)

	io.Copy(stderr, ptty)

	if err := cmd.Wait(); err != nil {
		if msg := cleanComposeError(errBuffer.String()); len(msg) > 0 {
			return fmt.Errorf("%w: %s", err, msg)
		}
	}
	return err
}
