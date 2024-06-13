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
	"sync"

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

	ptty, tty, err := pty.Open()
	if err != nil {
		return fmt.Errorf("failed to open pseudo-tty to capture stderr: %w", err)
	}

	var errBuffer bytes.Buffer
	cmd.Stderr = tty
	var stderr io.Writer = &errBuffer
	if logger.IsDebugMode() {
		cmd.Stdout = os.Stdout
		stderr = io.MultiWriter(&errBuffer, os.Stderr)
	}
	if opts.stdout != nil {
		cmd.Stdout = opts.stdout
	}

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		io.Copy(stderr, ptty)
	}()

	logger.Debugf("running command: %s", cmd)
	err = cmd.Run()
	tty.Close()
	wg.Wait()

	// Don't close the PTTY before the goroutine with the Copy has finished.
	ptty.Close()

	if err != nil {
		if msg := cleanComposeError(errBuffer.String()); len(msg) > 0 {
			return fmt.Errorf("%w: %s", err, msg)
		}
	}
	return err
}
