// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package shell

import (
	"os"
	"os/exec"
	"path/filepath"

	"github.com/elastic/elastic-package/internal/configuration/locations"
	"github.com/elastic/elastic-package/internal/logger"
	"github.com/hashicorp/go-plugin"
	"github.com/spf13/cobra"
)

func initCommands() (map[string]Command, error) {
	lm, err := locations.NewLocationManager()
	if err != nil {
		return nil, err
	}

	pluginsDir := lm.ShellPluginsDir()
	entries, err := os.ReadDir(pluginsDir)
	if err != nil {
		return nil, err
	}

	pluginMap := map[string]plugin.Plugin{
		"shell_plugin": &ShellPlugin{},
	}

	m := map[string]Command{}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		pluginPath := filepath.Join(pluginsDir, e.Name())
		client := plugin.NewClient(&plugin.ClientConfig{
			HandshakeConfig: Handshake(),
			Plugins:         pluginMap,
			Cmd:             exec.Command(pluginPath),
		})

		// Connect via RPC
		rpcClient, err := client.Client()
		if err != nil {
			return nil, err
		}

		// Request the plugin
		raw, err := rpcClient.Dispense("shell_plugin")
		if err != nil {
			return nil, err
		}

		// Obtain the interface implementation we can use to
		// interact with the plugin.
		shellPlugin := raw.(Plugin)
		for k, v := range shellPlugin.Commands() {
			m[k] = v
		}
	}
	return m, nil
}

func AttachCommands(parent *cobra.Command) {
	commands, err := initCommands()
	if err != nil {
		logger.Error(err)
		return
	}
	for _, command := range commands {
		cmd := &cobra.Command{
			Use:                   command.Usage(),
			Short:                 command.Desc(),
			RunE:                  commandRunE(command),
			DisableFlagParsing:    true,
			DisableFlagsInUseLine: true,
		}
		parent.AddCommand(cmd)
	}
}

func commandRunE(command Command) func(*cobra.Command, []string) error {
	return func(cmd *cobra.Command, args []string) error {
		wd, _ := os.Getwd()
		return command.Exec(wd, args, cmd.OutOrStdout(), cmd.OutOrStderr())
	}
}

// Handshake is a common handshake that is shared by plugin and host.
func Handshake() plugin.HandshakeConfig {
	return plugin.HandshakeConfig{
		ProtocolVersion:  1,
		MagicCookieKey:   "SHELL_PLUGIN",
		MagicCookieValue: "2a28a2da-7812-467c-b65b-d3a996f2e692",
	}
}
