// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/spf13/pflag"

	"github.com/elastic/elastic-package/pkg/shell"
)

var _ shell.Command = &runscriptCmd{}

type runscriptCmd struct {
	p                 *Plugin
	flags             *pflag.FlagSet
	name, usage, desc string
}

func registerRunscriptCmd(p *Plugin) {
	flags := pflag.NewFlagSet("", pflag.ContinueOnError)
	flags.String("path", "", "Path to the script file.")
	cmd := &runscriptCmd{
		p:     p,
		flags: flags,
		name:  "run-script",
		usage: "run-script --path path [args...]",
		desc:  "Runs a script for each of the packages in context 'Shell.Packages'. The package dir will be the first argument and the provided args will go next.",
	}
	p.RegisterCommand(cmd)
}

func (c *runscriptCmd) Name() string  { return c.name }
func (c *runscriptCmd) Usage() string { return c.usage }
func (c *runscriptCmd) Desc() string  { return c.desc }

func (c *runscriptCmd) Exec(wd string, args []string, stdout, stderr io.Writer) error {
	packages, ok := c.p.GetValueFromCtx(ctxKeyPackages).([]string)
	if !ok {
		return errors.New("no packages found in the context")
	}

	if err := c.flags.Parse(args); err != nil {
		return err
	}

	path, _ := c.flags.GetString("path")

	if !filepath.IsAbs(path) {
		path = filepath.Clean(filepath.Join(wd, path))
	}

	if _, err := os.Stat(path); err != nil {
		return fmt.Errorf("script not found at %s", path)
	}

	for _, pkg := range packages {
		packageRoot := filepath.Join(wd, pkg)
		// check if we are in packages folder
		if _, err := os.Stat(packageRoot); err != nil {
			// check if we are in integrations root folder
			packageRoot = filepath.Join(wd, "packages", pkg)
			if _, err := os.Stat(packageRoot); err != nil {
				return errors.New("you need to be in integrations root folder or in the packages folder")
			}
		}
		cmd := exec.Command(path, append([]string{packageRoot}, args...)...)
		cmd.Stdout = stdout
		cmd.Stderr = stderr
		if err := cmd.Run(); err != nil {
			fmt.Fprintln(stderr, err)
		}
	}

	return nil
}
