// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package cobraext

import (
	"github.com/spf13/cobra"
)

// CommandAction defines the signature of a cobra command action function
type CommandAction func(cmd *cobra.Command, args []string) error

// ComposeCommandActions runs the given command actions in order
func ComposeCommandActions(cmd *cobra.Command, args []string, actions ...CommandAction) error {
	for _, action := range actions {
		err := action(cmd, args)
		if err != nil {
			return err
		}
	}
	return nil
}
