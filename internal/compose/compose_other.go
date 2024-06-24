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
	"strings"

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

	err := debugPtyStats()
	if err != nil {
		logger.Debugf("failed to get pty stats: %s", err)
	}

	ptty, err := pty.Start(cmd)
	if err != nil {
		return fmt.Errorf("failed to start command with pseudo-tty: %w", err)
	}
	defer ptty.Close()
	logger.Debugf("running command: %s", cmd)

	io.Copy(stderr, ptty)

	if err := cmd.Wait(); err != nil {
		if msg := cleanComposeError(errBuffer.String()); len(msg) > 0 {
			return fmt.Errorf("%w: %s", err, msg)
		}
	}
	return err
}

func debugPtyStats() error {
	nr, err := os.ReadFile("/proc/sys/kernel/pty/nr")
	if err != nil {
		return fmt.Errorf("failed to read number of ptys: %w", err)

	}
	maxPtys, err := os.ReadFile("/proc/sys/kernel/pty/max")
	if err != nil {
		return fmt.Errorf("failed to read max, number of ptys: %w", err)
	}
	reservedPtys, err := os.ReadFile("/proc/sys/kernel/pty/reserve")
	if err != nil {
		return fmt.Errorf("failed to read reserved number of ptys: %w", err)
	}
	logger.Debugf("pty stats, used=%s max=%s reserved=%s",
		strings.TrimSpace(string(nr)),
		strings.TrimSpace(string(maxPtys)),
		strings.TrimSpace(string(reservedPtys)),
	)
	return nil
}
