// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package main

import (
	"errors"
	"io"
	"os"
	"path/filepath"

	"github.com/elastic/elastic-package/pkg/shell"

	yqcmd "github.com/mikefarah/yq/v4/cmd"
)

var _ shell.Command = &yqCmd{}

type yqCmd struct {
	p                 *Plugin
	name, usage, desc string
}

func registerYqCmd(p *Plugin) {
	cmd := &yqCmd{
		p:     p,
		name:  "yq",
		usage: "yq is a lightweight and portable command-line YAML processor.",
		desc:  "Runs the yq command for each of the packages in context 'Shell.Packages'. See https://mikefarah.gitbook.io/yq/ for detailed documentation and examples.",
	}
	p.RegisterCommand(cmd)
}

func (c *yqCmd) Name() string  { return c.name }
func (c *yqCmd) Usage() string { return c.usage }
func (c *yqCmd) Desc() string  { return c.desc }

func (c *yqCmd) Exec(wd string, args []string, stdout, stderr io.Writer) error {
	packages, ok := c.p.GetValueFromCtx(ctxKeyPackages).([]string)
	if !ok {
		return errors.New("no packages found in the context")
	}

	cmd := yqcmd.New()

	cmd.SetErr(stderr)
	cmd.SetOut(stdout)
	cmd.SetArgs(args)

	wdbak, err := os.Getwd()
	if err != nil {
		return err
	}
	defer os.Chdir(wdbak)

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

		if err := os.Chdir(packageRoot); err != nil {
			return err
		}

		if err := cmd.Execute(); err != nil {
			// no need to return, yq will output to stderr
			return nil
		}
	}

	return nil
}
