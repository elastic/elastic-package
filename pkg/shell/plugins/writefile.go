// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/pflag"

	"github.com/elastic/elastic-package/pkg/shell"
)

var _ shell.Command = writefileCmd{}

type writefileCmd struct{}

func (writefileCmd) Usage() string {
	return "write-file --path path --contents contents"
}

func (writefileCmd) Desc() string {
	return "Writes a file in each of the packages in context 'Shell.Packages'."
}

func (writefileCmd) Flags() *pflag.FlagSet {
	flags := pflag.NewFlagSet("", pflag.ContinueOnError)
	flags.String("path", "", "Path to the file (relative to the package root).")
	flags.String("contents", "", "Contents of the file")
	return flags
}

func (writefileCmd) Exec(ctx context.Context, flags *pflag.FlagSet, args []string, _, stderr io.Writer) (context.Context, error) {
	packages, ok := ctx.Value(ctxKeyPackages).([]string)
	if !ok {
		fmt.Fprintln(stderr, "no packages found in the context")
		return ctx, nil
	}
	for _, pkg := range packages {
		packageRoot := pkg
		// check if we are in packages folder
		if _, err := os.Stat(filepath.Join(".", pkg)); err != nil {
			// check if we are in integrations root folder
			packageRoot = filepath.Join(".", "packages", pkg)
			if _, err := os.Stat(packageRoot); err != nil {
				return ctx, errors.New("you need to be in intgerations root folder or in the packages folder")
			}
		}

		path, _ := flags.GetString("path")
		path = filepath.Join(packageRoot, path)

		contents, _ := flags.GetString("contents")

		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			return ctx, err
		}

		f, err := os.Create(path)
		if err != nil {
			return ctx, err
		}

		if _, err := f.WriteString(strings.ReplaceAll(contents, `\n`, "\n")); err != nil {
			f.Close()
			return ctx, err
		}

		f.Close()
	}
	return ctx, nil
}
