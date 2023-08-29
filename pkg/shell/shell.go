// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package shell

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"plugin"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/elastic/elastic-package/internal/configuration/locations"
	"github.com/elastic/elastic-package/internal/logger"
)

var (
	commands = []Command{}

	// globalCtx is updated through all the run time of the shell and
	// it is used to pass information between plugins
	globalCtx = context.Background()
)

type Command interface {
	// Usage is the one-line usage message.
	// Recommended syntax is as follows:
	//   [ ] identifies an optional argument. Arguments that are not enclosed in brackets are required.
	//   ... indicates that you can specify multiple values for the previous argument.
	//   |   indicates mutually exclusive information. You can use the argument to the left of the separator or the
	//       argument to the right of the separator. You cannot use both arguments in a single use of the command.
	//   { } delimits a set of mutually exclusive arguments when one of the arguments is required. If the arguments are
	//       optional, they are enclosed in brackets ([ ]).
	// Example: add [-F file | -D dir]... [-f format] profile
	Usage() string
	Desc() string
	Flags() *pflag.FlagSet
	Exec(ctx context.Context, flags *pflag.FlagSet, args []string, stdout, stderr io.Writer) (context.Context, error)
}

type Plugin interface {
	Commands() []Command
}

func initCommands() error {
	lm, err := locations.NewLocationManager()
	if err != nil {
		return err
	}

	pluginsDir := lm.ShellPluginsDir()
	entries, err := os.ReadDir(pluginsDir)
	if err != nil {
		return err
	}

	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".so" {
			continue
		}

		pluginPath := filepath.Join(pluginsDir, e.Name())

		p, err := plugin.Open(pluginPath)
		if err != nil {
			return err
		}

		regSymbol, err := p.Lookup("Registry")
		if err != nil {
			return err
		}

		registry, ok := regSymbol.(Plugin)
		if !ok {
			return fmt.Errorf("registry in plugin %s does not implement the Plugin interface", pluginPath)
		}

		commands = append(commands, registry.Commands()...)
	}

	return nil
}

func AttachCommands(parent *cobra.Command) {
	if err := initCommands(); err != nil {
		logger.Error(err)
	}
	for _, command := range commands {
		cmd := &cobra.Command{
			Use:   command.Usage(),
			Short: command.Desc(),
			RunE:  commandRunE(command),
		}
		if command.Flags() != nil {
			command.Flags().VisitAll(func(f *pflag.Flag) {
				cmd.Flags().AddFlag(f)
			})
		}
		parent.AddCommand(cmd)
	}
}

func commandRunE(command Command) func(*cobra.Command, []string) error {
	return func(cmd *cobra.Command, args []string) error {
		flags := cmd.Flags()
		ctx, err := command.Exec(globalCtx, flags, args, cmd.OutOrStdout(), cmd.OutOrStderr())
		if err != nil {
			return err
		}
		globalCtx = ctx
		return nil
	}
}
